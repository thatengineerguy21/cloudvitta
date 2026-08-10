# 3. Cache-aside with Event-driven Warming

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta serves live API requests requiring rapid, concurrent cloud price comparisons. Querying the normalized Postgres store on every request is too slow and would place undue load on the database. 

We must cache pricing data in Upstash Redis, keyed by `(provider, service_category, region)`. However, we need a strategy to keep the cache in sync with the database when the ingestion jobs fetch new prices, without introducing extreme architectural complexity (like a transactional outbox) or burning through Upstash Redis's free tier quota (500K commands/month).

## Decision

We will use a **Cache-Aside pattern combined with Event-Driven Warming** and a safety TTL:
1. **Event-driven Warming**: The ingestion job writes fresh data to Postgres and then immediately attempts to write the dataset to Redis.
2. **Acceptable Staleness (No Outbox)**: We accept a bounded staleness window. If the Redis write fails due to a network blip, we will *not* retry via a complex transactional outbox. The database and cache will temporarily diverge.
3. **TTL Self-Healing**: Redis keys will have a mandatory TTL (e.g., 5-15 minutes, particularly scoped for volatile future stages like Spot instances). When the TTL expires, the next API request triggers a cache miss, fetches fresh data from Postgres via `singleflight`, and rewrites the cache.
4. **Quota Protection**: To protect the 500K/month Upstash command quota, event-driven warming will be throttled/batched. We will not proactively blast Redis with every micro-fluctuation of a Spot price; instead, we rely on the TTL bounds and periodic updates to batch writes and preserve the quota.

## Consequences

### Positive
- Vastly simpler ingestion architecture without distributed transaction managers or outboxes.
- Protects Redis command quotas by accepting eventual consistency.
- Reads are highly performant and protected from stampedes via `singleflight`.

### Negative
- Users may occasionally see data that is up to 15 minutes stale compared to what CloudVitta has ingested in Postgres.
- Requires strict configuration of TTLs to balance staleness requirements against Postgres query load.
