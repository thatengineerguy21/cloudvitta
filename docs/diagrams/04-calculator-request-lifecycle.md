# Calculator Request Lifecycle

This document describes the request lifecycles for single-category pricing lookups, composite multi-service workload calculations, and provider health/freshness checks.

---

## 1. Single-Category Pricing Lifecycle (Cache-Miss Flow)

The following diagram illustrates a request to `GET /api/v1/prices/{category}` (e.g., compute, storage, network) when the requested pricing slice is not present in the Redis cache.

```mermaid
sequenceDiagram
    autonumber
    participant Client as Client (Browser / CLI / Agent)
    participant CORS as CORS Middleware
    participant AuthMW as Auth Middleware
    participant RateLimit as 4-Step Tiered Rate Limiter
    participant Handler as REST Pricing Handler
    participant Svc as PricingService
    participant Cache as Redis Cache
    participant SF as singleflight.Group
    participant DB as Postgres (price_observations)
    participant Match as Matching Engine (CategoryScorer)
    participant Fresh as FreshnessService

    Client->>CORS: GET /api/v1/prices/{category}?params...
    Note over CORS: Check Origin against AllowedOrigins<br/>Set Vary: Origin & Allow-Credentials (if matched)
    
    CORS->>AuthMW: Forward Request
    Note over AuthMW: Parse Authorization: Bearer <jwt><br/>Attach UserClaims to Context (if valid)
    
    AuthMW->>RateLimit: Forward Request
    Note over RateLimit: 1. Evaluate Blunt IP Ceiling (60 req/min)<br/>2. If User -> Standard Tier (120 req/min)<br/>3. Else if cv_anon_id -> Free Tier (20 req/min)<br/>4. Else -> IP Free Tier (20 req/min) & mint cookie
    
    RateLimit->>Handler: Forward Request within rate limit
    Handler->>Handler: Parse & Validate Query Parameters
    Handler->>Svc: GetPrices(ctx, filter)
    
    %% Cache lookup
    Svc->>Cache: GET v1:{provider}:{category}:{region}
    
    alt Cache Hit
        Cache-->>Svc: Cached Observation Slice (JSON)
    else Cache Miss
        Cache-->>Svc: nil (Key Miss / Expired TTL)
        Svc->>SF: DoChan(cache_key, queryFn)
        
        critical Singleflight DB Query Collapse
            SF->>DB: GetNormalizedPricesByProviderCategoryRegion(provider, category, region)
            DB-->>SF: []price_observations rows
            SF->>Cache: SETEX v1:{provider}:{category}:{region} (TTL 5-15 min)
            SF-->>Svc: Normalized Observations
        end
    end
    
    %% Scoring and Matching
    Svc->>Match: MatchObservations(requested_specs, observations)
    Note over Match: Calculate weighted distance<br/>Assign match_quality (exact / close / loose)<br/>Resolve ties & surface alternatives
    Match-->>Svc: Matched Price Result
    
    %% Freshness Evaluation
    Svc->>Fresh: IsStale(provider, category, fetched_at)
    Fresh-->>Svc: is_stale boolean
    
    Svc-->>Handler: Domain Pricing Response
    Handler-->>Client: 200 OK JSON Response + X-RateLimit-* Headers
```

---

## 2. Composite Calculation Lifecycle (`POST /api/v1/calculate`)

The following diagram illustrates the composite workload calculation lifecycle across multiple providers and categories, enforcing the ADR 0022 Honesty Contract.

```mermaid
sequenceDiagram
    autonumber
    participant Client as Client Application
    participant Router as REST Router / Middleware
    participant CalcHandler as CalculateHandler
    participant CalcSvc as CalculateService
    participant PricingSvc as PricingService
    participant Scorer as Strategy Scorers
    participant CalcLogic as Pricing Calculations (Decimal)
    participant Fresh as FreshnessService

    Client->>Router: POST /api/v1/calculate {workload payload}
    Router->>CalcHandler: Authenticated & Rate-Limited Request
    CalcHandler->>CalcHandler: Decode JSON Body & Validate Workload Specs
    CalcHandler->>CalcSvc: Calculate(ctx, request)
    
    par Concurrent Category Evaluation via errgroup
        CalcSvc->>PricingSvc: Evaluate Compute (AWS, Azure, GCP)
        PricingSvc->>Scorer: Score Compute (vCPU, RAM, architecture)
        Scorer-->>PricingSvc: Compute Matches
        PricingSvc-->>CalcSvc: Compute Results
    and
        CalcSvc->>PricingSvc: Evaluate Storage (AWS, Azure, GCP)
        PricingSvc->>Scorer: Score Storage (tier, capacity)
        Scorer-->>PricingSvc: Storage Matches
        PricingSvc-->>CalcSvc: Storage Results
    and
        CalcSvc->>PricingSvc: Evaluate Network (AWS, Azure, GCP)
        PricingSvc->>Scorer: Score Network (egress GB, direction)
        Scorer-->>PricingSvc: Network Matches
        PricingSvc-->>CalcSvc: Network Results
    end

    CalcSvc->>CalcLogic: Compute Hourly Breakdown per Provider (decimal.Decimal)
    
    loop For each provider
        CalcSvc->>Fresh: Check Provider Freshness
        Fresh-->>CalcSvc: Stale Flags & Warnings
        
        alt All Requested Categories Available
            CalcSvc->>CalcSvc: partial = false<br/>total_normalized_hourly_usd = sum(items)
        else Missing One or More Requested Categories (ADR 0022)
            CalcSvc->>CalcSvc: partial = true<br/>omit total_normalized_hourly_usd<br/>partial_total_normalized_hourly_usd = sum(available_items)<br/>Attach category_not_supported / not_yet_ingested warning
        end
    end
    
    CalcSvc-->>CalcHandler: CalculationResult Domain Model
    CalcHandler-->>Client: 200 OK CalculationResponse JSON
```

---

## 3. Provider Status & Freshness Lifecycle (`GET /api/v1/providers/{provider}/status`)

The following diagram illustrates provider operational health and data freshness inspection.

```mermaid
sequenceDiagram
    autonumber
    participant Client as Client Application
    participant Handler as ProviderStatusHandler
    participant FreshSvc as FreshnessService
    participant DB as Postgres (price_observations)
    participant DLQ as Redis DLQ (ingestion:dlq)

    Client->>Handler: GET /api/v1/providers/{provider}/status
    Handler->>FreshSvc: GetProviderStatus(ctx, provider)
    
    alt Stage 3 Un-ingested Provider (oracle, ibm, alibaba, digitalocean)
        FreshSvc-->>Handler: ProviderStatus {status: "not_yet_ingested", stale: true}
        Handler-->>Client: 200 OK ProviderStatus JSON
    else Supported Provider (aws, azure, gcp)
        FreshSvc->>DB: GetProviderCategoryStatus(provider)
        DB-->>FreshSvc: Rows (category, last_fetched_at, last_seen_at, count)
        
        loop For each supported category
            FreshSvc->>FreshSvc: Check Timestamp Age against Staleness Threshold (168h)
            FreshSvc->>DLQ: Check Active Job Failure (provider, category)
            DLQ-->>FreshSvc: DLQ Failure Record (if present)
        end
        
        FreshSvc->>FreshSvc: Resolve Overall Status (healthy / degraded / stale / blocked)
        FreshSvc-->>Handler: ProviderStatus Domain Model
        Handler-->>Client: 200 OK ProviderStatus JSON
    else Unknown Provider Identifier
        FreshSvc-->>Handler: ErrProviderNotFound
        Handler-->>Client: 404 Not Found (RFC 7807 problem details)
    end
```

---

## 4. Key Architectural Invariants

1. **Stampede Protection**: All database fallbacks on cache misses pass through `singleflight.Group.DoChan` using `context.WithoutCancel` so client cancellations do not abort in-flight database population.
2. **Honesty Contract**: Incomplete provider comparisons set `partial: true`, omit `total_normalized_hourly_usd`, and provide `partial_total_normalized_hourly_usd` to prevent false ranking victories.
3. **Decimal Arithmetic**: All pricing sums, conversions, and breakdowns strictly use `decimal.Decimal` to eliminate floating-point rounding errors.
4. **Stateless Tiered Rate Limiting**: The 4-step rate limiter executes before pricing arithmetic, protecting backend database compute and Redis memory.
