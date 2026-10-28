#!/bin/sh
# Daily PostgreSQL backup.
#
# Local-only by default. When BACKUP_S3_BUCKET and the usual AWS_* env
# vars are set, the dump is also uploaded off-site — that's the variant
# you want for any real production, because a VPS disk failure with only
# local dumps is the same as no backup at all.
#
# Usage:
#   BACKUP_S3_BUCKET=s3://replider-backups ./backup-db.sh
#
# Requires:
#   - docker compose CLI (for pg_dump via db service)
#   - aws CLI + credentials when BACKUP_S3_BUCKET is set
#
# Off-site retention lives in the bucket's lifecycle policy (set once in
# the bucket config), not here — the script is idempotent on purpose so
# a botched cron doesn't accidentally delete historical snapshots.

set -eu

BACKUP_DIR="${BACKUP_DIR:-$HOME/replider/backups}"
LOCAL_RETENTION_DAYS="${LOCAL_RETENTION_DAYS:-30}"
TIMESTAMP=$(date -u +%Y%m%dT%H%M%SZ)
BACKUP_FILE="${BACKUP_DIR}/replider-${TIMESTAMP}.sql.gz"

mkdir -p "$BACKUP_DIR"

echo "[backup] starting pg_dump..."
docker compose -f ~/replider/docker-compose.prod.yml exec -T db \
  pg_dump -U "${DB_USER:-replider}" "${DB_NAME:-replider_db}" \
  | gzip > "$BACKUP_FILE"

# Sanity check the gzip — a truncated dump is worse than no dump because
# it masks the failure until restore day. Fail loudly.
if ! gzip -t "$BACKUP_FILE" 2>/dev/null; then
  echo "[backup] FATAL: gzip integrity check failed for $BACKUP_FILE" >&2
  rm -f "$BACKUP_FILE"
  exit 1
fi

SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
echo "[backup] local dump ok: $BACKUP_FILE ($SIZE)"

# Off-site upload (optional but strongly recommended). We use the AWS
# CLI because it supports both S3 and S3-compatible endpoints (Cloudflare
# R2, Backblaze B2, Wasabi, MinIO) via --endpoint-url.
if [ -n "${BACKUP_S3_BUCKET:-}" ]; then
  echo "[backup] uploading to $BACKUP_S3_BUCKET..."
  if [ -n "${BACKUP_S3_ENDPOINT:-}" ]; then
    aws --endpoint-url "$BACKUP_S3_ENDPOINT" s3 cp "$BACKUP_FILE" "$BACKUP_S3_BUCKET/" --quiet
  else
    aws s3 cp "$BACKUP_FILE" "$BACKUP_S3_BUCKET/" --quiet
  fi
  echo "[backup] off-site upload ok"
else
  echo "[backup] BACKUP_S3_BUCKET unset — skipping off-site upload (NOT recommended for production)"
fi

# Local pruning (off-site retention is handled by bucket lifecycle rules).
echo "[backup] pruning local dumps older than ${LOCAL_RETENTION_DAYS} days..."
find "$BACKUP_DIR" -name "replider-*.sql.gz" -mtime +"${LOCAL_RETENTION_DAYS}" -delete

echo "[backup] done."
