# CloudVitta Master Development & Setup Guide


## Overview
This guide provides step-by-step instructions to provision external services (Neon Postgres, Upstash Redis, Google Cloud Storage, Grafana Cloud, provider APIs), configure your local `.env` file, run database migrations, start application processes, and verify telemetry, pricing calculations, and Model Context Protocol (MCP) endpoints.

---

## 1. Prerequisites & Required Tools
Before starting, make sure the following tools are installed on your development system:
- **Go**: Version `1.26+` installed and available in `$PATH`.
- **Git**: Installed for version control.
- **Task Runner (`task`)**: Version `3+` for running project automation recipes.
- **Tern Migration Tool**: Install via Go:
  ```bash
  go install github.com/jackc/tern/v2@latest
  ```
- **Google Cloud SDK (`gcloud`)**: Installed for local GCS authentication and Cloud Run management.
- **golangci-lint**: Installed for static code analysis.

### Common Task Commands
- `task`: List all available project commands.
- `task build`: Build all project binaries (`api`, `ingest`, `migrate`, `quarantine-digest`).
- `task dev:api`: Start the API server locally.
- `task dev:ingest`: Run the cloud pricing ingestion orchestrator locally.
- `task dev:quarantine`: Run the quarantine digest processor locally.
- `task test`: Run all tests with the Go race detector enabled (`go test -race ./...`).
- `task test:fast`: Run all tests without the race detector (local fast iteration only).
- `task test:cover`: Run all tests with race detection and output a code coverage report.
- `task lint`: Run source code formatting checks and `go vet ./...`.
- `task fmt`: Auto-format all Go source files with `gofmt`.
- `task swagger`: Regenerate OpenAPI 2.0 Swagger specifications.
- `task sqlc`: Regenerate type-safe SQL store queries using SQLC.
- `task db:migrate`: Run database schema migrations forward using `tern`.
- `task clean`: Clean Go build cache and test artifacts.

---

## 2. Environment Configuration (`.env`)

CloudVitta loads environment variables prefixed with `CLOUDVITTA_`. Copy `.env.example` to `.env` in the repository root:

```bash
cp .env.example .env
```

Below is the complete reference of `.env` configuration keys and their default development values:

```env
# --- Server Configuration ---
CLOUDVITTA_SERVER_PORT=8080
CLOUDVITTA_PRIMARY_ENVIRONMENT=development
CLOUDVITTA_PRIMARY_LOG_LEVEL=debug

# --- Database (Neon Serverless Postgres) ---
# App Runtime: Use the POOLED host (contains "-pooler") for application runtime connections
CLOUDVITTA_DATABASE_HOST=ep-scratch-123456-pooler.us-east-2.aws.neon.tech
# Migrations: Use the DIRECT host (without "-pooler") for schema migrations via tern
CLOUDVITTA_DATABASE_MIGRATION_HOST=ep-scratch-123456.us-east-2.aws.neon.tech
CLOUDVITTA_DATABASE_PORT=5432
CLOUDVITTA_DATABASE_USER=neondb_owner
CLOUDVITTA_DATABASE_PASSWORD=your_neon_password
CLOUDVITTA_DATABASE_NAME=neondb
CLOUDVITTA_DATABASE_SSL_MODE=require

# Optional Database Pool Tuning Overrides
# CLOUDVITTA_DATABASE_MAX_OPEN_CONNS=5
# CLOUDVITTA_DATABASE_MAX_IDLE_CONNS=2
# CLOUDVITTA_DATABASE_CONN_MAX_LIFETIME=1800
# CLOUDVITTA_DATABASE_CONN_MAX_IDLE_TIME=300

# --- Cache & Singleflight (Upstash Redis) ---
CLOUDVITTA_REDIS_URL=rediss://default:your_upstash_password@your-redis-endpoint.upstash.io:6379

# --- Storage (Google Cloud Storage) ---
CLOUDVITTA_STORAGE_GCS_BUCKET_NAME=cloudvitta-raw-dev

# --- Authentication & Cookies ---
# Secret used for signing JWT access tokens (must be at least 32 characters long)
CLOUDVITTA_AUTH_JWT_SECRET=super-secret-jwt-key-at-least-32-chars-long
# Secret used for HMAC-signing anonymous session tracking cookies (cv_anon_id)
CLOUDVITTA_AUTH_ANON_COOKIE_SECRET=optional-anon-cookie-secret-key-here

# --- Cloud Provider API Keys & Credentials (Optional) ---
# Optional GCP Cloud Billing Catalog API Key for public catalog ingestion
CLOUDVITTA_GCP_API_KEY=
# Optional IBM Cloud IAM API Key for Global Catalog ingestion
CLOUDVITTA_IBM_API_KEY=
# Optional Alibaba Cloud AccessKeyId and AccessKeySecret for ECS RPC API ingestion
CLOUDVITTA_ALIBABA_ACCESS_KEY_ID=
CLOUDVITTA_ALIBABA_ACCESS_KEY_SECRET=
# Optional DigitalOcean Personal Access Token for Droplet Sizes API ingestion
CLOUDVITTA_DIGITALOCEAN_TOKEN=

# --- Rate Limiting & CORS Overrides (Optional) ---
# CLOUDVITTA_RATELIMIT_STANDARD_TIER_RATE=120
# CLOUDVITTA_RATELIMIT_FREE_TIER_RATE=20
# CLOUDVITTA_RATELIMIT_IP_CEILING_RATE=60
# CLOUDVITTA_RATELIMIT_LOGIN_RATE=10
# CLOUDVITTA_CORS_ALLOWED_ORIGINS=http://localhost:3000,https://cloudvitta.dev
# CLOUDVITTA_CORS_ALLOW_CREDENTIALS=true

# --- Data Freshness (Optional) ---
# Staleness threshold in hours (default: 168 hours = 7 days)
# CLOUDVITTA_FRESHNESS_STALENESS_THRESHOLD_HOURS=168

# --- Observability (OpenTelemetry / Grafana Cloud) ---
CLOUDVITTA_OBSERVABILITY_OTLP_ENDPOINT=https://otlp-gateway-prod-us-central-0.grafana.net/otlp
CLOUDVITTA_OBSERVABILITY_OTLP_HEADERS=Authorization=Basic MTIzNDU2OmdsY19leUouLi4=
```

---

## 3. Provisioning External Services

### Service A: Neon Serverless Postgres (Database)
1. **Create Project**: Log in to [Neon Console](https://console.neon.tech/) and create a new project (e.g. `cloudvitta-dev`). Select PostgreSQL version `16+` and your target region (e.g. `us-east-1` or `us-east-2`).
2. **Extract Credentials**: On the Neon dashboard, inspect **Connection Details**:
   - `CLOUDVITTA_DATABASE_USER`: e.g. `neondb_owner`.
   - `CLOUDVITTA_DATABASE_PASSWORD`: Your secret Neon user password.
   - `CLOUDVITTA_DATABASE_NAME`: e.g. `neondb` or `cloudvitta`.
   - `CLOUDVITTA_DATABASE_SSL_MODE`: `require`.
3. **Configure Hosts**:
   - Toggle **Pooled connection** in the Neon console. Copy the hostname containing `-pooler` (e.g. `ep-xyz-pooler.us-east-2.aws.neon.tech`) and set `CLOUDVITTA_DATABASE_HOST`.
   - Toggle **Direct connection** (or remove `-pooler` from the hostname). Copy the direct hostname (e.g. `ep-xyz.us-east-2.aws.neon.tech`) and set `CLOUDVITTA_DATABASE_MIGRATION_HOST`.
   > [!NOTE]
   > `tern` schema migrations require `CLOUDVITTA_DATABASE_MIGRATION_HOST` (direct endpoint) because transaction-level connection pooling prevents DDL migration locks.

---

### Service B: Upstash Redis (Cache, Singleflight, & DLQ)
1. **Create Database**: Log in to [Upstash Console](https://console.upstash.io/) and create a Redis database (e.g. `cloudvitta-cache-dev`). Select a primary region close to your database.
2. **Copy TLS URL**: Under database details, locate **Node client / Redis URL**. Select the TLS connection string starting with `rediss://` (e.g. `rediss://default:password@xxx.upstash.io:6379`).
3. **Configure Key**: Set `CLOUDVITTA_REDIS_URL` in `.env`.
   *Note: If `CLOUDVITTA_REDIS_URL` is unavailable or empty, the API server logs a warning and automatically falls back to direct Postgres reads.*

---

### Service C: Google Cloud Storage (Raw Payloads & Quarantine Sinks)
1. **Create Bucket**: Log in to [Google Cloud Console](https://console.cloud.google.com/) and create a Cloud Storage bucket (e.g. `cloudvitta-raw-dev`).
2. **Configure Key**: Set `CLOUDVITTA_STORAGE_GCS_BUCKET_NAME=cloudvitta-raw-dev` in `.env`.
3. **Authenticate Local Environment**: Authenticate your local machine using Application Default Credentials:
   ```bash
   gcloud auth application-default login
   ```
   The Go Google Cloud SDK uses these local credentials automatically when executing `cmd/ingest`.

---

### Service D: Grafana Cloud (OpenTelemetry Traces, Metrics, and Logs)
1. **Locate OTLP Details**: Log in to [Grafana Cloud Console](https://grafana.com/) and navigate to Stack Details -> **OpenTelemetry** -> **Configure**.
2. **Extract OTLP Endpoint**: Copy your OTLP HTTP Endpoint (e.g. `https://otlp-gateway-prod-us-central-0.grafana.net/otlp`) and set `CLOUDVITTA_OBSERVABILITY_OTLP_ENDPOINT`.
3. **Generate Authorization Header**:
   - Create an Access Token / API Key with publisher permissions in Grafana Cloud.
   - Note your Grafana Instance ID (numeric ID on the OTel details page).
   - Format the string: `Instance_ID:API_Token` (e.g. `123456:glc_eyJ...`).
   - Base64-encode the string (e.g. `MTIzNDU2OmdsY19leUouLi4=`).
   - Set `CLOUDVITTA_OBSERVABILITY_OTLP_HEADERS="Authorization=Basic MTIzNDU2OmdsY19leUouLi4="`.
   *Note: If `CLOUDVITTA_OBSERVABILITY_OTLP_ENDPOINT` is empty, OpenTelemetry falls back to local Prometheus metrics (`/metrics`) and stdout JSON logging without attempting remote exports.*

---

### Service E: External Cloud Provider APIs & Foreign Exchange (FX) Reference
CloudVitta pulls real pricing data from 7 cloud providers and real exchange rates from Frankfurter:
- **AWS**: Public Bulk Pricing JSON endpoints (unauthenticated).
- **Azure**: Public Azure Retail Prices API (unauthenticated).
- **GCP**: Google Cloud Billing Catalog API (requires optional `CLOUDVITTA_GCP_API_KEY` for high-volume requests).
- **Oracle OCI**: Oracle CE Tools Public Pricelist JSON (unauthenticated).
- **IBM Cloud**: IBM Cloud Global Catalog API and IAM token exchange (requires optional `CLOUDVITTA_IBM_API_KEY`).
- **Alibaba Cloud**: Alibaba ECS DescribePrice RPC API with HMAC-SHA1 signature (requires optional `CLOUDVITTA_ALIBABA_ACCESS_KEY_ID` and `CLOUDVITTA_ALIBABA_ACCESS_KEY_SECRET`).
- **DigitalOcean**: DigitalOcean Droplets Sizes REST API (requires optional `CLOUDVITTA_DIGITALOCEAN_TOKEN`).
- **Frankfurter API**: European Central Bank (ECB) daily reference rates (unauthenticated public API, backed by local `fx_rates` database table).

---

## 4. Running Database Migrations

CloudVitta uses forward-only migrations managed via `tern` (`migrations/*.sql`):
- `0001_initial_schema.sql`: Core pricing observations, historical tracking, anomalies, and indexes.
- `0002_create_users_and_refresh_tokens.sql`: User accounts, hashed credentials, and refresh token families.
- `0003_create_fx_rates.sql`: Foreign exchange rates cache table and composite base/target lookup indexes.

Run migrations using the built-in runner:

```bash
go run ./cmd/migrate
```

### Alternative: Running `tern` directly
```bash
go run github.com/jackc/tern/v2@latest migrate --connstring "postgresql://$CLOUDVITTA_DATABASE_USER:$CLOUDVITTA_DATABASE_PASSWORD@$CLOUDVITTA_DATABASE_MIGRATION_HOST:$CLOUDVITTA_DATABASE_PORT/$CLOUDVITTA_DATABASE_NAME?sslmode=$CLOUDVITTA_DATABASE_SSL_MODE" -m migrations
```

---

## 5. Running Application Processes

### Running the API Server
Start the HTTP REST API server:
```bash
go run ./cmd/api
# or using task
task dev:api
```
On startup, the API server:
1. Initializes OpenTelemetry telemetry (traces, metrics, structured logs).
2. Connects to the Neon Postgres database pool and Upstash Redis.
3. Loads foreign exchange rates via `FXService` with database fallback.
4. Initializes Freshness, Pricing, and Authentication services.
5. Mounts REST endpoints (`/api/v1/*`) and Model Context Protocol transport (`/mcp`).
6. Starts listening on port `8080` (or `CLOUDVITTA_SERVER_PORT`).

### Running Data Ingestion
Execute the provider pricing ingestion orchestrator:
```bash
go run ./cmd/ingest
# or using task
task dev:ingest
```
The ingestion orchestrator:
- Synchronizes daily foreign exchange rates into `fx_rates`.
- Concurrently fetches pricing for all 7 registered providers across active categories.
- Enforces per-provider rate limits and exponential backoff retries.
- Acquires Redis distributed locks to prevent overlapping ingestion runs.
- Persists raw JSON payloads to Google Cloud Storage before normalization.
- Normalizes pricing data and writes observations to Postgres with anomaly checks.
- Isolates unmapped product SKUs and regions to the quarantine sink.
- Records unrecoverable provider failures into the Redis Dead Letter Queue (DLQ).

### Running Quarantine Digest Processor
Inspect and process unmapped catalog items isolated during ingestion:
```bash
go run ./cmd/quarantine-digest
# or using task
task dev:quarantine
```

---

## 6. Endpoints & Telemetry Verification

### System & Telemetry Endpoints

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/healthz` | `GET` | Process liveness probe (`200 OK`) |
| `/readyz` | `GET` | Readiness probe (verifies Postgres pool & Redis reachability) |
| `/metrics` | `GET` | Prometheus metrics endpoint (includes `cache_requests_total`) |
| `/docs/` | `GET` | Interactive Swagger UI OpenAPI documentation |
| `/api/v1/prices/compute` | `GET` | Compute pricing lookup & SKU comparison (supports `?currency=...`) |
| `/api/v1/prices/storage` | `GET` | Storage pricing lookup & comparison (supports `?currency=...`) |
| `/api/v1/prices/network` | `GET` | Network egress pricing lookup & comparison (supports `?currency=...`) |
| `/api/v1/prices/database` | `GET` | Relational database (RDBMS) pricing comparison (supports `?currency=...`) |
| `/api/v1/prices/database-nosql` | `GET` | NoSQL database pricing comparison (supports `?currency=...`) |
| `/api/v1/prices/kubernetes` | `GET` | Managed Kubernetes control-plane fee comparison (supports `?currency=...`) |
| `/api/v1/prices/serverless` | `GET` | Serverless compute (FaaS) pricing comparison (supports `?currency=...`) |
| `/api/v1/calculate` | `POST` | Composite multi-category workload total across 7 categories (supports `?currency=...`) |
| `/api/v1/providers/{provider}/status` | `GET` | Provider operational status and data age across 7 providers |
| `/api/v1/auth/signup` | `POST` | User registration endpoint (returns user profile) |
| `/api/v1/auth/login` | `POST` | User authentication endpoint (returns JWT access and refresh tokens) |
| `/api/v1/auth/refresh` | `POST` | Token rotation endpoint (requires `idempotency_key`, returns new token pair) |
| `/api/v1/auth/logout` | `POST` | Session revocation endpoint (revokes token family and invalidates cache) |
| `/mcp` | `POST` | Model Context Protocol (MCP) Streamable HTTP tools endpoint (9 tools) |

### Local Verification Commands

1. **Verify Liveness and Readiness**:
   ```bash
   curl http://localhost:8080/healthz
   curl http://localhost:8080/readyz
   ```

2. **Query Single-Category Pricing Endpoints (7 Categories with Multi-Currency)**:
   ```bash
   # 1. Compute (USD and EUR conversion)
   curl "http://localhost:8080/api/v1/prices/compute?vcpu=4&ram_gb=16&region=us-east"
   curl "http://localhost:8080/api/v1/prices/compute?vcpu=4&ram_gb=16&region=us-east&currency=EUR"
   
   # 2. Storage
   curl "http://localhost:8080/api/v1/prices/storage?size_gb=500&storage_class=standard&region=us-east"
   
   # 3. Network Egress
   curl "http://localhost:8080/api/v1/prices/network?egress_gb=1000&region=us-east"
   
   # 4. Relational Database (RDBMS)
   curl "http://localhost:8080/api/v1/prices/database?engine=postgresql&vcpu=4&ram_gb=16&storage_gb=100&region=us-east"
   
   # 5. NoSQL Database
   curl "http://localhost:8080/api/v1/prices/database-nosql?data_model=document&pricing_mode=provisioned&read_units=1000&write_units=500&storage_gb=200&region=us-east"
   
   # 6. Managed Kubernetes Control-Plane
   curl "http://localhost:8080/api/v1/prices/kubernetes?tier=standard&cluster_topology=zonal&region=us-east"
   
   # 7. Serverless Compute (FaaS)
   curl "http://localhost:8080/api/v1/prices/serverless?tier=standard&architecture=x86_64&requests_per_month=5000000&execution_duration_ms=200&memory_mb=512&region=us-east"
   ```

3. **Query Composite Calculation Endpoint (7 Categories with Multi-Currency)**:
   ```bash
   curl -X POST "http://localhost:8080/api/v1/calculate?currency=EUR" \
     -H "Content-Type: application/json" \
     -d '{
       "region": "us-east",
       "compute": {"vcpu": 4, "ram_gb": 16},
       "storage": {"size_gb": 500},
       "network": {"egress_gb": 100},
       "database": {"engine": "postgresql", "vcpu": 4, "ram_gb": 16, "storage_gb": 100},
       "database_nosql": {"data_model": "document", "pricing_mode": "provisioned", "read_units": 1000, "write_units": 500, "storage_gb": 200},
       "kubernetes": {"tier": "standard", "cluster_topology": "zonal"},
       "serverless": {"tier": "standard", "architecture": "x86_64", "requests_per_month": 5000000, "execution_duration_ms": 200, "memory_mb": 512}
     }'
   ```

4. **Query Provider Operational Status Across All 7 Providers**:
   ```bash
   # Query status for AWS, Azure, GCP, Oracle, IBM, Alibaba, DigitalOcean
   curl http://localhost:8080/api/v1/providers/aws/status
   curl http://localhost:8080/api/v1/providers/azure/status
   curl http://localhost:8080/api/v1/providers/gcp/status
   curl http://localhost:8080/api/v1/providers/oracle/status
   curl http://localhost:8080/api/v1/providers/ibm/status
   curl http://localhost:8080/api/v1/providers/alibaba/status
   curl http://localhost:8080/api/v1/providers/digitalocean/status
   ```

5. **Query Model Context Protocol (MCP) Streamable HTTP Tools (9 Tools)**:
   ```bash
   # List available tools (requires Bearer JWT)
   curl -X POST "http://localhost:8080/mcp" \
     -H "Authorization: Bearer <access_token>" \
     -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

   # Call compare_compute tool
   curl -X POST "http://localhost:8080/mcp" \
     -H "Authorization: Bearer <access_token>" \
     -H "Content-Type: application/json" \
     -d '{
       "jsonrpc": "2.0",
       "id": 2,
       "method": "tools/call",
       "params": {
         "name": "compare_compute",
         "arguments": {
           "vcpu": 4,
           "ram_gb": 16,
           "region": "us-east",
           "currency": "EUR"
         }
       }
     }'
   ```

6. **Register, Authenticate, and Rotate Session**:
   ```bash
   # Register new user
   curl -X POST "http://localhost:8080/api/v1/auth/signup" \
     -H "Content-Type: application/json" \
     -d '{"email":"user@example.com","password":"securePassword123"}'

   # Authenticate and receive token pair
   curl -X POST "http://localhost:8080/api/v1/auth/login" \
     -H "Content-Type: application/json" \
     -d '{"email":"user@example.com","password":"securePassword123"}'

   # Rotate refresh token (requires idempotency_key)
   curl -X POST "http://localhost:8080/api/v1/auth/refresh" \
     -H "Content-Type: application/json" \
     -d '{"refresh_token":"<raw_refresh_token>","idempotency_key":"b57422f1-6789-4a0b-93f4-2f22c544e311"}'

   # Logout and revoke token family
   curl -X POST "http://localhost:8080/api/v1/auth/logout" \
     -H "Content-Type: application/json" \
     -d '{"refresh_token":"<raw_refresh_token>"}'
   ```

7. **Verify Rate Limiter**:
   Execute >10 requests to `/api/v1/auth/login` or >60 unauthenticated requests to generic comparison endpoints within 1 minute to receive `429 Too Many Requests` with a `Retry-After` header.

8. **Inspect Local Prometheus Metrics**:
   ```bash
   curl http://localhost:8080/metrics | grep cache_requests_total
   ```

---

## 7. Testing & Quality Checks

Run full static analysis and unit/integration test suites before committing code:

```bash
# Run all tests with Go race detector
go test -race ./...

# Run static analysis and linters
go vet ./...
golangci-lint run
```

---

## 8. Exploring Observability Data in Grafana Cloud

CloudVitta exports OpenTelemetry traces, metrics, and logs to Grafana Cloud.

### 1. Traces (Tempo)
Distributed traces display request latency and break down internal operations (Cache Read vs. Postgres Query vs. Singleflight).
* **How to view**:
  1. Open the **Explore** tab in Grafana.
  2. Select your traces data source (e.g. `grafanacloud-<name>-traces`).
  3. Filter by Service Name: `cloudvitta-api` or `cloudvitta-ingest`.
  4. Click **Run Query** and inspect the waterfall visualization.

### 2. Logs (Loki)
Structured JSON logs include correlated `trace_id` fields for seamless debugging.
* **How to view**:
  1. Open the **Explore** tab in Grafana.
  2. Select your logs data source (e.g. `grafanacloud-<name>-logs`).
  3. In the query builder, use label filters: `{service_name="cloudvitta-api"}`.
  4. Click **Run Query** to view structured log events.

### 3. Application Metrics (Prometheus)
Metrics track HTTP request rates, error rates, latencies, and custom application metrics (e.g. `cache_requests_total`).
* **How to view Application Dashboards**:
  1. Navigate to **Observability > Application > Services** in the Grafana sidebar.
  2. Select `cloudvitta-api` to inspect auto-generated RED (Rate, Errors, Duration) dashboards.
* **How to query Custom Metrics**:
  1. Open the **Explore** tab.
  2. Select your Prometheus data source.
  3. Query `cache_requests_total` to inspect cache hit and miss counts.

