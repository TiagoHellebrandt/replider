#!/bin/sh
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

# Exported so Air's pre_cmd (see .air.toml) can re-run `migrate up` on
# each rebuild without duplicating the URL-encoding logic.
export DB_URL="postgres://$(urlencode "${DB_USER}"):$(urlencode "${DB_PASSWORD}")@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}"

echo "Running database migrations..."

# Auto-recover from dirty migration state
VERSION_OUTPUT=$(migrate -path /app/db/migrations -database "${DB_URL}" version 2>&1 || true)
echo "Current migration state: ${VERSION_OUTPUT}"

if echo "${VERSION_OUTPUT}" | grep -q "dirty"; then
    DIRTY_VERSION=$(echo "${VERSION_OUTPUT}" | grep -oE '[0-9]+' | head -1)
    PREV_VERSION=$((DIRTY_VERSION - 1))
    echo "Dirty state detected at version ${DIRTY_VERSION}. Forcing back to ${PREV_VERSION}..."
    migrate -path /app/db/migrations -database "${DB_URL}" force "${PREV_VERSION}"
fi

# Detect stale volume: DB version ahead of latest migration file → force to latest
CURRENT_VERSION=$(echo "${VERSION_OUTPUT}" | grep -oE '^[0-9]+' | head -1 || echo "0")
LATEST_VERSION=$(ls /app/db/migrations/*.up.sql 2>/dev/null | grep -oE '[0-9]+' | sort -n | tail -1 || echo "0")
if [ -n "${CURRENT_VERSION}" ] && [ "${CURRENT_VERSION}" -gt "${LATEST_VERSION}" ] 2>/dev/null; then
    echo "DB version (${CURRENT_VERSION}) is ahead of available migrations (${LATEST_VERSION}). Forcing to ${LATEST_VERSION}..."
    migrate -path /app/db/migrations -database "${DB_URL}" force "${LATEST_VERSION}"
fi

migrate -path /app/db/migrations -database "${DB_URL}" up
echo "Migrations done."

# Start ngrok tunnel if NGROK_AUTHTOKEN is set, then update APP_BASE_URL
if [ -n "${NGROK_AUTHTOKEN}" ]; then
    echo "Starting ngrok tunnel on port ${PORT:-8080}..."
    ngrok config add-authtoken "${NGROK_AUTHTOKEN}" > /dev/null 2>&1
    ngrok http "${PORT:-8080}" --log=stdout > /tmp/ngrok.log 2>&1 &

    # Wait for ngrok to be ready (up to 10s)
    NGROK_URL=""
    for i in $(seq 1 20); do
        sleep 0.5
        NGROK_URL=$(wget -qO- http://localhost:4040/api/tunnels 2>/dev/null \
            | grep -o '"public_url":"https://[^"]*"' \
            | head -1 \
            | grep -o 'https://[^"]*' || true)
        if [ -n "${NGROK_URL}" ]; then
            break
        fi
    done

    if [ -n "${NGROK_URL}" ]; then
        echo "ngrok tunnel active: ${NGROK_URL}"
        export APP_BASE_URL="${NGROK_URL}"
    else
        echo "WARNING: ngrok tunnel did not start in time. APP_BASE_URL remains: ${APP_BASE_URL}"
    fi
else
    echo "NGROK_AUTHTOKEN not set — skipping ngrok (using APP_BASE_URL=${APP_BASE_URL})"
fi

exec air -c .air.toml
