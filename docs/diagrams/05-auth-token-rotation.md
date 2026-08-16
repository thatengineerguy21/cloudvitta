# Auth Token Rotation and Theft Containment

This document describes the authentication token rotation lifecycle, idempotency replay containment, and logout flow.

## 1. Sequence Diagram

```mermaid
sequenceDiagram
    autonumber
    participant Client as Client Application
    participant Router as REST Router / Handler
    participant Svc as AuthService
    participant Cache as In-Memory RotationCache
    participant DB as Postgres (users, refresh_tokens)

    %% Login / Signup Flow
    Note over Client,DB: User Login Flow
    Client->>Router: POST /api/v1/auth/login {email, password}
    Router->>Svc: Login(ctx, email, password)
    Svc->>DB: GetUserByEmail(email)
    DB-->>Svc: user record
    Svc->>Svc: CheckPasswordTimingSafe()
    Svc->>DB: InsertRefreshToken(user_id, family_id, token_hash, expires_at)
    DB-->>Svc: refresh_token record
    Svc->>Svc: GenerateAccessToken(user_id, "standard")
    Svc-->>Router: TokenPair (access_token, refresh_token)
    Router-->>Client: 200 OK {access_token, refresh_token, token_type, expires_in}

    %% Refresh Rotation Flow
    Note over Client,DB: Refresh Token Rotation Flow
    Client->>Router: POST /api/v1/auth/refresh {refresh_token, idempotency_key}
    Router->>Svc: Refresh(ctx, refresh_token, idempotency_key)
    Svc->>DB: BEGIN TX; GetRefreshTokenByHashForUpdate(token_hash)
    DB-->>Svc: token row (locked)

    alt Token Active and Unrevoked
        Svc->>Svc: GenerateRefreshToken() & GenerateAccessToken()
        Svc->>DB: InsertRefreshToken(same family_id, new token_hash)
        DB-->>Svc: new token record
        Svc->>DB: RevokeRefreshTokenWithReplacement(old_id, replaced_by=new_id); COMMIT TX
        Svc->>Cache: Put(old_token_hash, idempotency_key, token_pair, ttl=10s)
        Svc-->>Router: New TokenPair
        Router-->>Client: 200 OK {access_token, refresh_token, token_type, expires_in}
    else Token Revoked (Replay or Theft)
        Svc->>Cache: Get(old_token_hash)
        alt Cache Hit & Same Idempotency Key (Benign Replay)
            Cache-->>Svc: cached TokenPair
            Svc->>DB: COMMIT TX
            Svc-->>Router: Cached TokenPair
            Router-->>Client: 200 OK (Replay of original response)
        else Cache Miss or Mismatched Idempotency Key (Theft Detected)
            Svc->>DB: RevokeRefreshTokenFamily(family_id, revoked_at=now()); COMMIT TX
            Svc->>Svc: Log Security Warning (slog.WarnContext)
            Svc-->>Router: ErrTokenFamilyRevoked
            Router-->>Client: 401 Unauthorized (RFC 7807 problem details)
        end
    end

    %% Logout Flow
    Note over Client,DB: Session Revocation / Logout Flow
    Client->>Router: POST /api/v1/auth/logout {refresh_token}
    Router->>Svc: Logout(ctx, refresh_token)
    Svc->>DB: RevokeRefreshTokenByHash(token_hash)
    Svc->>Cache: Delete(token_hash)
    Svc-->>Router: nil
    Router-->>Client: 200 OK {"message": "logged out successfully"}
```

## 2. Security Invariants

1. **Idempotency Protection:**
   The `POST /api/v1/auth/refresh` endpoint requires both `refresh_token` and `idempotency_key`. A thread-safe in-memory cache stores rotated token pairs for 10 seconds. Network retries that transmit the identical `idempotency_key` receive the cached token pair without creating new database records.

2. **Theft Containment:**
   If a client presents a revoked refresh token with a different `idempotency_key`, or after the 10-second replay window, the system flags the request as token theft. The system immediately executes an $O(1)$ query that revokes all active tokens in that `family_id` and writes a security log.

3. **Concurrency Safety:**
   Rotation uses `SELECT ... FOR UPDATE` row-level database locking inside an isolated transaction. Concurrent requests with identical tokens wait on the lock and then evaluate against the replay cache safely.
