# CloudVitta Master Development & Setup Guide


## Overview
This guide provides step-by-step instructions to provision required external services (Neon Postgres, Upstash Redis, Google Cloud Storage, Grafana Cloud), configure your local `.env` environment, run database migrations, start application processes, and verify telemetry and endpoints.

---

## 1. Prerequisites & Required Tools
Before starting, ensure the following tools are installed on your development machine:
- **Go**: Version `1.26+` installed and available in `$PATH`.
- **Git**: Installed for version control.
- **Tern Migration Tool**: Install via Go:
  ```bash
  go install github.com/jackc/tern/v2@latest
  ```
- **Google Cloud SDK (`gcloud`)**: Installed for local GCS authentication.
- **golangci-lint**: Installed for static code analysis.

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
This fetches raw pricing payloads from provider APIs (e.g. AWS EC2), uploads raw JSON to your GCS bucket, and inserts normalized pricing rows into `price_observations`.

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

### Local Verification Commands

1. **Verify Liveness and Dependency Readiness**:
   ```bash
   curl http://localhost:8080/healthz
   curl http://localhost:8080/readyz
   ```

2. **Query Compute Pricing Endpoint**:
   ```bash
   curl "http://localhost:8080/api/v1/prices/compute?vcpu=4&ram_gb=16&region=us-east-1"
   ```

3. **Verify Rate Limiter (60 req/min)**:
   Execute >60 requests within 1 minute to receive `429 Too Many Requests` with a `Retry-After` header.

4. **Inspect Local Prometheus Metrics**:
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
