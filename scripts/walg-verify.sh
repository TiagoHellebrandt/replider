#!/usr/bin/env bash
# US-F002: assert an off-site base backup exists AND is fresh.
#
# A backup job that silently stopped looks exactly like one that is working:
# both produce no output. This script is what turns that silence into a
# non-zero exit — wire it to cron/monitoring, not to a human's memory.
#
# NOTE: not to be confused with infra/backup-verify.sh (US-198), which
# restore-checks the LOCAL pg_dump on the same VPS. This one checks the
# OFF-SITE wal-g archive — the copy that survives losing the machine.
#
# Usage (on the VPS):
#   ./scripts/backup-verify.sh                 # defaults to the db container
#   DB_CONTAINER=replider_db MAX_AGE_HOURS=26 ./scripts/backup-verify.sh
#
# Exit codes: 0 = fresh backup found, 1 = missing/stale/unreadable.
set -Eeuo pipefail

DB_CONTAINER="${DB_CONTAINER:-replider_db}"
# 26h, not 24h: the daily job needs slack for a slow backup or a restart
# without paging anyone at 03:05.
MAX_AGE_HOURS="${MAX_AGE_HOURS:-26}"

fail() { echo "BACKUP CHECK FAILED: $*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || fail "docker not available"

if ! docker ps --format '{{.Names}}' | grep -qx "${DB_CONTAINER}"; then
  fail "container ${DB_CONTAINER} is not running"
fi

# `wal-g backup-list` prints a header plus one line per backup:
#   backup_name  modified  wal_file_name  storage_name
LIST="$(docker exec "${DB_CONTAINER}" wal-g backup-list 2>/dev/null || true)"
[ -n "${LIST}" ] || fail "wal-g backup-list produced no output (not configured, or the bucket is unreachable)"

LATEST_LINE="$(echo "${LIST}" | awk 'NR>1 && NF>=2' | tail -n 1)"
[ -n "${LATEST_LINE}" ] || fail "no base backup found in the bucket"

BACKUP_NAME="$(echo "${LATEST_LINE}" | awk '{print $1}')"
MODIFIED="$(echo "${LATEST_LINE}" | awk '{print $2}')"

# GNU date (the container has it) parses the RFC3339 stamp wal-g prints; the
# host's date may be BSD, so do the arithmetic inside the container.
AGE_SECONDS="$(docker exec "${DB_CONTAINER}" sh -c \
  "echo \$(( \$(date -u +%s) - \$(date -u -d '${MODIFIED}' +%s) ))" 2>/dev/null || true)"

case "${AGE_SECONDS}" in
  ''|*[!0-9-]*) fail "could not compute the age of backup ${BACKUP_NAME} (modified=${MODIFIED})" ;;
esac

MAX_AGE_SECONDS=$(( MAX_AGE_HOURS * 3600 ))
AGE_HOURS=$(( AGE_SECONDS / 3600 ))

if [ "${AGE_SECONDS}" -gt "${MAX_AGE_SECONDS}" ]; then
  fail "latest backup ${BACKUP_NAME} is ${AGE_HOURS}h old (limit ${MAX_AGE_HOURS}h) — archiving or the daily base backup is broken"
fi

echo "OK: latest backup ${BACKUP_NAME} is ${AGE_HOURS}h old (limit ${MAX_AGE_HOURS}h)"
echo "${LIST}" | tail -n 5
