# 9. Tiered Rate Limiting with Anonymous Cookies

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta exposes a public API that requires rate limiting to protect backend resources and Upstash Redis quotas. A standard approach is to rate-limit strictly by IP address. However, IP-based limits indiscriminately group all users behind a corporate NAT, campus WiFi, or mobile carrier CGNAT, leading to unfair throttling of legitimate users.

To solve this, we designed a 4-step tiered rate limiter:
1. Authenticated JWT (`user:{id}`)
2. Anonymous Cookie (`anon:{id}`)
3. Fallback IP (`ip:{client_ip}`)
4. Overarching blunt per-IP ceiling.

## Decision

We will use an **HMAC-signed Anonymous Cookie** to track non-authenticated free-tier users, falling back to IP limits only when the cookie is absent or rejected.

## Consequences

### Positive
- Prevents legitimate users sharing a NAT from cannibalizing each other's rate limits.
- Improves rate-limiting accuracy for the common case (standard web traffic).
- Attackers attempting to bypass the cookie (e.g., by dropping it via a script) simply fall into the baseline IP limit anyway, gaining no advantage. The system remains protected while UX is preserved.

### Negative
- Slightly more complex middleware logic requires minting, signing, and verifying the `cv_anon_id` cookie on requests.
