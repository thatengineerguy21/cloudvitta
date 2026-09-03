# Calculator Request Lifecycle

This document describes the request lifecycles for single-category pricing lookups, composite multi-service workload calculations, and provider health/freshness checks.

---

## 1. Single-Category Pricing Lifecycle (Cache-Miss Flow)

The following diagram illustrates a request to `GET /api/v1/prices/{category}` across all seven supported categories (`compute`, `storage`, `network`, `database`, `database-nosql`, `kubernetes`, `serverless`) when the requested pricing slice is not present in the Redis cache.

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
    Note over Match: Category-specific Strategy Scorer<br/>Assign match_quality (exact / close / approximate)<br/>Elastic categories (storage, network) match qualitative specs;<br/>volume applies as cost multiplier in pricing arithmetic.<br/>Dynamic join for DB & NoSQL candidates
    Match-->>Svc: Matched Price Result
    
    %% Freshness Evaluation
    Svc->>Fresh: IsStale(provider, category, fetched_at)
    Fresh-->>Svc: is_stale boolean
    
    Svc-->>Handler: Domain Pricing Response
    Handler-->>Client: 200 OK JSON Response + X-RateLimit-* Headers
```

---

## 2. Composite Calculation Lifecycle (`POST /api/v1/calculate`)

The following diagram illustrates the composite workload calculation lifecycle across all seven categories via the `calculateCategoryRegistry` (ADR 0031), supporting request alias conflict detection (ADR 0032) and enforcing the ADR 0022 Honesty Contract.

```mermaid
sequenceDiagram
    autonumber
    participant Client as Client Application
    participant Router as REST Router / Middleware
    participant CalcHandler as CalculateHandler
    participant CalcSvc as CalculateService
    participant Reg as calculateCategoryRegistry
    participant PricingSvc as PricingService
    participant Scorer as Strategy Scorers
    participant CalcLogic as Pricing Calculations (Decimal)
    participant Fresh as FreshnessService

    Client->>Router: POST /api/v1/calculate {workload payload}
    Router->>CalcHandler: Authenticated & Rate-Limited Request
    CalcHandler->>CalcHandler: Decode JSON Body & Validate Workload Specs
    Note over CalcHandler: ResolveAliasedField (database vs database_rdbms)<br/>Reject conflicting specs with 400 Bad Request
    CalcHandler->>CalcSvc: Calculate(ctx, request)
    
    CalcSvc->>Reg: Extract MatchTargets for Registered Categories
    Reg-->>CalcSvc: Map[Category]MatchTarget (compute, storage, network, db, nosql, k8s, serverless)
    
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
    and
        CalcSvc->>PricingSvc: Evaluate Relational Database (AWS, Azure, GCP)
        PricingSvc->>Scorer: Score RDBMS (Instance + Storage Join, IOPS)
        Scorer-->>PricingSvc: Database Matches
        PricingSvc-->>CalcSvc: Database Results
    and
        CalcSvc->>PricingSvc: Evaluate NoSQL Database (AWS, Azure, GCP)
        PricingSvc->>Scorer: Score NoSQL (Throughput + Storage Join, 1 KB payload)
        Scorer-->>PricingSvc: NoSQL Matches
        PricingSvc-->>CalcSvc: NoSQL Results
    and
        CalcSvc->>PricingSvc: Evaluate Kubernetes Control Plane (AWS, Azure, GCP)
        PricingSvc->>Scorer: Score Kubernetes (Conditional GKE Credit)
        Scorer-->>PricingSvc: Kubernetes Matches
        PricingSvc-->>CalcSvc: Kubernetes Results
    and
        CalcSvc->>PricingSvc: Evaluate Serverless Compute (AWS, Azure, GCP)
        PricingSvc->>Scorer: Score Serverless (Duration + Request Netting)
        Scorer-->>PricingSvc: Serverless Matches
        PricingSvc-->>CalcSvc: Serverless Results
    end

    CalcSvc->>CalcLogic: Compute Category Hourly/Monthly Totals (decimal.Decimal)
    
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
    
    alt Stage 4 Un-ingested Provider (oracle, ibm, alibaba, digitalocean)
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
        
        FreshSvc->>FreshSvc: Resolve Overall Status (healthy / partially_healthy / degraded / stale / blocked)
        FreshSvc-->>Handler: ProviderStatus Domain Model
        Handler-->>Client: 200 OK ProviderStatus JSON
    else Unknown Provider Identifier
        FreshSvc-->>Handler: ErrProviderNotFound
        Handler-->>Client: 404 Not Found (RFC 7807 problem details)
    end
```

---

## 4. Model Context Protocol (MCP) Streamable HTTP Lifecycle (`/mcp`)

The following diagram illustrates how AI agent clients connect, authenticate, discover tools, and invoke comparison operations over Streamable HTTP transport across all nine registered MCP tools.

```mermaid
sequenceDiagram
    autonumber
    participant Agent as AI Agent Client (Claude / Custom)
    participant AuthMW as Auth Middleware (Extract Claims)
    participant Guard as RequireAuth Guard
    participant RateLimit as Rate Limiter (Standard Tier)
    participant MCPHandler as Streamable HTTP Handler
    participant MCPServer as MCP Server Adapter
    participant Svc as PricingService / FreshnessService

    Agent->>AuthMW: POST /mcp (JSON-RPC initialize / tools/list / tools/call)
    Note over AuthMW: Parse Authorization: Bearer <jwt><br/>Attach AuthContext to Request Context
    
    AuthMW->>Guard: Forward Request
    alt Unauthenticated (Missing / Invalid Bearer Token)
        Guard-->>Agent: 401 Unauthorized (application/problem+json)
    else Authenticated User Context Present
        Guard->>RateLimit: Forward Request
        Note over RateLimit: User-keyed Standard Tier Quota<br/>Key: ratelimit:user:{user_id}:{window}<br/>Quota: 120 req/min
        
        alt Quota Exceeded
            RateLimit-->>Agent: 429 Too Many Requests + Retry-After
        else Within Quota
            RateLimit->>MCPHandler: Forward Request + RateLimit Headers
            MCPHandler->>MCPServer: Dispatch JSON-RPC Method
            
            alt tools/list
                MCPServer-->>MCPHandler: Return 9 Tool Schemas (JSON)
                MCPHandler-->>Agent: 200 OK ListToolsResult
            else tools/call (9 Tools)
                Note over MCPServer: compare_compute, compare_storage, compare_network,<br/>compare_database, compare_database_nosql,<br/>compare_kubernetes, compare_serverless,<br/>calculate_workload, get_provider_status
                MCPServer->>Svc: Invoke Unified Pricing / Freshness Service
                Svc-->>MCPServer: Calculated Domain Results + Honesty Attributes
                MCPServer-->>MCPHandler: CallToolResult (JSON Content)
                MCPHandler-->>Agent: 200 OK CallToolResult
            end
        end
    end
```

---

## 5. Key Architectural Invariants

1. **Stampede Protection**: All database fallbacks on cache misses pass through `singleflight.Group.DoChan` using `context.WithoutCancel` so client cancellations do not abort in-flight database population.
2. **Honesty Contract**: Incomplete provider comparisons set `partial: true`, omit `total_normalized_hourly_usd`, and provide `partial_total_normalized_hourly_usd` to prevent false ranking victories (ADR 0022).
3. **Decimal Arithmetic**: All pricing sums, conversions, and breakdowns strictly use `decimal.Decimal` to eliminate floating-point rounding errors (ADR 0015).
4. **Stateless Tiered Rate Limiting**: The 4-step rate limiter executes before pricing arithmetic, protecting backend database compute and Redis memory (ADR 0009).
5. **Strict MCP Authentication**: MCP tool access requires a valid JWT Bearer token and enforces the 120 req/min Standard Tier quota keyed by `user_id`.
6. **Extensible Registries**: Category serialization, calculate dispatch, and pricing arithmetic use strategy registries, eliminating monolithic switch blocks (ADR 0031).
7. **Alias Conflict Resolution**: Conflicting specifications between alias and canonical request parameters are rejected with HTTP 400 (ADR 0032).
