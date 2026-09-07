#!/usr/bin/env bash
# ==============================================================================
# Script: setup-gcs-lifecycle.sh
# Purpose: Configure lifecycle rules on the CloudVitta Google Cloud Storage bucket.
# Language standard: ASD-STE100 Simple Technical English comments.
#
# Rules:
#   1. Transition raw provider response objects to NEARLINE storage after 30 days.
#   2. Transition raw provider response objects to COLDLINE storage after 90 days.
#   3. Delete raw provider response objects after 180 days.
# ==============================================================================

set -euo pipefail

BUCKET_NAME="${1:-${CLOUDVITTA_STORAGE_GCS_BUCKET_NAME:-${GCS_BUCKET_NAME:-cloudvitta-raw-fixtures}}}"

echo "Configuring Google Cloud Storage lifecycle policy for bucket: gs://${BUCKET_NAME}"

# Create a temporary file for the lifecycle JSON configuration
LIFECYCLE_CONFIG=$(mktemp /tmp/gcs-lifecycle-XXXXXX.json 2>/dev/null || mktemp -t 'gcs-lifecycle.json')
trap 'rm -f "${LIFECYCLE_CONFIG}"' EXIT

cat > "${LIFECYCLE_CONFIG}" <<'EOF'
{
  "rule": [
    {
      "action": {
        "type": "SetStorageClass",
        "storageClass": "NEARLINE"
      },
      "condition": {
        "age": 30,
        "matchesPrefix": ["raw/"]
      }
    },
    {
      "action": {
        "type": "SetStorageClass",
        "storageClass": "COLDLINE"
      },
      "condition": {
        "age": 90,
        "matchesPrefix": ["raw/"]
      }
    },
    {
      "action": {
        "type": "Delete"
      },
      "condition": {
        "age": 180,
        "matchesPrefix": ["raw/"]
      }
    }
  ]
}
EOF

# Apply lifecycle policy using gcloud storage (preferred) or gsutil fallback
if command -v gcloud >/dev/null 2>&1; then
  echo "Applying lifecycle policy using 'gcloud storage buckets update'..."
  gcloud storage buckets update "gs://${BUCKET_NAME}" --lifecycle-file="${LIFECYCLE_CONFIG}"
elif command -v gsutil >/dev/null 2>&1; then
  echo "Applying lifecycle policy using 'gsutil lifecycle set'..."
  gsutil lifecycle set "${LIFECYCLE_CONFIG}" "gs://${BUCKET_NAME}"
else
  echo "ERROR: Neither 'gcloud' nor 'gsutil' CLI was found in PATH." >&2
  exit 1
fi

echo "Successfully applied lifecycle policy to gs://${BUCKET_NAME}."
