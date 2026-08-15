# Ingestion Flow (Orchestrator + Event-driven Warming)

```mermaid
sequenceDiagram
    participant Scheduler as Cloud Scheduler
    participant Main as cmd/ingest
    participant Orch as Orchestrator
    participant Lock as Redis Lock
    participant Factory as Provider Factory
    participant Adapter as Provider Adapter
    participant API as External Provider API
    participant GCS as Google Cloud Storage
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
            
            critical Streaming JSON Normalization
                Note over Adapter: Streaming parser requires 'products' before 'terms'
                alt Schema ordering violation ('terms' before 'products')
                    Adapter-->>Orch: ErrPermanentFailure (schema shape mismatch)
                    Orch->>DLQ: Record as 'blocked' (no retry burn)
                else Valid ordering
                    Adapter->>Adapter: Stream tokens, skip unused blocks via depth tracking
                    Adapter-->>Orch: FetchResult (observations + GCS path)
                end
            end
            
            loop For each observation
                Orch->>DB: GetLatestPriceForSKU (anomaly check)
                alt Price ratio >= 10x
                    Orch->>DB: Insert with anomaly_status = 'pending_review'
                else Normal price
                    Orch->>DB: Insert with anomaly_status = NULL
                end
            end
            
            Orch->>DLQ: Clear entry on success
            Orch->>Redis: Event-driven cache warm (by region)
            Orch->>Lock: Release lock (Lua script, token-safe)
            
            alt Fetch failure (after retries exhausted or permanent failure)
                Orch->>DLQ: Record failure (provider, category, error, count)
            end
        end
    end
```

## Payload Ordering & Streaming Invariant
To guarantee memory safety in resource-constrained environments (Cloud Run 512MiB memory ceiling), provider bulk pricing parsers stream JSON tokens incrementally rather than buffering whole documents.

- **Ordering Contract:** The AWS EC2 adapter requires `products` to precede `terms`.
- **Failure Classification:** If a payload violates this ordering, parsing terminates immediately with `provider.ErrPermanentFailure`. The orchestrator bypasses retries and writes the incident directly to the Redis DLQ with `status = "blocked"`.
- **Zero-Allocation Skipping:** Non-compute products, unused pricing terms (`Reserved`, `SavingsPlans`), and metadata objects are skipped via depth-tracking token loops (`skipValue`) without allocating memory for the discarded subtrees.
