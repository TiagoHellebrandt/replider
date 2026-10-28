#!/usr/bin/env bash
# Entrypoint wrapper that enables wal-g archiving on top of the stock
# postgres entrypoint (US-F002).
#
# Design notes:
#   * The config is APPENDED to postgresql.conf once (idempotent marker), not
#     passed as CLI flags, so it applies to every start path.
#   * When wal-g is not configured (no bucket), archiving stays OFF and the
#     container behaves exactly like the stock image — dev must not need GCS
#     credentials to run the stack.
set -Eeuo pipefail

MARKER='# --- wal-g (US-F002) ---'
PGDATA_DIR="${PGDATA:-/var/lib/postgresql/data}"

# The deploy injects secrets as environment variables — there are no files
# on the VPS by design. wal-g, though, only reads a service-account JSON from
# a PATH, so accept the same base64 form everything else in this stack uses
# and materialise it here.
#
# Written to /tmp on purpose: it is inside the container, outside PGDATA (so
# it can never end up inside a backup of itself), and gone when the container
# is. Never echoed — a leaked service account is a leaked bucket.
materialize_gcs_credentials() {
  # WALG_-specific wins; otherwise reuse the account the API already has.
  # Sharing one service account across media, Calendar and backups is a
  # deliberate call by the operator (a leak means rotating one key). The
  # fallback exists so that rotation really IS one key: a duplicated secret
  # gets rotated in one place and forgotten in the other, and the copy left
  # behind is the backup — authenticating with a dead key, silently.
  local b64="${WALG_GOOGLE_CREDENTIALS_BASE64:-${GOOGLE_CREDENTIALS_BASE64:-}}"
  [ -n "$b64" ] || return 0
  # An explicit path wins: an operator who mounted a file meant it.
  if [ -n "${GOOGLE_APPLICATION_CREDENTIALS:-}" ] && [ -f "${GOOGLE_APPLICATION_CREDENTIALS}" ]; then
    return 0
  fi
  local target=/tmp/walg-gcs.json
  if ! printf '%s' "$b64" | base64 -d > "$target" 2>/dev/null; then
    echo "[wal-g] WALG_GOOGLE_CREDENTIALS_BASE64 is not valid base64 — archiving will fail" >&2
    rm -f "$target"
    return 1
  fi
  # archive_command runs as postgres, which inherits this environment.
  chmod 600 "$target"
  chown postgres:postgres "$target" 2>/dev/null || true
  export GOOGLE_APPLICATION_CREDENTIALS="$target"
  echo "[wal-g] service-account credentials loaded from env"
}

walg_configured() {
  [ -n "${WALG_GS_PREFIX:-}" ] || [ -n "${WALG_FILE_PREFIX:-}" ]
}

enable_archiving() {
  local conf="${PGDATA_DIR}/postgresql.conf"
  [ -f "$conf" ] || return 0
  if grep -qF "$MARKER" "$conf"; then
    return 0
  fi
  {
    echo ""
    echo "$MARKER"
    cat /etc/postgresql/postgresql.walg.conf
  } >> "$conf"
  echo "[wal-g] archiving enabled in postgresql.conf"
}

# The FILE backend (dev drills / CI, where there is no GCS) writes as the
# postgres user, but a bind-mounted or fresh named volume lands owned by root
# — the archive_command then fails forever with "permission denied" while the
# server looks perfectly healthy. Irrelevant for GCS, cheap to fix here.
if [ -n "${WALG_FILE_PREFIX:-}" ] && [ "$(id -u)" = "0" ]; then
  # Create it too, not just chown: a missing directory makes every
  # archive_command fail forever while the server reports itself perfectly
  # healthy — the same silent rot the ownership fix above exists to avoid.
  mkdir -p "${WALG_FILE_PREFIX}" || true
  chown -R postgres:postgres "${WALG_FILE_PREFIX}" || true
fi

if walg_configured; then
  materialize_gcs_credentials || true
  # EXISTING cluster: patch before the server starts, so archive_mode (a
  # restart-only GUC) is live from this very boot. A FRESH cluster is handled
  # by /docker-entrypoint-initdb.d/00-enable-walg.sh, which runs after initdb
  # and before the final server start.
  if [ -f "${PGDATA_DIR}/PG_VERSION" ]; then
    enable_archiving
  fi
else
  echo "[wal-g] no WALG_GS_PREFIX/WALG_FILE_PREFIX set — archiving disabled (dev mode)"
fi

exec docker-entrypoint.sh "$@"
