# CloudVitta DevOps & Deployment Guide

This guide explains how to set up the infrastructure and manage the deployment of CloudVitta to Google Cloud Run via GitHub Actions.

## Architecture Overview
- **CI (`ci.yml`)**: On every push and PR, code is linted, tested against an ephemeral Postgres container, and built to ensure it compiles.
- **CD (`deploy.yml`)**: On pushes to `main`, the code is containerized, pushed to Google Artifact Registry (GAR), database migrations are applied to the Neon production database, and the new container is deployed to Cloud Run.
- **Authentication**: GitHub Actions communicates with Google Cloud using **Workload Identity Federation** (WIF), eliminating the need for long-lived service account keys.

---

## Phase 1: Google Cloud Setup

You will need the `gcloud` CLI installed and authenticated.

### 1. Set up the Environment Variables
Run this in your terminal:
```bash
export PROJECT_ID="your-google-cloud-project-id"
export REGION="asia-southeast1"
export REPO_NAME="cloudvitta-repo"
export SERVICE_ACCOUNT_NAME="cloudvitta-deployer"
export GITHUB_REPO="yourusername/CloudVitta"
```

### 2. Enable Required APIs
```bash
gcloud services enable \
  artifactregistry.googleapis.com \
  run.googleapis.com \
  iamcredentials.googleapis.com \
  cloudresourcemanager.googleapis.com \
  --project="${PROJECT_ID}"
```

### 3. Create the Artifact Registry
```bash
gcloud artifacts repositories create ${REPO_NAME} \
  --repository-format=docker \
  --location=${REGION} \
  --description="CloudVitta Docker repository" \
  --project="${PROJECT_ID}"
```

---

## Phase 2: Workload Identity Federation (WIF) Setup

This configures a trust relationship between GitHub and your GCP project.

### 1. Create a Service Account
This is the account GitHub Actions will "impersonate".
```bash
gcloud iam service-accounts create ${SERVICE_ACCOUNT_NAME} \
  --display-name="CloudVitta Deployer" \
  --project="${PROJECT_ID}"

export SERVICE_ACCOUNT_EMAIL="${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
```

### 2. Grant Permissions to the Service Account
Give it permission to push to Artifact Registry and deploy to Cloud Run.
```bash
# Permission to push to Artifact Registry
gcloud artifacts repositories add-iam-policy-binding ${REPO_NAME} \
  --location=${REGION} \
  --member="serviceAccount:${SERVICE_ACCOUNT_EMAIL}" \
  --role="roles/artifactregistry.writer" \
  --project="${PROJECT_ID}"

# Permission to deploy to Cloud Run
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SERVICE_ACCOUNT_EMAIL}" \
  --role="roles/run.admin"

# Permission for the Cloud Run service to act as itself (Service Account User)
gcloud iam service-accounts add-iam-policy-binding \
  $(gcloud projects describe ${PROJECT_ID} --format="value(projectNumber)")-compute@developer.gserviceaccount.com \
  --member="serviceAccount:${SERVICE_ACCOUNT_EMAIL}" \
  --role="roles/iam.serviceAccountUser" \
  --project="${PROJECT_ID}"
```

### 3. Create the WIF Pool and Provider
```bash
# Create the Pool
gcloud iam workload-identity-pools create "github-pool" \
  --project="${PROJECT_ID}" \
  --location="global" \
  --display-name="GitHub Actions Pool"

export WORKLOAD_IDENTITY_POOL_ID=$(gcloud iam workload-identity-pools describe "github-pool" --project="${PROJECT_ID}" --location="global" --format="value(name)")

# Create the Provider
gcloud iam workload-identity-pools providers create-oidc "github-provider" \
  --project="${PROJECT_ID}" \
  --location="global" \
  --workload-identity-pool="github-pool" \
  --display-name="GitHub Actions Provider" \
  --attribute-mapping="google.subject=assertion.sub,attribute.actor=assertion.actor,attribute.repository=assertion.repository" \
  --issuer-uri="https://token.actions.githubusercontent.com"
```

### 4. Bind the Service Account to the GitHub Repository
This tells GCP: "Only allow GitHub Actions running in THIS specific repository to impersonate this service account."
```bash
gcloud iam service-accounts add-iam-policy-binding "${SERVICE_ACCOUNT_EMAIL}" \
  --project="${PROJECT_ID}" \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/${WORKLOAD_IDENTITY_POOL_ID}/attribute.repository/${GITHUB_REPO}"
```

### 5. Allow Public Invocations for Cloud Run (One-Time Setup)
To allow public users to call the CloudVitta API without authentication, grant the `Cloud Run Invoker` role to `allUsers`:
```bash
gcloud run services add-iam-policy-binding "${SERVICE_NAME:-cloudvitta-api}" \
  --region="${REGION}" \
  --member="allUsers" \
  --role="roles/run.invoker" \
  --project="${PROJECT_ID}"
```
*Note: The GitHub Actions deploy workflow also specifies `--allow-unauthenticated` on deployments.*

---

## Phase 3: GitHub Secrets and Variables Configuration

Navigate to your repository on GitHub:
**Settings** > **Secrets and variables** > **Actions** > **Secrets** tab > click **New repository secret**.

---

### 1. Google Cloud & Workload Identity Federation (WIF) Secrets

| Secret Name | Description & Acquisition Command |
| :--- | :--- |
| `GCP_PROJECT_ID` | Your Google Cloud project ID (e.g. `cloudvitta-prod`). |
| `GCP_SERVICE_ACCOUNT` | The service account email created in Phase 2 (e.g. `cloudvitta-deployer@your-project-id.iam.gserviceaccount.com`). |
| `GCP_WORKLOAD_IDENTITY_PROVIDER` | Full resource URI for the GitHub Actions WIF provider. Retrieve with:<br>`gcloud iam workload-identity-pools providers describe "github-provider" --project="${PROJECT_ID}" --location="global" --format="value(name)"` |

---

### 2. Database & Cache Infrastructure Secrets

| Secret Name | Description & Extraction Steps |
| :--- | :--- |
| `NEON_PROD_DSN` | Production PostgreSQL connection string with SSL required:<br>`postgres://user:password@ep-prod-db-1234.us-east-2.aws.neon.tech/neondb?sslmode=require`<br>*Note: Use the direct host (non-pooler) to allow Tern DDL migration locks.* |
| `REDIS_URL` | Upstash Redis TLS connection string:<br>`rediss://default:password@us1-prod-redis-1234.upstash.io:32451` |

---

### 3. Authentication & Security Secrets

Generate high-entropy keys on your local terminal before adding to GitHub Secrets:

1. **`CLOUDVITTA_AUTH_JWT_SECRET` (or `JWT_SECRET`)**:
   - Generate a 256-bit base64 secret (must be $\ge 32$ characters):
     ```bash
     openssl rand -base64 32
     ```
   - Paste the generated string into the secret value.

2. **`CLOUDVITTA_AUTH_ANON_COOKIE_SECRET`**:
   - Generate a distinct 256-bit base64 secret for HMAC signing `cv_anon_id` cookies:
     ```bash
     openssl rand -base64 32
     ```
   - Paste the generated string into the secret value.

---

### 4. Cloud Provider API Ingestion Secrets (Optional)

Configure API keys to enable scheduled ingestion runs across supported cloud providers:

1. **`CLOUDVITTA_GCP_API_KEY`**:
   - *Console*: [Google Cloud Console](https://console.cloud.google.com/) > **APIs & Services** > **Credentials** > **Create Credentials** > **API key**.
   - *Value*: The created API key string (e.g. `AIzaSy...`).

2. **`CLOUDVITTA_IBM_API_KEY`**:
   - *Console*: [IBM Cloud Console](https://cloud.ibm.com/) > **Manage** > **Access (IAM)** > **API keys** > **Create an IBM Cloud API key**.
   - *Value*: The generated IBM IAM API key.

3. **`CLOUDVITTA_ALIBABA_ACCESS_KEY_ID`**:
   - *Console*: [Alibaba Cloud Console](https://usercenter.console.aliyun.com/) > **AccessKey Management** > **Create AccessKey**.
   - *Value*: The generated `AccessKeyId` string.

4. **`CLOUDVITTA_ALIBABA_ACCESS_KEY_SECRET`**:
   - *Console*: [Alibaba Cloud Console](https://usercenter.console.aliyun.com/) > **AccessKey Management** > **Create AccessKey**.
   - *Value*: The matching `AccessKeySecret` string.

5. **`CLOUDVITTA_DIGITALOCEAN_TOKEN`**:
   - *Console*: [DigitalOcean Control Panel](https://cloud.digitalocean.com/) > **API** > **Tokens** > **Generate New Token** (select `Read` scope).
   - *Value*: The generated Personal Access Token (e.g. `dop_v1_...`).

---

### 5. Observability Secrets (Grafana Cloud)

| Secret Name | Description & Setup Steps |
| :--- | :--- |
| `GRAFANA_OTLP_ENDPOINT` | OTLP HTTP export endpoint from Grafana Cloud (e.g. `https://otlp-gateway-prod-us-central-0.grafana.net/otlp`). |
| `GRAFANA_OTLP_HEADERS` | Basic authentication header formatted as `Authorization=Basic <BASE64_STRING>`.<br>Generate base64 string using: `echo -n "InstanceID:APIToken" \| base64` |

---

### 6. Repository Variables (Optional Overrides)

Go to GitHub -> **Settings** -> **Secrets and variables** -> **Actions** -> **Variables** tab -> click **New repository variable**:

* **`GCP_REGION`**: Target GCP region for Cloud Run and Artifact Registry (default: `asia-southeast1`).
* **`GAR_LOCATION`**: Specific Artifact Registry location if different from `GCP_REGION` (default: value of `GCP_REGION`).
* **`GAR_REPO`**: Artifact Registry Docker repository name (default: `cloudvitta-repo`).
* **`SERVICE_NAME`**: Cloud Run service name (default: `cloudvitta-api`).
* **`GCS_BUCKET_NAME`**: GCS raw fixtures and payload bucket name (default: `cloudvitta-raw-fixtures`).
* **`CORS_ALLOWED_ORIGINS`**: Comma-separated allowed frontend origins (default: `https://cloudvitta.dev`).

---

## Phase 4: Day-to-Day Operations

### Deploying new code
1. Create a branch and push your code. The **CI** pipeline (`ci.yml`) will run tests and linting.
2. Open a Pull Request.
3. Merge the Pull Request into `main`. The **CD** pipeline (`deploy.yml`) will automatically trigger.

### What happens during CD?
1. **Build & Push**: The Go application is packaged into a minimal Docker container and sent to Artifact Registry.
2. **Database Migrations**: `tern` connects to your Neon production database and applies any new forward migrations (`0001_initial_schema.sql`, `0002_create_users_and_refresh_tokens.sql`, `0003_create_fx_rates.sql`). Since we enforce forward-only migrations (ADR 0028), this executes safely on every deploy.
3. **Deploy**: Cloud Run pulls the new image and spins up new instances. Traffic is automatically shifted to the new revision once it passes readiness probes.

### Rollbacks
If a deployment exhibits unexpected behavior:
1. Go to the [Google Cloud Run Console](https://console.cloud.google.com/run).
2. Click on the `cloudvitta-api` service.
3. Go to the **Revisions** tab.
4. Select the previous healthy revision and click **Manage Traffic**.
5. Route 100% of traffic back to the old revision.
6. Fix the issue locally with a new forward commit (or forward migration), and push to `main`.
