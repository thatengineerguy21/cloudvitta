# Repository Structure

This document defines the target repository layout and directory placement rules.
Create directories only when their stage implementation requires them.

```
.
├── cmd/
│   ├── api/                     # main() entrypoint for REST and MCP service
│   ├── ingest/                  # main() entrypoint for scheduled ingestion worker
│   ├── migrate/                 # database migration runner
│   └── quarantine-digest/       # CLI utility for flagged price anomaly review
│
├── internal/
│   ├── domain/                  # pure business models (PriceObservation, MatchedSpec, etc.)
│   │
│   ├── service/                 # core domain service: Compare, Calculate, matching, FX orchestration
│   │
│   ├── adapter/
│   │   └── provider/
│   │       ├── aws/             # AWS provider adapter, fetch + normalize + supported_categories.go
│   │       ├── azure/           # Azure provider adapter, fetch + normalize + supported_categories.go
│   │       ├── gcp/             # GCP provider adapter, fetch + normalize + supported_categories.go
│   │       ├── factory.go       # provider adapter construction factory
│   │       └── retry.go         # retry and backoff policies
│   │
│   ├── fx/                      # foreign exchange service interface and implementations
│   │   └── frankfurter/         # Frankfurter (ECB) API client
│   │
│   ├── store/                   # SQLC generated database code and connection pool constructor
│   │   └── queries/             # SQL query definition files (*.sql)
│   │
│   ├── storage/                 # GCS raw payload storage interface and mocks
│   │
│   ├── cache/                   # Redis client wrapper, singleflight, and schema-versioned cache keys
│   │
│   ├── dlq/                     # lightweight Redis dead-letter queue client
│   │
│   ├── quarantine/              # quarantine sink interface and recorder for unmapped entities
│   │
│   ├── transport/
│   │   ├── rest/                # HTTP REST transport handlers, routing, and RFC 7807 error mappings
│   │   └── mcp/                 # Model Context Protocol Streamable HTTP tool handlers
│   │
│   ├── config/                  # Koanf configuration loader and startup validation
│   │
│   ├── matching/
│   │   ├── catalogmap/          # per-provider product to service category maps
│   │   ├── databaseenginemap/   # per-provider database engine canonical maps
│   │   ├── kubernetestieremap/  # per-provider Kubernetes tier canonical maps
│   │   ├── nosqldatamodelmap/   # per-provider NoSQL data model canonical maps
│   │   ├── regionmap/           # per-provider region to canonical region_group maps
│   │   ├── serverlessarchmap/   # per-provider serverless CPU architecture canonical maps
│   │   ├── serverlessunitmap/   # per-provider serverless rate unit canonical maps
│   │   ├── storageclassmap/     # per-provider storage class canonical maps
│   │   └── transfertypemap/     # per-provider network transfer type canonical maps
│   │
│   ├── observability/           # OpenTelemetry instrumentation (traces, metrics, logs) and slog setup
│   │
│   ├── auth/                    # bcrypt hashing, JWT access token minting, refresh token rotation
│   │
│   └── middleware/
│       ├── authmw/              # HTTP Bearer token parser and user claims injector
│       └── ratelimit/           # 4-tier rate limiting engine with HMAC cookie issuance
│
├── migrations/                  # Tern forward-only SQL migration files
│
├── docs/
│   ├── REPO-STRUCTURE.md     # repository structure reference
│   ├── DEVELOPMENT_GUIDE.md     # step-by-step setup and local development guide
│   ├── adr/                     # Architectural Decision Records (ADRs)
│   ├── devops/                  # deployment and operations runbooks
│   └── diagrams/                # architecture and lifecycle diagrams
│
├── .agents/                     # agent guidelines, context, plans, and skills
│
├── .github/
│   └── workflows/
│       ├── ci.yml               # GitHub Actions CI workflow
│       └── deploy.yml           # GitHub Actions Cloud Run deploy workflow
│
├── Taskfile.yml                 # task runner automation configuration
├── Dockerfile                   # multi-stage container build
├── sqlc.yaml                    # SQLC code generator configuration
├── .golangci.yml                # linter configuration
├── .env.example                 # sample environment variables
├── go.mod / go.sum              # Go module dependency files
└── README.md                    # project overview and quickstart documentation
```

## Placement Rules

- **Fixture Files**: Test fixtures live alongside provider adapters in `testdata/` directories, each containing recorded timestamps for freshness evaluation.
- **`internal/store`**: Contains SQLC-generated queries and the database pool constructor (`pool.go`). Code in `internal/service` interacts with `store.Querier` and never imports `pgx` driver types directly.
- **`internal/service`**: Transport-agnostic domain engine. It is the only package where matching algorithms, pricing math, and totaling occur.
- **Transport Packages**: `internal/transport/rest` and `internal/transport/mcp` handle protocol encoding, parameter extraction, and input validation before delegating to `service`.
