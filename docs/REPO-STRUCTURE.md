# Repository Structure

This document defines the target repository layout and directory placement rules.
Create directories only when their stage implementation requires them.

```
.
├── cmd/
│   ├── api/                     # main() entrypoint for REST and MCP service
│   ├── ingest/                  # main() entrypoint and adapter registration factory for scheduled ingestion worker
│   ├── migrate/                 # database migration runner
│   └── quarantine-digest/       # CLI utility for flagged price anomaly review
│
├── internal/
│   ├── domain/                  # pure business models (PriceObservation, ComputeCatalogItem, MatchedSpec, etc.)
│   │
│   ├── service/                 # core domain service: Compare, Calculate, CatalogService, matching, FX orchestration
│   │
│   ├── adapter/
│   │   └── provider/
│   │       ├── alibaba/         # Alibaba Cloud provider adapter, fetch + normalize + supported_categories.go
│   │       ├── aws/             # AWS provider adapter, fetch + normalize + supported_categories.go
│   │       ├── azure/           # Azure provider adapter, fetch + normalize + supported_categories.go
│   │       ├── digitalocean/    # DigitalOcean provider adapter, fetch + normalize + supported_categories.go
│   │       ├── gcp/             # GCP provider adapter, fetch + normalize + supported_categories.go
│   │       ├── ibm/             # IBM Cloud provider adapter, fetch + normalize + supported_categories.go
│   │       ├── oracle/          # Oracle OCI provider adapter, fetch + normalize + supported_categories.go
│   │       ├── factory.go       # provider adapter construction factory
│   │       └── retry.go         # retry and backoff policies
│   │
│   ├── fx/                      # foreign exchange service interface and implementations
│   │   └── frankfurter/         # Frankfurter (ECB) API client
│   │
│   ├── store/                   # SQLC generated database code, connection pool constructor with otelpgx tracing, and pool metrics
│   │   └── queries/             # SQL query definition files (*.sql)
│   │
│   ├── storage/                 # GCS and in-memory raw payload storage with OpenTelemetry tracing and latency metrics
│   │
│   ├── cache/                   # Redis client wrapper with redisotel tracing and metrics, singleflight, and schema-versioned cache keys
│   │
│   ├── dlq/                     # lightweight Redis dead-letter queue client with OpenTelemetry tracing, logging, and metrics
│   │
│   ├── quarantine/              # quarantine sink interface and recorder for unmapped entities
│   │
│   ├── transport/
│   │   ├── rest/                # HTTP REST transport handlers (compare, calculate, catalog, auth, provider status), routing, and RFC 7807 error mappings
│   │   ├── mcp/                 # Model Context Protocol Streamable HTTP tool handlers (compare, calculate, get_compute_catalog)
│   │   └── spa/                 # embedded SPA static asset delivery, SPA fallback routing, and API guards
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
├── web/                         # React, Vite, TypeScript, and Tailwind frontend SPA
│   ├── e2e/                     # Playwright end-to-end smoke test specs
│   ├── src/                     # React source code, components, design tokens, and utilities
│   │   ├── api/                 # fetch wrapper, RFC 7807 parsing, 401 mutex, TanStack Query hooks
│   │   ├── auth/                # in-memory AuthProvider and useAuth context
│   │   ├── components/          # layout, bento grid, honesty badges, auth modals, compare template, calculate builder, and catalog autocomplete/summary
│   │   ├── hooks/               # custom hooks (useUrlParams two-way query synchronization)
│   │   ├── lib/                 # formatting, workload URL serialization, query client configuration, and class merging utils
│   │   ├── pages/               # compare category views (compute, storage, network, db, k8s, serverless) and composite calculator
│   │   ├── router/              # HTML5 pushState/replaceState Router, Link, and location hooks
│   │   └── types/               # generated OpenAPI types and domain honesty models
│   ├── public/                  # Static assets (favicons, brand fonts, manifest)
│   ├── index.html               # SPA HTML entry point
│   ├── nginx.conf               # Hardened Nginx configuration for SPA routing fallback and security headers
│   ├── playwright.config.ts     # Playwright E2E configuration
│   ├── vite.config.ts           # Vite bundler and Vitest test configuration
│   ├── tailwind.config.js       # Tailwind 0px geometry and theme tokens
│   ├── tsconfig.json            # TypeScript project reference root
│   └── package.json             # Frontend dependencies and scripts
│
├── Taskfile.yml                 # task runner automation configuration
├── Dockerfile                   # multi-stage unified container build (Node web-builder, Go static compiler with embedded SPA, Distroless runtime)
├── Dockerfile.web               # multi-stage Vite SPA + Nginx production container build (standalone option)
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
- **Transport Packages**: `internal/transport/rest` and `internal/transport/mcp` handle protocol encoding, parameter extraction, and input validation before delegating to `service`. `internal/transport/spa` delivers embedded frontend SPA assets, manages HTML5 history fallback routing, and enforces strict API route guards.
- **Infrastructure Observability**: Infrastructure packages (`internal/store`, `internal/cache`, `internal/dlq`, and `internal/storage`) maintain native OpenTelemetry tracing and metrics instrumentation. Database queries use `otelpgx` spans and export pool gauges. Redis commands use `redisotel` spans and latency metrics. The DLQ client traces `dlq.record`, `dlq.clear`, and `dlq.get` with operation counters. Raw storage traces operations and records operation latency histograms.

