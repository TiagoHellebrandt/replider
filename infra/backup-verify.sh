#!/bin/sh
# Weekly backup *restore* verification (US-198).
#
# Backup existence is not the same as backup recoverability: a dump can be
# truncated, reference a dropped extension, or be subtly corrupt and you only
# find out on restore day. This job closes that gap by actually restoring the
# latest dump into a throwaway database every week and running a smoke query.
#
# Runs as the compose service `backup-verify` on the backend network so it can
# reach the `db` host and read the shared `pgbackups` volume populated by the
# `pg-backup` service. On failure it posts to the same Slack/Discord webhooks
# Alertmanager uses (infra/alertmanager.yml); on success it touches a freshness
# marker the Docker healthcheck gates on, so a hung or crashed verifier also
# surfaces as unhealthy.
#
# The throwaway DB lives on the same `db` server, so extensions the dump needs
# (e.g. pgvector) are already installed — we are validating the dump, not the
# server build.
#
# Manual one-shot run on the VPS:
#   docker compose -f docker-compose.prod.yml run --rm -e RUN_ONCE=1 backup-verify
set -u

BACKUP_DIR="${BACKUP_DIR:-/backups}"
VERIFY_DB="${VERIFY_DB:-replider_verify}"
MARKER="${BACKUP_DIR}/.last_verify_ok"
INTERVAL="${VERIFY_INTERVAL:-604800}"   # weekly
SMOKE_TABLE="${SMOKE_TABLE:-accounts}"
PSQL="psql -h db -U ${DB_USER}"

# Post a failure to whichever chat webhooks are configured. curl when present
# (we apk-add it below), else busybox wget — both understand --post-data / -d.
http_post() {
  _url="$1"; _body="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsS -m 10 -H 'Content-Type: application/json' -d "${_body}" "${_url}" >/dev/null 2>&1
  else
    wget -q -O /dev/null -T 10 --header='Content-Type: application/json' \
      --post-data="${_body}" "${_url}" 2>/dev/null
  fi
}

alert() {
  _msg="🚨 [BACKUP-VERIFY] $1"
  echo "[backup-verify] FAILURE: $1" >&2
  if [ -n "${SLACK_ALERT_WEBHOOK_URL:-}" ]; then
    http_post "${SLACK_ALERT_WEBHOOK_URL}" "{\"text\":\"${_msg}\"}" || true
  fi
  if [ -n "${DISCORD_ALERT_WEBHOOK_URL:-}" ]; then
    http_post "${DISCORD_ALERT_WEBHOOK_URL}" "{\"content\":\"${_msg}\"}" || true
  fi
}

cleanup_db() {
  dropdb -h db -U "${DB_USER}" --if-exists "${VERIFY_DB}" >/dev/null 2>&1 || true
}

verify_once() {
  latest=$(ls -1t "${BACKUP_DIR}"/*.sql.gz 2>/dev/null | head -n1)
  if [ -z "${latest}" ]; then
    alert "no backup dump found in ${BACKUP_DIR}"
    return 1
  fi
  echo "[backup-verify] verifying ${latest}"

  # gzip integrity first — cheap, catches truncated dumps before a full restore.
  if ! gzip -t "${latest}" 2>/dev/null; then
    alert "gzip integrity check failed for ${latest}"
    return 1
  fi

  # Restore into a fresh throwaway DB (drop any leftover from a prior crash).
  cleanup_db
  if ! createdb -h db -U "${DB_USER}" "${VERIFY_DB}" 2>/tmp/verify.err; then
    alert "could not create throwaway DB ${VERIFY_DB}: $(tail -n2 /tmp/verify.err)"
    return 1
  fi

  if ! gunzip -c "${latest}" | ${PSQL} -v ON_ERROR_STOP=1 -q "${VERIFY_DB}" >/dev/null 2>/tmp/verify.err; then
    alert "restore of ${latest} failed: $(tail -n3 /tmp/verify.err)"
    cleanup_db
    return 1
  fi

  # Smoke query: the core table must exist and be queryable in the restore.
  count=$(${PSQL} -t -A -c "SELECT count(*) FROM ${SMOKE_TABLE};" "${VERIFY_DB}" 2>/tmp/verify.err)
  rc=$?
  cleanup_db
  if [ "${rc}" -ne 0 ]; then
    alert "smoke query on ${SMOKE_TABLE} failed: $(tail -n3 /tmp/verify.err)"
    return 1
  fi

  echo "[backup-verify] OK — ${latest} restored, ${SMOKE_TABLE} rows=${count}"
  date +%s > "${MARKER}"
  return 0
}

# curl gives nicer webhook semantics than busybox wget; best-effort install.
apk add --no-cache curl >/dev/null 2>&1 || true

if [ "${RUN_ONCE:-0}" = "1" ]; then
  verify_once
  exit $?
fi

# Give the daily pg-backup loop time to produce a first dump on a cold start.
sleep "${STARTUP_DELAY:-120}"
while true; do
  verify_once || true
  echo "[backup-verify] sleeping ${INTERVAL}s"
  sleep "${INTERVAL}"
done
