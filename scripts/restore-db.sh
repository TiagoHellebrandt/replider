#!/bin/sh
# Restore PostgreSQL from a backup file.
#
# Supports two backup sources:
#   1. Docker volume (new): backups created by the pg-backup container
#      stored in the named Docker volume 'replider_pgbackups'
#   2. Host directory (legacy): ~/replider/backups/
#
# Usage:
#   Restore from Docker volume (interactive picker):
#     ./restore-db.sh
#
#   Restore a specific file from host:
#     ./restore-db.sh ~/replider/backups/replider-20260407-030000.sql.gz
#
# WARNING: This will DROP and RECREATE the database. All current data will be lost.

set -e

COMPOSE_FILE=~/replider/docker-compose.prod.yml
DB_USER="${DB_USER:-replider}"
DB_NAME="${DB_NAME:-replider_db}"

# ── Pick backup file ───────────────────────────────────────────────────────────
if [ -n "$1" ]; then
  # Explicit path provided (legacy / manual)
  BACKUP_FILE="$1"
  if [ ! -f "$BACKUP_FILE" ]; then
    echo "[restore] ERROR: File not found: $BACKUP_FILE"
    exit 1
  fi
  RESTORE_FROM_VOLUME=0
else
  # Interactive: list backups from Docker volume
  echo "[restore] Listing backups from Docker volume (replider_pgbackups):"
  BACKUPS=$(docker run --rm \
    -v replider_pgbackups:/backups \
    alpine ls -1t /backups/*.sql.gz 2>/dev/null || true)

  if [ -z "$BACKUPS" ]; then
    echo "  (no backups found in volume)"
    echo ""
    echo "[restore] Checking host directory ~/replider/backups/ ..."
    ls -lh ~/replider/backups/replider-*.sql.gz 2>/dev/null || echo "  (none found)"
    echo ""
    echo "Usage: $0 <backup_file.sql.gz>"
    exit 1
  fi

  echo ""
  i=1
  for f in $BACKUPS; do
    echo "  $i) $(basename $f)"
    i=$((i+1))
  done
  echo ""
  printf "[restore] Enter number to restore (or Ctrl+C to abort): "
  read -r CHOICE

  BACKUP_FILE=$(echo "$BACKUPS" | sed -n "${CHOICE}p")
  if [ -z "$BACKUP_FILE" ]; then
    echo "[restore] Invalid choice."
    exit 1
  fi
  RESTORE_FROM_VOLUME=1
fi

# ── Confirmation ──────────────────────────────────────────────────────────────
echo ""
echo "[restore] ⚠️  WARNING: This will DROP database '$DB_NAME' and recreate it."
if [ "$RESTORE_FROM_VOLUME" = "1" ]; then
  echo "[restore] Backup: volume://replider_pgbackups/$(basename $BACKUP_FILE)"
else
  echo "[restore] Backup: $BACKUP_FILE"
fi
echo ""
printf "[restore] Type 'yes' to continue: "
read -r CONFIRM
if [ "$CONFIRM" != "yes" ]; then
  echo "[restore] Aborted."
  exit 0
fi

# ── Run restore ───────────────────────────────────────────────────────────────
echo "[restore] Stopping API to prevent new writes..."
docker compose -f "$COMPOSE_FILE" stop api

echo "[restore] Dropping and recreating database '$DB_NAME'..."
docker compose -f "$COMPOSE_FILE" exec -T db \
  psql -U "$DB_USER" -d postgres -c "DROP DATABASE IF EXISTS ${DB_NAME};"
docker compose -f "$COMPOSE_FILE" exec -T db \
  psql -U "$DB_USER" -d postgres -c "CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};"

echo "[restore] Restoring data..."
if [ "$RESTORE_FROM_VOLUME" = "1" ]; then
  docker run --rm \
    -v replider_pgbackups:/backups \
    --network replider_backend \
    -e PGPASSWORD="$DB_PASSWORD" \
    postgres:16-alpine \
    sh -c "gunzip -c ${BACKUP_FILE} | psql -h db -U ${DB_USER} -d ${DB_NAME}"
else
  gunzip -c "$BACKUP_FILE" | docker compose -f "$COMPOSE_FILE" exec -T db \
    psql -U "$DB_USER" -d "$DB_NAME"
fi

echo "[restore] Starting API (will run migrations if needed)..."
docker compose -f "$COMPOSE_FILE" up -d api

echo ""
echo "[restore] ✅ Done. Verify the service is working correctly."
echo "[restore]    Check logs: docker compose -f $COMPOSE_FILE logs -f api"
