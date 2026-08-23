# CloudVitta Master Development & Setup Guide


## Overview
This guide provides step-by-step instructions to provision required external services (Neon Postgres, Upstash Redis, Google Cloud Storage, Grafana Cloud), configure your local `.env` environment, run database migrations, start application processes, and verify telemetry and endpoints.

---

## 1. Prerequisites & Required Tools
Before starting, ensure the following tools are installed on your development machine:
- **Go**: Version `1.26+` installed and available in `$PATH`.
- **Git**: Installed for version control.
- **Task Runner (`task`)**: Version `3+` for running project commands.
- **Tern Migration Tool**: Install via Go:
  ```bash
  go install github.com/jackc/tern/v2@latest
  ```
- **Google Cloud SDK (`gcloud`)**: Installed for local GCS authentication.
- **golangci-lint**: Installed for static code analysis.

### Common Task Commands
- `task`: List all available project commands.
- `task build`: Build all binaries (`api`, `ingest`, `migrate`, `quarantine-digest`).
- `task dev:api`: Start API server locally.
- `task dev:ingest`: Run pricing ingestion worker locally.
- `task dev:quarantine`: Run quarantine digest processor locally.
- `task test`: Run all tests with race detector enabled (`go test -race ./...`).
- `task test:fast`: Run all tests without race detector (local iteration only, never gate CI or merges).
- `task test:cover`: Run all tests with race detector and generate coverage report.
- `task lint`: Run formatting check and `go vet`.
- `task fmt`: Auto-format all Go source files.
- `task swagger`: Regenerate OpenAPI Swagger documentation.
- `task sqlc`: Regenerate type-safe SQL store queries via SQLC.
- `task db:migrate`: Run database migrations forward.
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

# --- Cache (Upstash Redis) ---
CLOUDVITTA_REDIS_URL=rediss://default:your_upstash_password@your-redis-endpoint.upstash.io:6379

# --- Storage (Google Cloud Storage) ---
CLOUDVITTA_STORAGE_GCS_BUCKET_NAME=cloudvitta-raw-dev

# --- Observability (OpenTelemetry / Grafana Cloud) ---
CLOUDVITTA_OBSERVABILITY_OTLP_ENDPOINT=https://otlp-gateway-prod-us-central-0.grafana.net/otlp
CLOUDVITTA_OBSERVABILITY_OTLP_HEADERS=Authorization=Basic MTIzNDU2OmdsY19leUouLi4=
```

---

## 3. Provisioning External Services

### Service A: Neon Serverless Postgres (Database)
1. **Create Project**: Log in to [Neon Console](https://console.neon.tech/) and create a new project (e.g. `cloudvitta-dev`). Select PostgreSQL version `16+` and region `us-east-1` or `us-east-2`.
2. **Extract Credentials**: On the Neon dashboard, inspect **Connection Details**:
   - `CLOUDVITTA_DATABASE_USER`: e.g. `neondb_owner`.
   - `CLOUDVITTA_DATABASE_PASSWORD`: Your secret Neon user password.
   - `CLOUDVITTA_DATABASE_NAME`: e.g. `neondb` or `cloudvitta`.
   - `CLOUDVITTA_DATABASE_SSL_MODE`: `require`.
3. **Configure Hosts**:
   - Toggle **Pooled connection** in Neon console. Copy the hostname containing `-pooler` (e.g. `ep-xyz-pooler.us-east-2.aws.neon.tech`) and set `CLOUDVITTA_DATABASE_HOST`.
   - Toggle **Direct connection** (or remove `-pooler` from hostname). Copy the direct hostname (e.g. `ep-xyz.us-east-2.aws.neon.tech`) and set `CLOUDVITTA_DATABASE_MIGRATION_HOST`.
   > [!NOTE]
   > `tern` schema migrations require `CLOUDVITTA_DATABASE_MIGRATION_HOST` (direct endpoint) because transaction-level connection pooling breaks DDL migration locks.

---

### Service B: Upstash Redis (Cache & Singleflight)
1. **Create Database**: Log in to [Upstash Console](https://console.upstash.io/) and create a Redis database (e.g. `cloudvitta-cache-dev`). Select primary region matching your database (`us-east-1` or `us-east-2`).
2. **Copy TLS URL**: Under the database details page, locate **Node client / Redis URL**. Select the TLS connection string starting with `rediss://` (e.g. `rediss://default:password@xxx.upstash.io:6379`).
3. **Configure Key**: Set `CLOUDVITTA_REDIS_URL` in `.env`.
   *Note: If `CLOUDVITTA_REDIS_URL` is unavailable or invalid, the API server will log a warning and automatically fall back to Postgres-only reads.*

---

### Service C: Google Cloud Storage (GCS Raw Payload Storage)
1. **Create Bucket**: Log in to [Google Cloud Console](https://console.cloud.google.com/) and create a Cloud Storage bucket (e.g., `cloudvitta-raw-dev`).
2. **Configure Key**: Set `CLOUDVITTA_STORAGE_GCS_BUCKET_NAME=cloudvitta-raw-dev` in `.env`.
3. **Authenticate Local Environment**: Authenticate your local development machine to GCP using Application Default Credentials:
   ```bash
   gcloud auth application-default login
   ```
   This generates local credentials used automatically by the Go GCP SDK when running `cmd/ingest`.

---

### Service D: Grafana Cloud (OpenTelemetry Traces, Metrics, and Logs)
1. **Locate OTLP Details**: Log in to [Grafana Cloud Console](https://grafana.com/) and navigate to your Stack Details -> **OpenTelemetry** -> **Configure**.
2. **Extract OTLP Endpoint**: Copy your OTLP HTTP Endpoint (e.g., `https://otlp-gateway-prod-us-central-0.grafana.net/otlp`) and set `CLOUDVITTA_OBSERVABILITY_OTLP_ENDPOINT`.
3. **Generate Authorization Header**:
   - Create an Access Token / API Key with publisher permissions in Grafana Cloud.
   - Note your Grafana Instance ID (numeric ID on the OTel details page).
   - Format the string: `Instance_ID:API_Token` (e.g. `123456:glc_eyJ...`).
   - Base64-encode `Instance_ID:API_Token` string (e.g., `MTIzNDU2OmdsY19leUouLi4=`).
   - Set `CLOUDVITTA_OBSERVABILITY_OTLP_HEADERS="Authorization=Basic MTIzNDU2OmdsY19leUouLi4="`.
   *Note: If `CLOUDVITTA_OBSERVABILITY_OTLP_ENDPOINT` is left empty, OpenTelemetry gracefully degrades to local Prometheus metrics (`/metrics`) and stdout JSON logging without attempting remote exports.*

---

## 4. Running Database Migrations

You can run migrations directly using the built-in migration runner, which automatically reads credentials from your `.env` file:

```bash
go run ./cmd/migrate
```

### Alternative: Using `tern` CLI
If you prefer running `tern` directly, you can pass your connection parameters:
```bash
# Using Go to run tern directly from .env values
go run github.com/jackc/tern/v2@latest migrate --connstring "postgresql://$CLOUDVITTA_DATABASE_USER:$CLOUDVITTA_DATABASE_PASSWORD@$CLOUDVITTA_DATABASE_MIGRATION_HOST:$CLOUDVITTA_DATABASE_PORT/$CLOUDVITTA_DATABASE_NAME?sslmode=$CLOUDVITTA_DATABASE_SSL_MODE" -m migrations
```

---

## 5. Running Application Processes

### Running the API Server
Start the HTTP REST API server:
```bash
go run ./cmd/api
```
The server will initialize OpenTelemetry, connect to Neon Postgres and Upstash Redis, and start listening on port `8080` (or `CLOUDVITTA_SERVER_PORT`).

### Running Data Ingestion
Execute the provider data ingestion pipeline:
```bash
go run ./cmd/ingest
```
The orchestrator concurrently fetches pricing data from all registered provider/category pairs (e.g. AWS EC2). Each job uses per-provider rate limiting and retry policies, acquires a Redis idempotency lock to prevent overlapping runs, uploads raw JSON to GCS, inserts normalized pricing rows into `price_observations` with anomaly detection (flagging >10x price swings as `pending_review`), and records permanently failed jobs to the Redis DLQ.

---

## 6. Endpoints & Telemetry Verification

### System & Telemetry Endpoints

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/healthz` | `GET` | Process liveness probe (`200 OK`) |
| `/readyz` | `GET` | Readiness probe (verifies Postgres pool & Redis reachability) |
| `/metrics` | `GET` | Prometheus metrics endpoint (includes `cache_requests_total`) |
| `/docs/` | `GET` | Interactive Swagger UI OpenAPI documentation |
| `/api/v1/prices/compute` | `GET` | Compute pricing lookup & SKU comparison endpoint |
| `/api/v1/prices/storage` | `GET` | Storage pricing lookup & cost comparison endpoint |
| `/api/v1/prices/network` | `GET` | Network pricing lookup & egress cost comparison endpoint |
| `/api/v1/prices/database` | `GET` | Relational database (RDBMS) pricing comparison endpoint |
| `/api/v1/prices/database-nosql` | `GET` | NoSQL database pricing comparison endpoint |
| `/api/v1/calculate` | `POST` | Composite multi-category workload total, server-computed |
| `/api/v1/providers/{provider}/status` | `GET` | Provider operational status, category data age, and DLQ state |
| `/api/v1/auth/signup` | `POST` | User registration endpoint (returns user details) |
| `/api/v1/auth/login` | `POST` | User authentication endpoint (returns JWT access and refresh tokens) |
| `/api/v1/auth/refresh` | `POST` | Token rotation endpoint (requires `idempotency_key`, returns new token pair) |
| `/api/v1/auth/logout` | `POST` | Session revocation endpoint (revokes refresh token and clears cache) |

### Local Verification Commands

1. **Verify Liveness and Dependency Readiness**:
   ```bash
   curl http://localhost:8080/healthz
   curl http://localhost:8080/readyz
   ```

2. **Query Compute, Storage, and Network Pricing Endpoints**:
   ```bash
   curl "http://localhost:8080/api/v1/prices/compute?vcpu=4&ram_gb=16&region=us-east"
   curl "http://localhost:8080/api/v1/prices/storage?size_gb=500&storage_class=standard&region=us-east"
   curl "http://localhost:8080/api/v1/prices/network?egress_gb=1000&region=us-east"
   ```

3. **Query Composite Calculation Endpoint**:
   ```bash
   curl -X POST "http://localhost:8080/api/v1/calculate" \
     -H "Content-Type: application/json" \
     -d '{"region":"us-east","compute":{"vcpu":4,"ram_gb":16},"storage":{"size_gb":500},"network":{"egress_gb":100}}'
   ```

4. **Register and Authenticate User**:
   ```bash
   # Register new user
   curl -X POST "http://localhost:8080/api/v1/auth/signup" \
     -H "Content-Type: application/json" \
     -d '{"email":"user@example.com","password":"securePassword123"}'

   # Authenticate and receive tokens
   curl -X POST "http://localhost:8080/api/v1/auth/login" \
     -H "Content-Type: application/json" \
     -d '{"email":"user@example.com","password":"securePassword123"}'

   # Rotate refresh token (requires idempotency_key)
   curl -X POST "http://localhost:8080/api/v1/auth/refresh" \
     -H "Content-Type: application/json" \
     -d '{"refresh_token":"<raw_refresh_token>","idempotency_key":"b57422f1-6789-4a0b-93f4-2f22c544e311"}'

   # Logout / Revoke session
   curl -X POST "http://localhost:8080/api/v1/auth/logout" \
     -H "Content-Type: application/json" \
     -d '{"refresh_token":"<raw_refresh_token>"}'
   ```

5. **Verify Rate Limiter (60 req/min generic, 10 req/min login)**:
   Execute >10 requests to `/api/v1/auth/login` or >60 requests to generic endpoints within 1 minute to receive `429 Too Many Requests` with a `Retry-After` header.

6. **Inspect Local Prometheus Metrics**:
   ```bash
   curl http://localhost:8080/metrics | grep cache_requests_total
   ```

---

## 7. Testing & Quality Checks

Run full static analysis and unit/integration test suites before committing code:

```bash
# Run all tests with Go race detector
go test -race ./...

# Run static analysis & linters
go vet ./...
golangci-lint run
```

---

## 8. Exploring Observability Data in Grafana Cloud

CloudVitta exports standard OpenTelemetry traces, metrics, and logs to Grafana Cloud. Here is how to navigate the Grafana UI to understand your application data:

### 1. Traces (Tempo)
Distributed traces show you exactly how long a specific HTTP request took and break down the internal operations (e.g., Cache Read vs. Postgres Query).
* **How to view**:
  1. Open the **Explore** tab (compass icon) in Grafana.
  2. Select your traces data source (e.g., `grafanacloud-<name>-traces`).
  3. Use the "Search" tab, filter by Service Name: `cloudvitta-api`.
  4. Click **Run Query**. Click on any Trace ID to see the waterfall visualization of operations and their durations.

### 2. Logs (Loki)
Structured logs are forwarded automatically with `trace_id` attached, allowing you to correlate a log line directly to a slow request.
* **How to view**:
  1. Open the **Explore** tab.
  2. Select your logs data source (e.g., `grafanacloud-<name>-logs`).
  3. In the query builder, use the label filter: `{service_name="cloudvitta-api"}` or `{service_name="cloudvitta"}`.
  4. Click **Run Query** to see live, structured logs.

### 3. Application Metrics (Prometheus)
Metrics track aggregated health indicators like Request Rate (RPS), Error Rate, Request Duration, and custom metrics (e.g., `cache_requests_total`).
* **How to view Application Dashboards**:
  1. Go to **Observability > Application > Services** in the left sidebar.
  2. Select `cloudvitta-api`. Grafana automatically generates RED (Rate, Errors, Duration) dashboards based on OpenTelemetry HTTP metrics.
* **How to query Custom Metrics manually**:
  1. Open the **Explore** tab.
  2. Select your metrics data source (e.g., `grafanacloud-<name>-prom`).
  3. Run the query: `cache_requests_total` to see the breakdown of cache hits vs. misses.
