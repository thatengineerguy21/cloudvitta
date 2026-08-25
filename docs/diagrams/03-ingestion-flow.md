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
                    
                    alt Unmapped Taxonomy Encountered (Curated Maps)
                        Adapter->>QSink: Route unmapped token to quarantine.Sink
                        Note over Adapter: Continue parsing valid items without aborting stream
                    end
                    
                    Note over Adapter: Multi-Component Observation Splits:<br/>1. RDBMS -> instance + storage rows (ADR 0030)<br/>2. NoSQL -> throughput + storage rows (ADR 0033)<br/>3. Serverless -> request_fee + duration_fee (CPU/Mem)<br/>4. Multi-Provider Parity (AWS, Azure, GCP, Oracle, IBM, Alibaba, DigitalOcean)
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

## Runtime Taxonomy Isolation via Quarantine Sink & 3-Way Classification

When provider APIs return raw pricing streams, the normalizer triages records into three distinct classifications:
1. **In-Scope Valid (`ItemClassificationNormalized`)**: Successfully parsed into domain `PriceObservation` records.
2. **In-Scope Unknown (`ItemClassificationQuarantined`)**: In-scope resources with unmapped taxonomy attributes (unknown shape, tier, region, database engine). These records route to `quarantine.Sink` and increment `UnmappedCount`.
3. **Out-of-Scope Discarded (`ItemClassificationIgnored`)**: Unmodeled provider catalog lines (such as network egress under database feeds, non-IaaS enterprise services, and unsupported commitments). These records are excluded from quarantine and increment `IgnoredCount`.

### Threshold Invariant Formula
To prevent out-of-scope catalog noise from skewing the quarantine threshold, the unmapped ratio is computed strictly against in-scope items:
```
in_scope_total = unmapped_count + valid_observations_count
ratio = unmapped_count / in_scope_total
```
- If `ratio > MaxUnmappedRatio` (default 5%), the job fails with `ErrPermanentFailure` and logs a blocked incident to Redis DLQ to alert engineers of breaking upstream API taxonomy changes.
- If `ratio <= MaxUnmappedRatio`, valid observations proceed to database upsert and cache warming.
- Quarantined records are flushed to GCS at `quarantine/<provider>/<category>/<date>/<fetchID>.jsonl` and digested with `cmd/quarantine-digest`.
