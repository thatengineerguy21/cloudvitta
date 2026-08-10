# 14. Schema-Versioned Cache Keys

Date: 2026-08-06

## Status

Accepted

## Context

The cache keys in Upstash Redis map to specific `(provider, service_category, region)` combinations. Over time, the required JSON structure of the cached attribute vector will change (e.g., adding a new dimension). 

We needed a strategy to handle cache entries when the application schema evolves, specifically choosing between writing complex in-place migration code (to upgrade old cache JSON to the new shape on read) versus simply versioning the cache keys and letting old keys expire.

## Decision

We will use **Schema-Versioned Cache Keys** (e.g., `v1:aws:compute:us-east-1` transitioning to `v2:aws:compute:us-east-1`).
1. We explicitly reject writing in-place cache migration code.
2. Old `v1` keys will naturally age out and be purged by their Redis TTL.
3. To mitigate the "thundering herd" (a 100% cache miss rate) when `v2` containers deploy and look for non-existent `v2` keys, we implement a three-pronged defense:
   - **Pre-warming Job**: Proactively bulk-run the normal fetch-and-cache path under the new `v2` prefix for known hot keys before the traffic cutover.
   - **Canary Rollout**: Shift traffic to `v2` containers gradually to spread the remaining cache-miss wave over time.
   - **Database Concurrency Limiter**: Cap the worst-case simultaneous database load on the Postgres fetch path.

## Consequences

### Positive
- Prevents complex, brittle legacy migration logic from polluting the `internal/cache` layer.
- `golang.org/x/sync/singleflight` collapses duplicate concurrent requests for the *same* missing key, while the pre-warming and canary rollout prevent a spike of distinct missing keys.
- Ensures the cache format is always perfectly aligned with the running Go structs.

### Negative
- Requires a deliberate pre-warming and canary deployment strategy for major schema updates.
- Temporarily doubles Redis memory usage during the rollout window while both `v1` and `v2` keys coexist before the `v1` TTL expires.
