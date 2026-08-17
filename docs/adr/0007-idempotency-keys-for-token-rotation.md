# 7. Idempotency Keys for Token Rotation

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta uses stateless short-lived JWTs paired with stateful, hashed refresh tokens stored in Postgres. To protect against token theft, presenting an already-revoked refresh token triggers an immediate revocation of the entire token family, forcing a re-login.

However, unstable network connections (such as mobile handoffs) can cause a client to retry a refresh request if the connection drops after the server rotates the token but before the client receives the response.

A time-based grace window (5 to 10 seconds) creates a security window where an attacker who steals a token can replay it. Network-based heuristics like IP binding fail against proxy attackers and disrupt legitimate users switching networks.

## Decision

We use **Client-Generated Idempotency Keys (Nonces)** to handle token rotation retries:
1. **Client Nonce**: When requesting token rotation, the client submits a unique `idempotency_key` alongside the refresh token.
2. **Server Replay Cache**: The server executes the rotation inside a database transaction, revokes the old token, issues a new token pair, and caches the new token pair against the old token hash and `idempotency_key` in a thread-safe cache for 10 seconds.
3. **Deterministic Benign Replay**: If a revoked token is presented with the matching cached `idempotency_key`, the server returns the cached token pair without creating new database records.
4. **Immediate Theft Containment**: If a revoked token is presented with a mismatched `idempotency_key` or after the replay cache window, the server immediately revokes the entire token family and logs a security alert.

## Consequences

### Positive
- Prevents token replay vulnerabilities without relying on arbitrary timing heuristics.
- Eliminates false positives caused by client IP changes or mobile network transitions.
- Provides mathematically deterministic retry handling for distributed clients.

### Negative
- Clients must generate and send a unique `idempotency_key` (UUID or nonce) with every token rotation request.
- The server must maintain a short-lived in-memory replay cache (`RotationCache`) to store recent rotation results.

## Alternatives Considered

### Alternative 1: Time-Based Grace Window (5 to 10 Seconds)
Rejected. A time-based grace window permits an attacker who intercepts a refresh token to reuse it within the window, defeating prompt theft containment.

### Alternative 2: Client IP Binding and Device Fingerprinting
Rejected. IP binding produces frequent false-positive logouts when mobile devices transition between Wi-Fi and cellular connections, and fails to protect against attacks routed through shared corporate proxies.
