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
    participant Email as EmailSender (Resend)
    participant DB as Postgres (users, refresh_tokens, verification_tokens)

    %% Signup Flow
    Note over Client,DB: User Signup Flow
    Client->>Router: POST /api/v1/auth/signup {email, password}
    Router->>Svc: Signup(ctx, email, password)
    Svc->>Svc: HashPasswordBcrypt()
    Svc->>DB: CreateUser(email, password_hash, email_verified=false)
    DB-->>Svc: user record
    Svc->>DB: InsertVerificationToken(user_id, token_hash, expires_at)
    DB-->>Svc: verification_token record
    Svc->>Email: Send(ctx, Message{To: email, Subject, HTML})
    Email-->>Svc: nil
    Svc-->>Router: User
    Router-->>Client: 201 Created {id, email, created_at}

    %% Email Verification Flow
    Note over Client,DB: Email Verification Flow
    Client->>Router: POST /api/v1/auth/verify-email {token}
    Router->>Svc: VerifyEmail(ctx, token)
    Svc->>DB: ConsumeVerificationToken(token_hash)
    alt Token Missing or Expired or Consumed
        DB-->>Svc: ErrNoRows or token state check
        Svc-->>Router: ErrVerificationNotFound / Expired / Consumed
        Router-->>Client: 404 / 410 / 409 Problem Details
    else Token Valid
        DB-->>Svc: verification_token record
        Svc->>DB: MarkEmailVerified(user_id)
        DB-->>Svc: nil
        Svc-->>Router: nil
        Router-->>Client: 200 OK {"message": "email verified successfully"}
    end

    %% Resend Verification Flow
    Note over Client,Email: Resend Verification Flow
    Client->>Router: POST /api/v1/auth/resend-verification {email}
    Router->>Svc: ResendVerification(ctx, email)
    Svc->>DB: GetUserByEmail(email)
    alt User Exists and email_verified == false
        Svc->>DB: CountActiveVerificationTokens(user_id)
        alt Active Tokens < 3
            Svc->>DB: InsertVerificationToken(user_id, token_hash, expires_at)
            Svc->>Email: Send(ctx, Message{To: email, Subject, HTML})
        end
    end
    Svc-->>Router: nil
    Router-->>Client: 202 Accepted {"message": "if an account exists..."}

    %% Login Flow
    Note over Client,DB: User Login Flow
    Client->>Router: POST /api/v1/auth/login {email, password}
    Router->>Svc: Login(ctx, email, password)
    Svc->>DB: GetUserByEmail(email)
    DB-->>Svc: user record
    Svc->>Svc: CheckPasswordTimingSafe()
    alt Email Not Verified
        Svc-->>Router: ErrEmailNotVerified
        Router-->>Client: 403 Forbidden (RFC 7807 problem details)
    else Email Verified
        Svc->>DB: InsertRefreshToken(user_id, family_id, token_hash, expires_at)
        DB-->>Svc: refresh_token record
        Svc->>Svc: GenerateAccessToken(user_id, standard)
        Svc-->>Router: TokenPair (access_token, refresh_token)
        Router-->>Client: 200 OK {access_token, refresh_token, token_type, expires_in}
    end

    %% Refresh Rotation Flow
    Note over Client,DB: Refresh Token Rotation Flow
    Client->>Router: POST /api/v1/auth/refresh {refresh_token, idempotency_key}
    Router->>Svc: Refresh(ctx, refresh_token, idempotency_key)
    Svc->>DB: BEGIN TX
    Svc->>DB: GetRefreshTokenByHashForUpdate(token_hash)
    DB-->>Svc: token row (locked)

    alt Token Active and Unrevoked
        Svc->>Svc: GenerateRefreshToken() and GenerateAccessToken()
        Svc->>DB: InsertRefreshToken(same family_id, new token_hash)
        DB-->>Svc: new token record
        Svc->>DB: RevokeRefreshTokenWithReplacement(old_id, replaced_by=new_id)
        Svc->>DB: COMMIT TX
        Svc->>Cache: Put(old_token_hash, idempotency_key, token_pair, ttl=10s)
        Svc-->>Router: New TokenPair
        Router-->>Client: 200 OK {access_token, refresh_token, token_type, expires_in}
    else Token Revoked (Replay or Theft)
        Svc->>Cache: Get(old_token_hash)
        alt Cache Hit and Same Idempotency Key (Benign Replay)
            Cache-->>Svc: cached TokenPair
            Svc->>DB: COMMIT TX
            Svc-->>Router: Cached TokenPair
            Router-->>Client: 200 OK (Replay of original response)
        else Cache Miss or Mismatched Idempotency Key (Theft Detected)
            Svc->>DB: RevokeRefreshTokenFamily(family_id, revoked_at=now())
            Svc->>DB: COMMIT TX
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
    Router-->>Client: 200 OK (Logged out successfully)
```

## 2. Security Invariants

1. **Idempotency Protection:**
   The `POST /api/v1/auth/refresh` endpoint requires both `refresh_token` and `idempotency_key`. A thread-safe in-memory cache stores rotated token pairs for 10 seconds. Network retries that transmit the identical `idempotency_key` receive the cached token pair without creating new database records.

2. **Theft Containment:**
   If a client presents a revoked refresh token with a different `idempotency_key`, or after the 10-second replay window, the system flags the request as token theft. The system immediately executes an $O(1)$ query that revokes all active tokens in that `family_id` and writes a security log.

3. **Concurrency Safety:**
   Rotation uses `SELECT ... FOR UPDATE` row-level database locking inside an isolated transaction. Concurrent requests with identical tokens wait on the lock and then evaluate against the replay cache safely.

4. **Email Verification Gate and Enumeration Suppression:**
   Accounts created through `POST /api/v1/auth/signup` persist with `email_verified = false`. Login attempts for unverified accounts receive `403 Forbidden` with problem detail type `https://cloudvitta.dev/errors/email-not-verified`. The resend endpoint `POST /api/v1/auth/resend-verification` enforces a maximum of 3 active verification tokens and uniformly returns `202 Accepted` to prevent user email enumeration.
