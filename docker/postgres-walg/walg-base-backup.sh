#!/usr/bin/env bash
# Daily base backup (US-F002). WAL archiving alone cannot restore anything —
# recovery replays WAL ON TOP of a base backup, so a missing base makes every
# archived segment useless.
#
# Run from the DB container (it needs the data directory), e.g. via cron on
# the host:
#   docker exec replider_db /usr/local/bin/walg-base-backup.sh
set -Eeuo pipefail

PGDATA_DIR="${PGDATA:-/var/lib/postgresql/data}"

# wal-g opens a control connection to take the backup. Under `docker exec` the
# effective OS user is root, which is not a Postgres role — without these the
# push dies with `role "root" does not exist`. Defaults come from the image's
# own POSTGRES_* env so the script needs no extra configuration.
export PGHOST="${PGHOST:-/var/run/postgresql}"
export PGUSER="${PGUSER:-${POSTGRES_USER:-postgres}}"
export PGDATABASE="${PGDATABASE:-${POSTGRES_DB:-postgres}}"

if [ -z "${WALG_GS_PREFIX:-}" ] && [ -z "${WALG_FILE_PREFIX:-}" ]; then
  echo "[wal-g] not configured — nothing to back up" >&2
  exit 0
fi

echo "[wal-g] starting base backup of ${PGDATA_DIR}"
wal-g backup-push "${PGDATA_DIR}"

# Force the WAL segment holding the backup's checkpoint into the archive.
#
# Without this the base backup is UNRESTORABLE until the current segment
# fills or archive_timeout (5 min) elapses: recovery starts, asks for that
# segment, wal-g answers "Archive '0000000100000000000000XX' does not exist",
# and Postgres dies with "could not locate required checkpoint record" — while
# `wal-g backup-list` shows a perfectly healthy backup the whole time.
#
# Found by the US-F003 restore drill on its first run. This is the entire
# argument for drills: the failure is invisible from the backup side.
echo "[wal-g] flushing the checkpoint WAL segment to the archive"
psql -tAc "SELECT pg_switch_wal()" >/dev/null
psql -tAc "CHECKPOINT" >/dev/null

# The segment the new backup NEEDS is the one wal-g recorded with it. Compare
# against that, not against "did last_archived_wal change" — pg_stop_backup
# already rotates the log, so the extra switch above is frequently a no-op and
# a change-detector would report failure on a perfectly good backup.
#
# WAL names sort lexicographically within a timeline, so `>` is a valid
# "has the archiver caught up?" test. The `.backup`/`.partial` suffixes that
# pg_stat_archiver can report are stripped first.
REQUIRED_WAL="$(wal-g backup-list 2>/dev/null | awk 'NR>1 && NF>=3 {w=$3} END {print w}')"
[ -n "${REQUIRED_WAL}" ] || { echo "[wal-g] ERROR: could not read the backup's required WAL segment" >&2; exit 1; }

ARCHIVED=""
for _ in $(seq 1 30); do
  ARCHIVED="$(psql -tAc "SELECT split_part(coalesce(last_archived_wal, ''), '.', 1)::text FROM pg_stat_archiver" | tr -d '[:space:]')"
  if [ -n "${ARCHIVED}" ] && [ ! "${ARCHIVED}" \< "${REQUIRED_WAL}" ]; then
    break
  fi
  FAILED_COUNT="$(psql -tAc "SELECT failed_count FROM pg_stat_archiver" | tr -d '[:space:]')"
  if [ "${FAILED_COUNT:-0}" -gt 0 ]; then
    echo "[wal-g] WARNING: archiver reported ${FAILED_COUNT} failures — check archive_command" >&2
  fi
  sleep 2
done

if [ -z "${ARCHIVED}" ] || [ "${ARCHIVED}" \< "${REQUIRED_WAL}" ]; then
  echo "[wal-g] ERROR: segment ${REQUIRED_WAL} (required by this backup) is not archived; last archived is '${ARCHIVED:-none}'." >&2
  echo "[wal-g] The base backup exists but CANNOT be restored until it lands." >&2
  exit 1
fi
echo "[wal-g] archiver caught up: required ${REQUIRED_WAL}, last archived ${ARCHIVED}"

# Retention: keep 14 full backups. WAL needed by a retained backup is kept by
# `--confirm` delete semantics; older segments are pruned so the bucket does
# not grow forever.
RETAIN="${WALG_RETAIN_BACKUPS:-14}"
echo "[wal-g] pruning to the last ${RETAIN} base backups"
wal-g delete retain FULL "${RETAIN}" --confirm

echo "[wal-g] base backup complete"
wal-g backup-list
