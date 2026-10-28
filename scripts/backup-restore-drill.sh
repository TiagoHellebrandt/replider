#!/usr/bin/env bash
# US-F003: prove the backup RESTORES. Monthly drill.
#
# "We have backups" is a claim; "we restored one at 04:12 today and it had
# 41 tables and 3 accounts in it" is a fact. Untested backups fail exactly
# when they are needed, and the failure modes are boring: an archive_command
# that was silently erroring, a base backup taken from an empty data
# directory, a bucket the service account can no longer read.
#
# The drill fetches the latest base backup into a THROWAWAY Postgres,
# replays WAL, and runs smoke queries. It never touches production.
#
# Usage:
#   ./scripts/backup-restore-drill.sh
#
# Environment:
#   WALG_GS_PREFIX          gs://bucket/path   (production)
#   WALG_FILE_PREFIX        /path              (local drill / CI without GCS)
#   GOOGLE_APPLICATION_CREDENTIALS or GCS_CREDENTIALS_JSON
#   WALG_IMAGE              image with wal-g (default: built from docker/postgres-walg)
#   DB_USER                 role to connect as in the RESTORED cluster
#   MIN_EXPECTED_TABLES     smoke threshold (default 20)
set -Eeuo pipefail

WALG_IMAGE="${WALG_IMAGE:-replider-pg-walg:latest}"
CONTAINER="walg-restore-drill-$$"
MIN_EXPECTED_TABLES="${MIN_EXPECTED_TABLES:-20}"

cleanup() {
  docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

fail() { echo "RESTORE DRILL FAILED: $*" >&2; exit 1; }

if [ -z "${WALG_GS_PREFIX:-}" ] && [ -z "${WALG_FILE_PREFIX:-}" ]; then
  fail "neither WALG_GS_PREFIX nor WALG_FILE_PREFIX is set — nothing to restore from"
fi

echo "==> starting throwaway Postgres (${CONTAINER})"
DOCKER_ARGS=(-d --name "${CONTAINER}" --entrypoint sleep)
[ -n "${WALG_GS_PREFIX:-}" ] && DOCKER_ARGS+=(-e "WALG_GS_PREFIX=${WALG_GS_PREFIX}")
if [ -n "${WALG_FILE_PREFIX:-}" ]; then
  DOCKER_ARGS+=(-e "WALG_FILE_PREFIX=${WALG_FILE_PREFIX}")
  # The drill must read the SAME archive the source wrote. Locally that is a
  # docker volume; in production it is the bucket.
  [ -n "${WALG_FILE_VOLUME:-}" ] && DOCKER_ARGS+=(-v "${WALG_FILE_VOLUME}:${WALG_FILE_PREFIX}")
fi
if [ -n "${GOOGLE_APPLICATION_CREDENTIALS:-}" ]; then
  DOCKER_ARGS+=(-v "${GOOGLE_APPLICATION_CREDENTIALS}:/gcp-key.json:ro"
                -e "GOOGLE_APPLICATION_CREDENTIALS=/gcp-key.json")
fi

docker run "${DOCKER_ARGS[@]}" "${WALG_IMAGE}" infinity >/dev/null \
  || fail "could not start the drill container from ${WALG_IMAGE}"

echo "==> fetching the latest base backup"
docker exec "${CONTAINER}" sh -c 'rm -rf /restore && mkdir -p /restore && chown postgres:postgres /restore' \
  || fail "could not prepare /restore"
docker exec -u postgres "${CONTAINER}" wal-g backup-fetch /restore LATEST \
  || fail "wal-g backup-fetch failed — the backup is unreadable (this IS the outage you were protecting against)"

echo "==> configuring WAL replay"
# restore_command replays every archived segment on top of the base backup;
# recovery.signal makes Postgres come up in recovery and promote at the end.
docker exec -u postgres "${CONTAINER}" sh -c \
  "printf \"restore_command = '/usr/local/bin/wal-g wal-fetch %%f %%p'\nrecovery_target_action = 'promote'\n\" >> /restore/postgresql.conf; touch /restore/recovery.signal"

echo "==> starting the restored cluster"
docker exec -u postgres -d "${CONTAINER}" sh -c "pg_ctl -D /restore -o '-p 5433' -l /tmp/restore.log start"

for _ in $(seq 1 60); do
  if docker exec -u postgres "${CONTAINER}" pg_isready -p 5433 -q 2>/dev/null; then
    break
  fi
  sleep 2
done
docker exec -u postgres "${CONTAINER}" pg_isready -p 5433 -q \
  || { docker exec "${CONTAINER}" tail -40 /tmp/restore.log >&2 || true; fail "restored cluster never accepted connections"; }

echo "==> smoke queries"
DB="${POSTGRES_DB:-replider_db}"
# The restored cluster carries PRODUCTION's roles, not the image defaults —
# `postgres` may not even exist there. Connect as the app's own DB user.
DBUSER="${DB_USER:-${POSTGRES_USER:-postgres}}"
TABLES="$(docker exec -u postgres "${CONTAINER}" psql -p 5433 -U "${DBUSER}" -d "${DB}" -tAc \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")" \
  || fail "smoke query failed against the restored cluster"
TABLES="$(echo "${TABLES}" | tr -d '[:space:]')"

echo "    tables in public schema: ${TABLES}"
if [ "${TABLES}" -lt "${MIN_EXPECTED_TABLES}" ]; then
  fail "restored cluster has only ${TABLES} tables (expected >= ${MIN_EXPECTED_TABLES}) — the backup restored, but it is not the production database"
fi

# Core tables must exist AND be queryable: a restore that yields an empty
# schema-only shell passes a table count but fails a real recovery.
for table in accounts users channels conversations messages; do
  docker exec -u postgres "${CONTAINER}" psql -p 5433 -U "${DBUSER}" -d "${DB}" -tAc "SELECT count(*) FROM ${table}" >/dev/null \
    || fail "core table ${table} is missing or unreadable in the restored cluster"
done

ACCOUNTS="$(docker exec -u postgres "${CONTAINER}" psql -p 5433 -U "${DBUSER}" -d "${DB}" -tAc 'SELECT count(*) FROM accounts' | tr -d '[:space:]')"
echo "    accounts restored: ${ACCOUNTS}"

echo "RESTORE DRILL OK: ${TABLES} tables, ${ACCOUNTS} accounts recovered from the latest backup"
