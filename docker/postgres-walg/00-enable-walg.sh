#!/usr/bin/env bash
# Runs inside the stock entrypoint's initdb phase (first boot only).
#
# Why here and not in the wrapper: `archive_mode` is a RESTART-only GUC. The
# wrapper appends the config after the server is already up, so archiving
# would stay OFF until the next container restart — a fresh deploy would take
# its first base backup with no WAL archiving behind it, and that backup is
# unrestorable. Patching during initdb means the FINAL server start (the one
# the entrypoint does after these scripts) already has archiving on.
set -Eeuo pipefail

MARKER='# --- wal-g (US-F002) ---'
CONF="${PGDATA:-/var/lib/postgresql/data}/postgresql.conf"

if [ -z "${WALG_GS_PREFIX:-}" ] && [ -z "${WALG_FILE_PREFIX:-}" ]; then
  echo "[wal-g] no WALG_GS_PREFIX/WALG_FILE_PREFIX — archiving disabled (dev mode)"
  exit 0
fi

if grep -qF "${MARKER}" "${CONF}" 2>/dev/null; then
  exit 0
fi

{
  echo ""
  echo "${MARKER}"
  cat /etc/postgresql/postgresql.walg.conf
} >> "${CONF}"
echo "[wal-g] archiving enabled in postgresql.conf (applies from the next server start)"
