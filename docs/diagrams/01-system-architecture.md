# System Architecture

```mermaid
flowchart TD
    subgraph "External Providers"
        P1[AWS API]
        P2[Azure API]
        P3[GCP API]
        PN[Other Providers...]
    end

    subgraph "CloudVitta Cloud Run Service (Modular Monolith)"
        direction TB
        
        subgraph "Ingestion Jobs (Cloud Scheduler Triggered)"
            I_WG[ErrGroup Concurrency]
            I_AD[Provider Adapters]
            I_RL[Rate Limiting & Retry]
            I_WG --> I_AD
            I_AD --> I_RL
        end

        subgraph "API Layer (REST / gRPC / MCP)"
            API_R[Standard Lib HTTP Routing]
            API_RL[Tiered Rate Limiting]
            API_Auth[Auth Middleware]
            API_R --> API_RL
            API_RL --> API_Auth
        end

        subgraph "Service Layer"
            S_CE[Calculator Engine]
            S_SM[SKU Matching Strategy]
            S_FX[FX Service]
            
            S_CE --> S_SM
            S_CE --> S_FX
        end

        subgraph "Data Access & Cache Layer"
            D_SQLC[Direct SQLC Querier]
            D_Cache[Cache with Singleflight]
        end

        API_Auth --> S_CE
        I_RL --> D_SQLC
        I_RL --> D_Cache
        S_CE --> D_Cache
        D_Cache -.->|Cache Miss| D_SQLC
    end

    subgraph "Storage & Infrastructure"
        DB[(Serverless Postgres - Neon)]
        Redis[(Redis / Upstash)]
        GCS[(GCS - Raw JSON Responses)]
        OTEL[Grafana Cloud / OpenTelemetry]
    end

    P1 -.-> I_WG
    P2 -.-> I_WG
    P3 -.-> I_WG
    PN -.-> I_WG

    I_AD -->|Raw Storage| GCS
    D_SQLC -->|Normalized Store| DB
    D_Cache -->|Warming / Reads| Redis
    
    %% Telemetry and DLQ
    API_Auth -.->|Traces, Metrics, Logs| OTEL
    I_WG -.->|Traces, Metrics, Logs| OTEL
    I_WG -.->|Failure Checkpoints| Redis
```
