# 29. CORS Narrowing with Credentialed Cookies

Date: 2026-08-16

## Status

Accepted (Resolves Open Question #17)

## Context

In Stage 0, CloudVitta served a permissive Cross-Origin Resource Sharing (CORS) policy (`Access-Control-Allow-Origin: *`) across all public read endpoints.
The original justification stated that no sessions, cookies, or user private data existed on the system.

In Sub-stage 1.10, the system introduces the `cv_anon_id` tracking cookie and user authentication tokens.
Browser security standards (W3C CORS and Fetch API specification) enforce two strict rules:
1. An HTTP response cannot combine `Access-Control-Allow-Origin: *` with `Access-Control-Allow-Credentials: true`.
2. Setting tracking cookies alongside wildcard origins creates security risks where untrusted origins can perform ambient requests.

## Decision

We narrow the CORS policy on all REST endpoints through `internal/transport/rest/middleware/cors.go`:

1. **Configurable Allowed Origins**:
   - The application configuration defines an explicit list of allowed origins (`cfg.CORS.AllowedOrigins`).
   - If an incoming request contains an `Origin` header that matches the allowed origin list (or configured subdomain patterns such as `https://*.cloudvitta.dev`):
     - The server reflects the request origin in `Access-Control-Allow-Origin: <origin>`.
     - The server sets `Access-Control-Allow-Credentials: true`.
     - The server sets `Vary: Origin` to ensure HTTP caches partition responses correctly.

2. **Disallowed Origins**:
   - If an incoming browser request has an `Origin` header not in the allowed list, the server does not emit `Access-Control-Allow-Origin`.
   - The web browser blocks the response from untrusted scripts.

3. **Wildcard Fallback for Non-Credentialed Access**:
   - If the configuration explicitly sets `AllowedOrigins = ["*"]` with `AllowCredentials = false`, the server serves `Access-Control-Allow-Origin: *` without credentials.

4. **Preflight OPTIONS Handling**:
   - HTTP `OPTIONS` preflight requests receive:
     - `Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS`
     - `Access-Control-Allow-Headers: Authorization, Content-Type, Idempotency-Key, X-Requested-With`
     - `Access-Control-Max-Age: 86400` (24-hour cache duration)
     - HTTP Status `204 No Content`.

## Consequences

### Positive
- Complies with browser security standards and allows browsers to store and transmit `cv_anon_id` cookies safely.
- Protects user authentication credentials from cross-origin access by unauthorized domains.
- Prevents cache poisoning by setting `Vary: Origin`.

### Negative
- Frontend applications and test environments must configure their origin in `CLOUDVITTA_CORS_ALLOWED_ORIGINS` to perform cross-origin requests with credentials.

## Alternatives Considered

### Alternative 1: Permissive Wildcard CORS (`Access-Control-Allow-Origin: *`)
Rejected. W3C CORS and modern browser specifications forbid combining `Access-Control-Allow-Credentials: true` with wildcard origins. Maintaining a wildcard policy prevents browsers from transmitting or persisting authentication and anonymous tracking cookies.

### Alternative 2: Reverse Proxy Origin Rewriting Without CORS Middleware
Rejected. Requiring an external proxy (such as Nginx or Cloudflare) to manage CORS headers complicates local developer setups and couples application security behavior to external routing infrastructure.
