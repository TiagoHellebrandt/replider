#!/bin/sh
# =============================================================================
# entrypoint.prod.sh — Production startup for the Replider API
#
# 1. Runs golang-migrate to apply pending database migrations.
# 2. On ANY migration failure (including a dirty state) prints the error and
#    the current version LOUDLY and exits non-zero. It never `force`s.
# 3. Exec's the API binary so signals (SIGTERM, SIGINT) are forwarded correctly
#    — Docker stops the container gracefully (stop_grace_period in compose).
#
# Why no auto-recovery: until 2026-09-01 a dirty state was "fixed" here by
# forcing the version back to N-1 and re-running `up` on every boot. A
# migration that genuinely fails (bad SQL, a constraint the data violates)
# then failed again on the next boot, was forced back again, and the
# container sat in a restart loop that looked like a crash — the real error
# scrolled past once and the forced version hid what had half-applied.
# The operator fixes FORWARD instead: read the error below, repair the
# schema or the migration, then `migrate force <version>` BY HAND with the
# state in front of them (docs/runbook.md, "Handle Migration Failures").
# =============================================================================
set -e

# URL-encode a string so special characters (e.g. #, @, /) in passwords don't
# break the postgres:// connection string passed to golang-migrate.
urlencode() {
    printf '%s' "$1" | awk '
    {
        len = length($0)
        for (i = 1; i <= len; i++) {
            c = substr($0, i, 1)
            if (c ~ /[A-Za-z0-9._~-]/) {
                printf "%s", c
            } else {
                printf "%%%02X", ord(c)
            }
        }
    }
    function ord(c,    i) {
        for (i = 0; i < 256; i++) if (sprintf("%c", i) == c) return i
    }'
}

DB_URL="postgres://$(urlencode "${DB_USER}"):$(urlencode "${DB_PASSWORD}")@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}"
MIGRATIONS_DIR=/app/db/migrations

log() { echo "[entrypoint] $*"; }

# migration_failed prints everything an operator needs, then exits 1. The
# restart policy will bring the container back and it will fail here again,
# on purpose: a schema that did not apply is not a schema to serve traffic on.
migration_failed() {
    echo "=================================================================" >&2
    echo "[entrypoint] MIGRATION FAILED — refusing to start the API." >&2
    echo "[entrypoint] Reason: $1" >&2
    echo "[entrypoint] Current schema_migrations state: ${VERSION_OUTPUT}" >&2
    echo "[entrypoint] Nothing was forced. Fix FORWARD:" >&2
    echo "[entrypoint]   1. read the migrate error above (the SQL that failed);" >&2
    echo "[entrypoint]   2. repair the schema/data or the migration file;" >&2
    echo "[entrypoint]   3. if the state is dirty, clear it by hand ONLY after" >&2
    echo "[entrypoint]      checking what half-applied:" >&2
    echo "[entrypoint]        migrate -path ${MIGRATIONS_DIR} -database \"\$DB_URL\" force <version>" >&2
    echo "[entrypoint]   4. redeploy. See docs/runbook.md → 'Handle Migration Failures'." >&2
    echo "=================================================================" >&2
    exit 1
}

log "Running database migrations..."

VERSION_OUTPUT=$(migrate -path "${MIGRATIONS_DIR}" -database "${DB_URL}" version 2>&1 || true)
log "Current migration state: ${VERSION_OUTPUT}"

if echo "${VERSION_OUTPUT}" | grep -q "dirty"; then
    migration_failed "schema_migrations is DIRTY (a previous migration failed mid-way)"
fi

# `set -e` would exit silently on a failed `up`; capture the status so the
# diagnostics above are printed first.
set +e
migrate -path "${MIGRATIONS_DIR}" -database "${DB_URL}" up
MIGRATE_STATUS=$?
set -e
if [ "${MIGRATE_STATUS}" -ne 0 ]; then
    VERSION_OUTPUT=$(migrate -path "${MIGRATIONS_DIR}" -database "${DB_URL}" version 2>&1 || true)
    migration_failed "migrate up exited with status ${MIGRATE_STATUS}"
fi
log "Migrations complete."

log "Starting Replider API on port ${PORT:-8080}..."
exec /app/replider-api
