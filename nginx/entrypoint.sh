#!/bin/sh
# ─────────────────────────────────────────────────────────────────────────────
# nginx entrypoint wrapper
#
# Reloads nginx every 6 h so any cert updates are picked up
# without requiring a container restart.
# ─────────────────────────────────────────────────────────────────────────────
set -e

(
    while :; do
        sleep 6h
        echo "[nginx-reload] Reloading nginx to pick up any certificate updates..."
        nginx -s reload 2>/dev/null || true
    done
) &

exec /docker-entrypoint.sh "$@"
