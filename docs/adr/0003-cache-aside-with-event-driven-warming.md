# 3. Cache-aside with Event-driven Warming
 
 Date: 2026-08-06
 
 ## Status
 
 Accepted
 
 ## Context
 
 CloudVitta serves live API requests requiring rapid, concurrent cloud price comparisons. Querying the normalized Postgres store on every request is too slow and places heavy load on the database.
 
 We must cache pricing data in Redis (Upstash), keyed by `(provider, service_category, region)`. However, we need a strategy to keep the cache synchronized with the database when ingestion jobs fetch new prices, without introducing extreme architectural complexity (such as a transactional outbox) or exhausting Upstash Redis command quotas (500,000 commands/month on free tier).
 
 ## Decision
 
 We use a **Cache-Aside pattern combined with Event-Driven Warming** and a safety TTL:
 1. **Event-Driven Warming**: Ingestion jobs write fresh pricing data to Postgres and immediately attempt to write the dataset to Redis.
 2. **Acceptable Staleness (No Outbox)**: We accept a bounded staleness window. If a Redis write fails due to a network interruption, the system does not retry via a transactional outbox. The database and cache temporarily diverge until TTL expiry or the next scheduled run.
 3. **TTL Self-Healing**: Redis keys have a mandatory TTL (5 to 15 minutes). When the TTL expires, the next API request triggers a cache miss, fetches fresh data from Postgres via `singleflight`, and repopulates the cache.
 4. **Quota Protection**: To protect the Upstash command quota, event-driven warming is batched by region. The system does not write micro-updates on every price tick; it batches writes and relies on bounded TTLs.
 
 ## Consequences
 
 ### Positive
 - Vastly simpler ingestion architecture without distributed transaction managers or outboxes.
 - Protects Redis command quotas by accepting eventual consistency.
 - API reads are performant and protected from thundering herd stampedes via `singleflight`.
 
 ### Negative
 - Users can occasionally observe data that is up to 15 minutes stale relative to recent database insertions.
 - Requires disciplined configuration of TTLs to balance staleness requirements against Postgres query load.
 
 ## Alternatives Considered
 
 ### Alternative 1: Read-Through Cache with Transactional Outbox (CDC / Debezium)
 Rejected. A transactional outbox with Change Data Capture requires dedicated infrastructure components (Debezium/Kafka), increasing maintenance overhead when bounded eventual consistency is sufficient.
 
 ### Alternative 2: Direct PostgreSQL Queries on Every Request
 Rejected. Querying PostgreSQL on every calculation request saturates database connection pools and causes latency spikes under concurrent comparison workloads.
 
 ### Alternative 3: Synchronous Write-Through Cache with Distributed 2-Phase Commit
 Rejected. Synchronous distributed transactions tightly couple background ingestion to external cache availability, causing ingestion pipelines to fail whenever Redis encounters transient connectivity issues.
