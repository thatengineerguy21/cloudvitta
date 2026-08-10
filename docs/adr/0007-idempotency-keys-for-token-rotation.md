# 7. Idempotency Keys for Token Rotation

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta uses stateless short-lived JWTs paired with stateful, hashed refresh tokens stored in Postgres. To protect against token theft, presenting an already-revoked refresh token triggers an immediate revocation of the entire token family, forcing a re-login.

However, flakey networks (e.g., mobile connections) can cause a client to legitimately retry a refresh request if the network drops just after the server mints a new token but before the client receives it. 

The original design proposed a "5-10s time-based grace window" where the server would guess that a reused token was a benign retry. This introduced a small window where an attacker who stole the old token could replay it and steal the session. Complex mitigations like IP-binding were considered but rejected because they fail against the actual threat model (compromised proxy) and produce false positives for legitimate users switching networks (WiFi to Cellular).

## Decision

We will use **Client-generated Idempotency Keys (Nonces)** to solve the token rotation retry problem definitively.

1. **Client Nonce**: When requesting a token rotation, the client must submit a unique, one-time `idempotency_key` (nonce) alongside the refresh token.
2. **Server Caching**: The server processes the rotation, issues the new token pair, revokes the old token, and caches the *new pair* against that exact `idempotency_key` for a short period.
3. **Unambiguous Retry**: If a revoked token is presented again, the server checks the presented `idempotency_key`. If it matches the cached key for that rotation, it is unambiguously a benign network retry, and the server returns the cached new pair.
4. **Immediate Revocation**: If a revoked token is presented *without* the matching nonce, it is unambiguously token theft. The grace window is eliminated, and the entire token family is immediately revoked.

## Consequences

### Positive
- Shrinks the attack surface for stolen tokens close to zero by removing the timing heuristic.
- Avoids the complexity and false-positive risk of IP-binding or client fingerprinting.
- Definitively solves the flakey network retry problem with a mathematically precise mechanism rather than a guess.

### Negative
- Requires clients (e.g., frontend, CLI, agents) to correctly generate and send a unique nonce with every rotation request.
- Requires short-lived server-side caching (e.g., in Redis) to hold the response payload mapped to the idempotency key.
