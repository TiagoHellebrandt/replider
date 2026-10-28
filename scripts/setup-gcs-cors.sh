#!/usr/bin/env bash
# Apply CORS policy to the Replider GCS bucket so the Flutter dashboard
# can PUT media uploads directly via signed URLs.
#
# Without this, the browser's preflight OPTIONS request to
# storage.googleapis.com returns no Access-Control-Allow-Origin header
# and the upload fails with a net::ERR_FAILED / CORS policy error.
#
# Usage:
#   BUCKET=replider ./scripts/setup-gcs-cors.sh
#   # or with a custom bucket:
#   BUCKET=my-bucket ./scripts/setup-gcs-cors.sh
#
# Requires: gcloud CLI authenticated against the project owning the bucket.
set -euo pipefail

BUCKET="${BUCKET:-replider}"
CORS_FILE="$(cd "$(dirname "$0")" && pwd)/gcs-cors.json"

if ! command -v gcloud >/dev/null 2>&1; then
  echo "error: gcloud CLI not found in PATH" >&2
  exit 1
fi

if [[ ! -f "$CORS_FILE" ]]; then
  echo "error: cors file not found at $CORS_FILE" >&2
  exit 1
fi

echo "Applying CORS policy to gs://${BUCKET} from ${CORS_FILE}..."
gcloud storage buckets update "gs://${BUCKET}" --cors-file="${CORS_FILE}"

echo "Done. Current CORS on gs://${BUCKET}:"
gcloud storage buckets describe "gs://${BUCKET}" --format="value(cors_config)"
