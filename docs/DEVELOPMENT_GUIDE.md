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
# CLOUDVITTA_CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:3000,https://cloudvitta.thatengineerguy.in
# CLOUDVITTA_CORS_ALLOW_CREDENTIALS=true

# --- Data Freshness (Optional) ---
# Staleness threshold in hours (default: 168 hours = 7 days)
# CLOUDVITTA_FRESHNESS_STALENESS_THRESHOLD_HOURS=168

# --- Observability (OpenTelemetry / Grafana Cloud) ---
CLOUDVITTA_OBSERVABILITY_OTLP_ENDPOINT=https://otlp-gateway-prod-us-central-0.grafana.net/otlp
CLOUDVITTA_OBSERVABILITY_OTLP_HEADERS=Authorization=Basic MTIzNDU2OmdsY19leUouLi4=

# --- Transactional Email (Resend) ---
# Resend API key (falls back to in-memory NoopSender if empty)
CLOUDVITTA_EMAIL_RESEND_API_KEY=
# Sender email address with verified domain
CLOUDVITTA_EMAIL_FROM=CloudVitta <noreply@cloudvitta.thatengineerguy.in>
# Base URL for verification links sent in emails
CLOUDVITTA_EMAIL_VERIFY_EMAIL_BASE_URL=http://localhost:5173/verify-email
```

---

## 3. Provisioning External Services & Configuring Secrets

### Service A: Neon Serverless Postgres (Database)
1. **Create Project**: Log in to the [Neon Console](https://console.neon.tech/) and create a new project (e.g. `cloudvitta-dev`). Select PostgreSQL version `16+` and your target region (e.g. `us-east-1` or `us-east-2`).
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
1. **Create Database**: Log in to the [Upstash Console](https://console.upstash.io/) and create a Redis database (e.g. `cloudvitta-cache-dev`). Select a primary region close to your database.
2. **Copy TLS URL**: Under database details, locate **Node client / Redis URL**. Select the TLS connection string starting with `rediss://` (e.g. `rediss://default:password@xxx.upstash.io:6379`).
3. **Configure Key**: Set `CLOUDVITTA_REDIS_URL` in `.env`.
   *Note: If `CLOUDVITTA_REDIS_URL` is unavailable or empty, the API server logs a warning and automatically falls back to direct Postgres reads.*

---

### Service C: Google Cloud Storage (Raw Payloads & Quarantine Sinks)
1. **Create Bucket**: Log in to the [Google Cloud Console](https://console.cloud.google.com/) and create a Cloud Storage bucket (e.g. `cloudvitta-raw-dev`).
2. **Configure Key**: Set `CLOUDVITTA_STORAGE_GCS_BUCKET_NAME=cloudvitta-raw-dev` in `.env`.
3. **Authenticate Local Environment (Optional for local dev)**: Authenticate your local machine using Application Default Credentials:
   ```bash
   gcloud auth application-default login
   ```
   The Go Google Cloud SDK uses these local credentials automatically when executing `cmd/ingest`.
   *Note: If GCS credentials are not found in the `development` environment, the ingestion runner logs a warning and automatically falls back to in-memory raw storage.*

---

### Service D: Grafana Cloud (OpenTelemetry Traces, Metrics, and Logs)
1. **Locate OTLP Details**: Log in to the [Grafana Cloud Console](https://grafana.com/) and navigate to Stack Details -> **OpenTelemetry** -> **Configure**.
2. **Extract OTLP Endpoint**: Copy your OTLP HTTP Endpoint (e.g. `https://otlp-gateway-prod-us-central-0.grafana.net/otlp`) and set `CLOUDVITTA_OBSERVABILITY_OTLP_ENDPOINT`.
3. **Generate Authorization Header**:
   - Create an Access Token / API Key with publisher permissions in Grafana Cloud.
   - Note your Grafana Instance ID (numeric ID on the OTel details page).
   - Format the string: `Instance_ID:API_Token` (e.g. `123456:glc_eyJ...`).
   - Base64-encode the string using your terminal:
     ```bash
     echo -n "123456:glc_your_token_here" | base64
     ```
   - Set `CLOUDVITTA_OBSERVABILITY_OTLP_HEADERS="Authorization=Basic <BASE64_STRING>"`.
   *Note: If `CLOUDVITTA_OBSERVABILITY_OTLP_ENDPOINT` is empty, OpenTelemetry falls back to local Prometheus metrics (`/metrics`) and stdout JSON logging without attempting remote exports.*

---

### Service E: Authentication & Security Secrets

Generate high-entropy cryptographic keys for JWT signing and session tracking cookies:

1. **Generate JWT Access Token Secret (`CLOUDVITTA_AUTH_JWT_SECRET`)**:
   - The JWT signing secret must be at least 32 characters long.
   - Generate a random 256-bit base64-encoded string:
     ```bash
     openssl rand -base64 32
     ```
   - Copy the generated value to `CLOUDVITTA_AUTH_JWT_SECRET` in `.env`.

2. **Generate Anonymous Cookie Secret (`CLOUDVITTA_AUTH_ANON_COOKIE_SECRET`)**:
   - The anonymous cookie secret is used to HMAC-sign `cv_anon_id` tracking cookies for the Free Tier rate limiter.
   - Generate a distinct random 256-bit string:
     ```bash
     openssl rand -base64 32
     ```
   - Copy the value to `CLOUDVITTA_AUTH_ANON_COOKIE_SECRET` in `.env` (if omitted, it falls back to `CLOUDVITTA_AUTH_JWT_SECRET`).

---

### Service F: Cloud Provider API Keys & Credentials

CloudVitta ingests pricing catalogs from 7 cloud providers and exchange rates from Frankfurter:

1. **Google Cloud Platform (`CLOUDVITTA_GCP_API_KEY`)**:
   - *Purpose*: Optional API key to increase Google Cloud Billing Catalog API rate quotas during ingestion.
   - *Steps*:
     1. Open [Google Cloud Console](https://console.cloud.google.com/).
     2. Navigate to **APIs & Services** > **Credentials**.
     3. Click **Create Credentials** > **API key**.
     4. (Optional) Restrict key to **Cloud Billing API**.
     5. Set `CLOUDVITTA_GCP_API_KEY=<your_api_key>` in `.env`.

2. **IBM Cloud (`CLOUDVITTA_IBM_API_KEY`)**:
   - *Purpose*: IAM API key required to authenticate with the IBM Cloud Global Catalog API.
   - *Steps*:
     1. Log in to [IBM Cloud Console](https://cloud.ibm.com/).
     2. Navigate to **Manage** > **Access (IAM)** > **API keys**.
     3. Click **Create an IBM Cloud API key**, enter a name (e.g. `cloudvitta-ingest`), and click **Create**.
     4. Copy the generated key and set `CLOUDVITTA_IBM_API_KEY=<your_api_key>` in `.env`.

3. **Alibaba Cloud (`CLOUDVITTA_ALIBABA_ACCESS_KEY_ID` & `CLOUDVITTA_ALIBABA_ACCESS_KEY_SECRET`)**:
   - *Purpose*: AccessKey credentials required for HMAC-SHA1 signature computation on Alibaba Cloud ECS DescribePrice RPC requests.
   - *Steps*:
     1. Log in to [Alibaba Cloud Console](https://usercenter.console.aliyun.com/).
     2. Hover over your avatar and select **AccessKey Management**.
     3. Click **Create AccessKey** (use a RAM user with read-only ECS permissions for production).
     4. Set `CLOUDVITTA_ALIBABA_ACCESS_KEY_ID=<AccessKeyId>` and `CLOUDVITTA_ALIBABA_ACCESS_KEY_SECRET=<AccessKeySecret>` in `.env`.

4. **DigitalOcean (`CLOUDVITTA_DIGITALOCEAN_TOKEN`)**:
   - *Purpose*: Personal Access Token required to query the DigitalOcean Droplet Sizes REST API.
   - *Steps*:
     1. Log in to [DigitalOcean Control Panel](https://cloud.digitalocean.com/).
     2. Navigate to **API** > **Tokens**.
     3. Click **Generate New Token**. Enter a token name (e.g. `cloudvitta-ingest`) and select the **Read** scope.
     4. Copy the secret token and set `CLOUDVITTA_DIGITALOCEAN_TOKEN=<your_token>` in `.env`.

5. **Public & Unmetered Endpoints (No Credentials Needed)**:
   - **AWS**: Public bulk pricing JSON price lists (EC2, S3, RDS, DynamoDB, EKS).
   - **Azure**: Public Azure Retail Prices REST API (Virtual Machines, Storage, Cosmos DB, AKS).
   - **Oracle OCI**: Public Oracle CE Tools Pricelist JSON.
   - **Frankfurter API**: Public European Central Bank (ECB) daily reference rates cached locally in the `fx_rates` database table.

---

### Service G: Rate Limiting, CORS, & Data Freshness Overrides (Optional)

You can customize runtime protection policies in `.env`:
- **`CLOUDVITTA_RATELIMIT_STANDARD_TIER_RATE`**: Max requests per minute for authenticated JWT users (default: `120`).
- **`CLOUDVITTA_RATELIMIT_FREE_TIER_RATE`**: Max requests per minute for anonymous sessions with tracking cookies (default: `20`).
- **`CLOUDVITTA_RATELIMIT_IP_CEILING_RATE`**: Outer IP burst ceiling per minute (default: `60`).
- **`CLOUDVITTA_RATELIMIT_LOGIN_RATE`**: Max login attempts per minute per IP to prevent brute-force attacks (default: `10`).
- **`CLOUDVITTA_CORS_ALLOWED_ORIGINS`**: Comma-separated list of allowed frontend origins (e.g. `http://localhost:5173,http://localhost:3000,https://cloudvitta.thatengineerguy.in`).
- **`CLOUDVITTA_CORS_ALLOW_CREDENTIALS`**: Set to `true` to allow authorization headers and cookies.
- **`CLOUDVITTA_FRESHNESS_STALENESS_THRESHOLD_HOURS`**: Hours before provider pricing observations are flagged as stale (default: `168` hours = 7 days).

---

### Service H: Transactional Email & Account Verification (Resend)

CloudVitta sends single-use email verification links during user registration:

1. **Obtain API Key (`CLOUDVITTA_EMAIL_RESEND_API_KEY`)**:
   - Log in to the [Resend Console](https://resend.com/api-keys) and create a new API key.
   - Set `CLOUDVITTA_EMAIL_RESEND_API_KEY=re_...` in `.env`.
   - *Fallback*: If omitted or empty, the application uses an in-memory `NoopSender` and prints outgoing verification tokens to structured logs without failing.

2. **Configure Sender Address (`CLOUDVITTA_EMAIL_FROM`)**:
   - Set your verified sender address (e.g. `CloudVitta <noreply@cloudvitta.thatengineerguy.in>`).
   - For initial testing without a custom domain, use `CloudVitta <onboarding@resend.dev>` (delivers only to your Resend registration address).

3. **Configure Verification Base URL (`CLOUDVITTA_EMAIL_VERIFY_EMAIL_BASE_URL`)**:
   - Set the destination route for generated verification links:
     - Local development: `http://localhost:5173/verify-email`
     - Production: `https://cloudvitta.thatengineerguy.in/verify-email`

---

## 4. Running Database Migrations

CloudVitta uses forward-only migrations managed via `tern` (`migrations/*.sql`):
- `0001_initial_schema.sql`: Core pricing observations, historical tracking, anomalies, and indexes.
- `0002_create_users_and_refresh_tokens.sql`: User accounts, hashed credentials, and refresh token families.
- `0003_create_fx_rates.sql`: Foreign exchange rates cache table and composite base/target lookup indexes.
- `0004_create_compute_instance_catalog.sql`: Dedicated compute hardware instance catalog table and lookup indexes (ADR 0041).
- `0005_add_email_verification.sql`: User email verification status flag and single-use verification token table (ADR 0043).

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
| `/docs/swagger.json` | `GET` | OpenAPI specification in JSON format |
| `/docs/swagger.yaml` | `GET` | OpenAPI specification in YAML format |
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

   **Provider Operational Status Vocabulary**:
   - `healthy`: All declared and supported categories have fresh ingested pricing data within the staleness threshold.
   - `partially_healthy`: All categories with ingested data are fresh and healthy, but one or more declared categories have zero observations (`count == 0`).
   - `degraded`: One or more categories with ingested data have exceeded the staleness threshold or encountered active Dead Letter Queue (DLQ) failures.
   - `stale`: All supported categories for the provider are stale or contain zero observations.
   - `blocked`: Ingestion for a category is paused due to consecutive ingestion failures reaching the unrecoverable error threshold.
   - `not_yet_ingested`: Ingestion pipelines for the provider are not yet active or scheduled.

5. **Query Model Context Protocol (MCP) Streamable HTTP Tools (10 Tools)**:
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

6. **Register, Verify Email, Authenticate, and Rotate Session**:
   ```bash
   # Register new user (account created with email_verified = false)
   curl -X POST "http://localhost:8080/api/v1/auth/signup" \
     -H "Content-Type: application/json" \
     -d '{"email":"user@example.com","password":"securePassword123"}'

   # Note: Logging in before verification returns 403 Forbidden:
   # {"type":"https://cloudvitta.dev/errors/email-not-verified","title":"Forbidden",...}

   # Verify email address using the token received in email or stdout log:
   curl -X POST "http://localhost:8080/api/v1/auth/verify-email" \
     -H "Content-Type: application/json" \
     -d '{"token":"<raw_token_from_email>"}'

   # (Optional) Resend verification email if expired or lost (always returns 202 Accepted):
   curl -X POST "http://localhost:8080/api/v1/auth/resend-verification" \
     -H "Content-Type: application/json" \
     -d '{"email":"user@example.com"}'

   # Authenticate and receive token pair (succeeds once verified)
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

---

## 9. Frontend SPA Development & Unified Monolith (Model B)

CloudVitta includes a React 19 single-page application (SPA) located in the `web/` directory.

### 1. Local Development with Zero-CORS Vite Proxy
Start the Go backend server on port 8080:
```bash
task dev:api
```
Start the local Vite development server with hot module replacement (HMR) on port 3000:
```bash
task web:dev
# or
cd web && npm run dev
```
The Vite development server is pre-configured to proxy `/api` and `/mcp` requests directly to `http://localhost:8080`, providing zero-CORS local development with hot reloading.

### 2. Frontend Unit and Component Tests (Vitest)
Execute the complete component and utility test suite:
```bash
task web:test
# or
cd web && npm test
```

### 3. End-to-End Smoke Tests (Playwright)
Execute end-to-end smoke tests against Chromium:
```bash
task web:test:e2e
# or
cd web && npm run test:e2e
```
*Optional*: Run tests with the interactive visual UI:
```bash
cd web && npm run test:e2e:ui
```

### 4. Production Build & Unified Containerization (Model B)
In production (Model B), the Go backend binary embeds the compiled `web/dist` frontend bundle via standard library `//go:embed all:dist` (in `internal/transport/spa`):

To build frontend assets and compile static Go binaries locally:
```bash
task build:full
```

To build the unified production Docker container:
```bash
task docker:build
# or
docker build -t cloudvitta:latest .
```

To run the unified container locally:
```bash
docker run -p 8080:8080 cloudvitta:latest
```
Navigate to `http://localhost:8080` to access the web UI, Swagger OpenAPI documentation (`/docs/`), Streamable HTTP MCP server (`/mcp`), health probes (`/healthz`, `/readyz`), Prometheus metrics (`/metrics`), and REST APIs (`/api/v1/*`) from a single origin.

### 5. Standalone Nginx Container (Optional)
The standalone Nginx container (`Dockerfile.web`) remains available as an optional standalone build:
```bash
task web:docker:build
```

