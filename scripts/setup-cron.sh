#!/bin/sh
# Sets up automated daily database backup via cron.
# Run once on VPS setup: ./setup-cron.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKUP_SCRIPT="${SCRIPT_DIR}/backup-db.sh"

if ! crontab -l 2>/dev/null | grep -q "backup-db.sh"; then
  (crontab -l 2>/dev/null; echo "0 3 * * * ${BACKUP_SCRIPT} >> /var/log/replider-backup.log 2>&1") | crontab -
  echo "[cron] Added daily backup at 03:00 AM"
else
  echo "[cron] Backup cron already exists"
fi

echo "[cron] Current crontab:"
crontab -l
