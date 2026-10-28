#!/usr/bin/env bash
# =============================================================================
# backup-drill.sh — prove the LATEST wal-g base backup restores, from the VPS.
#
# A thin, VPS-friendly front for scripts/backup-restore-drill.sh: that script
# expects a credentials FILE (GOOGLE_APPLICATION_CREDENTIALS) and the wal-g
# prefix in the environment, while the production stack keeps everything in
# ~/replider/.env as base64 (no secret files on disk). This script reads the
# stack's .env, materialises the service-account JSON into a 0600 temp file
# for the duration of the drill, and runs the restore into a THROWAWAY
# container (never the live db). It ends with `SELECT count(*) FROM accounts`
# on the restored cluster.
#
# Usage (on the VPS):
#   scripts/backup-drill.sh                 # reads ~/replider/.env
#   STACK_DIR=/path scripts/backup-drill.sh
#
# Exit 0 = the latest backup restored and had accounts in it. Anything else
# is the outage you were buying insurance against — page somebody.
# Cron suggestion (monthly, after the 03:15 base backup):
#   0 5 1 * * /home/replider/replider/scripts/backup-drill.sh >> /var/log/walg-drill.log 2>&1 \
#     || curl -fsS -X POST -H 'Content-Type: application/json' \
#          -d '{"text":"🚨 Replider: o drill de restore do backup FALHOU"}' "$ALERT_WEBHOOK_URL"
# =============================================================================
set -euo pipefail

STACK_DIR="${STACK_DIR:-$HOME/replider}"
ENV_FILE="${STACK_DIR}/.env"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DRILL="${SCRIPT_DIR}/backup-restore-drill.sh"

log()  { printf '[backup-drill] %s\n' "$*"; }
fail() { printf '[backup-drill] FAILED: %s\n' "$*" >&2; exit 1; }

[ -f "${ENV_FILE}" ] || fail "no ${ENV_FILE} (set STACK_DIR)"
[ -x "${DRILL}" ] || [ -f "${DRILL}" ] || fail "missing ${DRILL}"
command -v docker >/dev/null || fail "docker not found"

# Read one KEY="value" / KEY=value line from the stack .env without sourcing
# it (values may contain shell metacharacters).
envval() {
  local key="$1"
  grep -E "^${key}=" "${ENV_FILE}" | head -1 | cut -d= -f2- | sed -e 's/^"//' -e 's/"$//'
}

WALG_GS_PREFIX="$(envval WALG_GS_PREFIX)"
DB_USER="$(envval DB_USER)"
DB_NAME="$(envval DB_NAME)"
CRED_B64="$(envval WALG_GOOGLE_CREDENTIALS_BASE64)"
[ -n "${CRED_B64}" ] || CRED_B64="$(envval GOOGLE_CREDENTIALS_BASE64)"

[ -n "${WALG_GS_PREFIX}" ] || fail "WALG_GS_PREFIX is empty in ${ENV_FILE} — continuous backup is OFF (docs/backup-walg.md, checklist)"
[ -n "${DB_USER}" ] || fail "DB_USER missing in ${ENV_FILE}"
[ -n "${CRED_B64}" ] || fail "neither WALG_GOOGLE_CREDENTIALS_BASE64 nor GOOGLE_CREDENTIALS_BASE64 set — wal-g cannot read the bucket"

# The image the live db runs (built from docker/postgres-walg by the deploy).
WALG_IMAGE="${WALG_IMAGE:-replider-pg-walg:pg16}"
docker image inspect "${WALG_IMAGE}" >/dev/null 2>&1 \
  || fail "image ${WALG_IMAGE} not found locally; the db service builds it on deploy (docker compose up -d --build db)"

# Materialise the credential for the drill only. 0600, removed on exit.
CRED_FILE="$(mktemp "${TMPDIR:-/tmp}/walg-drill-cred.XXXXXX")"
chmod 600 "${CRED_FILE}"
trap 'rm -f "${CRED_FILE}"' EXIT
printf '%s' "${CRED_B64}" | base64 -d > "${CRED_FILE}" || fail "credential is not valid base64"
grep -q '"private_key"' "${CRED_FILE}" || fail "decoded credential does not look like a service-account JSON"

log "restoring LATEST from ${WALG_GS_PREFIX} into a throwaway container (image ${WALG_IMAGE})"
WALG_GS_PREFIX="${WALG_GS_PREFIX}" \
GOOGLE_APPLICATION_CREDENTIALS="${CRED_FILE}" \
WALG_IMAGE="${WALG_IMAGE}" \
DB_USER="${DB_USER}" \
POSTGRES_DB="${DB_NAME:-replider_db}" \
  bash "${DRILL}"
# backup-restore-drill.sh already ran `SELECT count(*) FROM accounts` on the
# restored cluster and printed "RESTORE DRILL OK: N tables, M accounts".
log "OK — the latest base backup restores and answers SELECT count(*) FROM accounts"
