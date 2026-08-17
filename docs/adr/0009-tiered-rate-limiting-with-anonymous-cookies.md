# 9. Tiered Rate Limiting with Anonymous Cookies

Date: 2026-08-06 (Updated: 2026-08-16)

## Status

Accepted

## Context

CloudVitta provides a public API. The service requires rate limiting to protect backend database resources and Upstash Redis quotas.
A simple rate limit by IP address groups all users behind a corporate Network Address Translation (NAT), university network, or mobile gateway.
This causes unfair throttling for legitimate users who share one public IP address.

## Decision

We use a centralized 4-step rate limiting engine (`internal/middleware/ratelimit`) backed by Redis.
The engine executes the following evaluation steps in sequence:

1. **Step 4 (Outer Blunt IP Ceiling)**:
   - Evaluates key `ratelimit:blunt_ip:{client_ip}:{minuteWindow}`.
   - Enforces an outer ceiling limit of 60 requests per minute across all requests from the client IP address.
   - Blocks automated clients from bypassing tier limits through cookie rotation.

2. **Step 1 (Standard Tier - Authenticated User)**:
   - Triggered when the request contains a valid JWT access token (`Authorization: Bearer <token>`).
   - Evaluates key `ratelimit:user:{user_id}:{minuteWindow}`.
   - Enforces a standard tier limit of 120 requests per minute.

3. **Step 2 (Free Tier - Anonymous Tracking Cookie)**:
   - Triggered when the request contains a valid stateless `cv_anon_id` cookie.
   - The cookie value uses format `<uuid>.<timestamp>.<hmac_sha256_hex>`.
   - The server verifies the HMAC signature and timestamp (90-day time-to-live) statelessly without database storage.
   - Evaluates key `ratelimit:anon:{anon_id}:{minuteWindow}`.
   - Enforces a free tier limit of 20 requests per minute.

4. **Step 3 (Free Tier - Client IP Fallback and Cookie Minting)**:
   - Triggered when the request has no valid JWT and no valid cookie (such as command line tools or initial browser visits).
   - Evaluates key `ratelimit:ip:{client_ip}:{minuteWindow}`.
   - Enforces a free tier limit of 20 requests per minute.
   - Mints a fresh HMAC-signed `cv_anon_id` cookie and attaches it via the `Set-Cookie` HTTP response header.

5. **Response Headers**:
   - `X-RateLimit-Limit`: Maximum requests permitted in the current 1-minute window.
   - `X-RateLimit-Remaining`: Remaining requests permitted in the current window.
   - `X-RateLimit-Reset`: Unix timestamp in seconds when the current window resets.
   - `Retry-After`: Seconds remaining in window when HTTP 429 occurs.

6. **Fail-Open Policy**:
   - If Redis returns an error or is unreachable, the rate limiter logs a warning and permits the request to proceed without blocking users.

## Consequences

### Positive
- Legitimate users behind shared NAT networks do not exhaust each other's rate limits.
- The server stays stateless because cookie validation uses HMAC-SHA256 signatures and timestamps.
- Malicious clients cannot bypass rate limits by dropping or cycling cookies because the blunt IP ceiling limits all requests from that IP address.
- Response headers provide clear rate status to clients.

### Negative
- Middleware must calculate HMAC signatures and manage Redis keys on every request.

## Alternatives Considered

### Alternative 1: Single IP-Only Rate Limiting
Rejected. Enforcing rate limits solely by IP address groups all users behind a shared Network Address Translation (NAT), corporate proxy, or mobile gateway, unfairly throttling legitimate concurrent users.

### Alternative 2: Mandatory User Authentication for All Endpoints
Rejected. Requiring mandatory signup/login for simple pricing lookups creates high friction for evaluation, interactive documentation exploration, and lightweight developer tools.

### Alternative 3: Server-Side Stateful Session Store for Anonymous Users
Rejected. Storing anonymous visitor session tokens in PostgreSQL or Redis incurs database write amplification and consumes database memory for transient single-request visitors. Stateless HMAC-signed cookies avoid server-side storage overhead.
