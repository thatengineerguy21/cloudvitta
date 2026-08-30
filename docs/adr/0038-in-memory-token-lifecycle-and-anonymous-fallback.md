# 38. In-Memory Token Lifecycle, Single-Flight Mutex Refresh, and Silent Anonymous Fallback

Date: 2026-08-30

## Status

Accepted

## Context

Web browser applications that store authentication tokens in `localStorage` or `sessionStorage` are vulnerable to token extraction attacks if cross-site scripting (XSS) occurs. In addition, when multiple HTTP requests fail simultaneously with HTTP 401 Unauthorized status due to token expiration, each request attempts to refresh tokens independently.

Under CloudVitta's refresh token rotation and theft detection architecture (ADR 0007, ADR 0021), using a revoked or rotated refresh token a second time triggers immediate token family revocation. Concurrent, uncoordinated refresh requests cause race conditions where one request consumes the single-use refresh token, and subsequent requests trigger false-positive token family revocations.

Furthermore, CloudVitta provides a public Anonymous Free Tier (20 requests/minute) that requires no login. If an expired session cannot be refreshed, the application should maintain full read capabilities rather than blocking user interactions.

## Decision

1. **In-Memory Token Storage**:
   - Access tokens and refresh tokens are stored exclusively in JavaScript runtime memory (React Context and closure state).
   - No authentication tokens are written to `localStorage`, `sessionStorage`, or cookies.

2. **Single-Flight Refresh Mutex**:
   - HTTP client interceptors queue concurrent 401 errors behind a single shared refresh promise (`refreshQueue.ts`).
   - The first failed request initiates `POST /api/v1/auth/refresh` with an idempotency key.
   - All pending requests wait for this single refresh call to finish. On success, pending requests receive the new access token and retry their original HTTP requests.

3. **Silent Anonymous Fallback**:
   - If token refresh fails (due to session expiration, network error, or invalid credentials) or when the user reloads the browser window, authentication state resets to anonymous without disruptive UI errors.
   - The application continues serving price comparisons and calculations seamlessly under the Anonymous Free Tier rate limit.

## Consequences

### Positive

- **Eliminates XSS Token Extraction**: Malicious scripts cannot read tokens from web storage because tokens exist only in memory.
- **Prevents Token Family Revocation**: Single-flight refresh mutex guarantees that only one request consumes a refresh token at any time.
- **Resilient User Experience**: Application features remain functional under Anonymous Free Tier when sessions expire.

### Negative

- Refreshing the browser page clears tokens from memory and returns the user to the Anonymous Free Tier unless they sign in again.

## Alternatives Considered

### Alternative 1: LocalStorage / SessionStorage Persistence
Rejected. Web storage exposes tokens to XSS extraction vulnerabilities and violates security guidelines.

### Alternative 2: Uncoordinated Concurrent Refresh Calls
Rejected. Independent refresh calls produce race conditions that trigger ADR 0021 theft containment and revoke valid token families.
