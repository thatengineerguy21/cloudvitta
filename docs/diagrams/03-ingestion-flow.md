# Ingestion Flow (Orchestrator, Multi-Component Normalization, & Quarantine Sink)

```mermaid
sequenceDiagram
    autonumber
    participant Scheduler as Cloud Scheduler
    participant Main as cmd/ingest
    participant Orch as Orchestrator
    participant Lock as Redis Lock
    participant Factory as Provider Factory
    participant Adapter as Provider Adapter
    participant API as External Provider API
    participant GCS as Google Cloud Storage
    participant QSink as Quarantine Sink (GCS / Memory)
    participant DB as Postgres (Neon)
    participant Redis as Redis / Upstash
    participant DLQ as Redis DLQ

    Scheduler->>Main: Trigger run
    Main->>Factory: BuildJobs()
    Factory-->>Main: []Job (provider, category, limiter, retry)
    Main->>Orch: RunAll(ctx)
    
    par For each (provider, category) job via errgroup.SetLimit
        Orch->>Lock: AcquireIngestionLock (SET NX EX)
        alt Lock already held
            Lock-->>Orch: ErrLockHeld → skip job
        else Lock acquired
            Orch->>Adapter: Fetch (with retry + rate limiting)
            
            loop Retry (max 3, backoff 5s→60s with jitter)
                Adapter->>API: HTTP Request (token bucket rate limited)
                alt 429 Too Many Requests
                    API-->>Adapter: Retry-After or doubled backoff
                else 401/403 Auth Error
                    API-->>Adapter: Fail fast → ErrPermanentFailure
                else 5xx / Timeout
                    API-->>Adapter: Exponential backoff + jitter
                else 200 OK
                    API-->>Adapter: Raw JSON Response
                end
            end

            Adapter->>GCS: Store Raw Response (before normalizing)
            GCS-->>Adapter: Return Storage Ref
            
            critical Streaming JSON Normalization & Multi-Component Split
                Note over Adapter: Streaming parser requires 'products' before 'terms'
                alt Schema ordering violation ('terms' before 'products')
                    Adapter-->>Orch: ErrPermanentFailure (schema shape mismatch)
                    Orch->>DLQ: Record as 'blocked' (no retry burn)
                else Valid ordering
                    Adapter->>Adapter: Stream tokens, skip unused blocks via depth tracking
                    
                    alt Unmapped Taxonomy Encountered (5 Curated Maps)
                        Adapter->>QSink: Route unmapped token to quarantine.Sink
                        Note over Adapter: Continue parsing valid items without aborting stream
                    end
                    
                    Note over Adapter: Multi-Component Observation Splits:<br/>1. RDBMS -> instance + storage rows (ADR 0030)<br/>2. NoSQL -> throughput + storage rows (ADR 0033)<br/>3. Serverless -> request_fee + duration_fee (CPU/Mem)
                    Adapter-->>Orch: FetchResult (observations + unmappedCount + GCS path)
                end
            end
            
            alt Unmapped Ratio Exceeds Threshold (> 5%)
                Orch->>DLQ: Record as 'blocked' (breaking API change detected)
            else Normal Unmapped Ratio (<= 5%)
                loop For each observation
                    Orch->>DB: GetLatestPriceForSKU (anomaly check)
                    alt Price ratio >= 10x
                        Orch->>DB: Upsert with anomaly_status = 'pending_review'
                    else Price unchanged
                        Orch->>DB: Update last_seen_at timestamp
                    else Price changed
                        Orch->>DB: Insert new row with anomaly_status = NULL
                    end
                end
                
                Orch->>DLQ: Clear entry on success
                Orch->>Redis: Event-driven cache warm (by native region)
                Orch->>Lock: Release lock (Lua script, token-safe)
            end
            
            alt Fetch failure (after retries exhausted or permanent failure)
                Orch->>DLQ: Record failure (provider, category, error, count)
            end
        end
    end
```

---

## Payload Ordering & Streaming Invariants

To guarantee memory safety in resource-constrained environments (Cloud Run 512MiB memory ceiling), provider bulk pricing parsers stream JSON tokens incrementally rather than buffering whole documents.

- **Ordering Contract:** The AWS EC2 adapter requires `products` to precede `terms`.
- **Failure Classification:** If a payload violates this ordering, parsing terminates immediately with `provider.ErrPermanentFailure`. The orchestrator bypasses retries and writes the incident directly to the Redis DLQ with `status = "blocked"`.
- **Zero-Allocation Skipping:** Non-compute products, unused pricing terms (`Reserved`, `SavingsPlans`), and metadata objects are skipped via depth-tracking token loops (`skipValue`) without allocating memory for the discarded subtrees.

---

## Multi-Component Observation Splitting

To prevent Cartesian explosion in the database, multi-meter services are ingested as distinct component rows in `price_observations` with `service_category` and `attributes.component_type`:

1. **Relational Databases (`database_rdbms` - ADR 0030)**:
   - Compute instances: `component_type = "instance"` (hourly rate).
   - Storage capacity: `component_type = "storage"` (monthly rate per GB, joined at query time).
2. **NoSQL Databases (`database_nosql` - ADR 0033)**:
   - Operational throughput: `component_type = "throughput"` (provisioned RCU/WCU/RU or on-demand operations).
   - Storage capacity: `component_type = "storage"` (monthly rate per GB, joined at query time).
3. **Serverless Compute (`serverless`)**:
   - Request fees: `rate_component = "request_fee"` (rate per 1M requests).
   - Duration fees: `rate_component = "duration_fee"` (rate per GB-second) or split `duration_fee_cpu` and `duration_fee_memory` (GCP).

---

## Runtime Taxonomy Isolation via Quarantine Sink

When provider APIs return new or unrecognized taxonomy values (regions, storage classes, transfer types, database engines, Kubernetes tiers, or serverless units):
1. The normalizer isolates the unmapped record to `quarantine.Sink`.
2. Parsing continues for all valid items in the stream.
3. Quarantined records are flushed to GCS at `quarantine/<provider>/<category>/<date>/<fetchID>.jsonl`.
4. If `unmapped / (unmapped + valid) > MaxUnmappedRatio` (default 5%), the job fails loudly with `ErrPermanentFailure` and logs an incident to Redis DLQ to alert engineering of upstream API breaking changes.
5. Quarantined records can be digested into taxonomy PR code using `cmd/quarantine-digest`.
