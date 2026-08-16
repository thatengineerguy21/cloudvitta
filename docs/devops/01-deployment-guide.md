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

### 1. Repository Secrets (Required)
Go to your repository on GitHub -> **Settings** -> **Secrets and variables** -> **Actions** -> **Secrets** -> **New repository secret**.

1. **`GCP_PROJECT_ID`**: Your Google Cloud Project ID (e.g., `cloudvitta-prod`).
2. **`GCP_SERVICE_ACCOUNT`**: The email of the service account you created (e.g., `cloudvitta-deployer@your-project-id.iam.gserviceaccount.com`).
3. **`GCP_WORKLOAD_IDENTITY_PROVIDER`**: The Workload Identity Provider resource name obtained via:
   ```bash
   gcloud iam workload-identity-pools providers describe "github-provider" \
     --project="${PROJECT_ID}" \
     --location="global" \
     --format="value(name)"
   ```
4. **`NEON_PROD_DSN`**: The connection string to your production Neon Postgres database (e.g., `postgres://user:password@ep-cool-db-1234.us-east-2.aws.neon.tech/neondb?sslmode=require`).
5. **`REDIS_URL`**: The connection string for Upstash/Redis (e.g., `rediss://default:password@us1-cool-redis-1234.upstash.io:32451`).
6. **`CLOUDVITTA_AUTH_JWT_SECRET`** (or **`JWT_SECRET`**): Secret used for signing JWT access tokens (must be at least 32 characters long).
7. **`GRAFANA_OTLP_ENDPOINT`**: Your Grafana Cloud OTLP HTTP endpoint (e.g., `https://otlp-gateway-prod-us-central-0.grafana.net/otlp`).
8. **`GRAFANA_OTLP_HEADERS`**: Base64-encoded basic authentication header for Grafana Cloud (e.g., `Authorization=Basic <BASE64_ENCODED_INSTANCE_ID_AND_TOKEN>`).

### 2. Repository Variables (Optional Overrides)
Go to your repository on GitHub -> **Settings** -> **Secrets and variables** -> **Actions** -> **Variables** -> **New repository variable**.

* **`GCP_REGION`**: Target GCP region for Cloud Run and Artifact Registry (default: `asia-southeast1`).
* **`GAR_LOCATION`**: Specific Artifact Registry location if different from `GCP_REGION` (default: value of `GCP_REGION`).
* **`GAR_REPO`**: Artifact Registry Docker repository name (default: `cloudvitta-repo`).
* **`SERVICE_NAME`**: Cloud Run service name (default: `cloudvitta-api`).
* **`GCS_BUCKET_NAME`**: GCS raw fixtures bucket name (default: `cloudvitta-raw-fixtures`).

---

## Phase 4: Day-to-Day Operations

### Deploying new code
1. Create a branch and push your code. The **CI** pipeline (`ci.yml`) will run tests and linting.
2. Open a Pull Request.
3. Merge the Pull Request into `main`. The **CD** pipeline (`deploy.yml`) will automatically trigger.

### What happens during CD?
1. **Build & Push**: The Go application is packaged into a minimal Docker container and sent to Artifact Registry.
2. **Database Migrations**: `tern` connects to your Neon production database and applies any new migrations. Since we enforce forward-only migrations, this is safe to do automatically.
3. **Deploy**: Cloud Run pulls the new image and spins up new instances. Traffic is automatically shifted to the new revision once it is healthy.

### Rollbacks
If a bad deployment goes out:
1. Go to the [Google Cloud Run Console](https://console.cloud.google.com/run).
2. Click on the `cloudvitta-api` service.
3. Go to the **Revisions** tab.
4. Select the previous healthy revision and click **Manage Traffic**.
5. Route 100% of traffic back to the old revision.
6. Fix the code locally, commit, and push a new fix to `main`.
