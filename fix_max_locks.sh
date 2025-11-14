#!/usr/bin/env bash
# fix_max_locks.sh
# Safely update max_locks_per_transaction in postgresql.conf and restart PostgreSQL
# Usage: sudo ./fix_max_locks.sh [NEW_VALUE]

set -euo pipefail
NEW_VALUE=${1:-256}

CONFIG_FILE="$(sudo -u postgres psql -t -c "SHOW config_file;" | tr -d '[:space:]')"
if [ -z "$CONFIG_FILE" ]; then
  echo "Could not locate postgresql.conf; aborting"
  exit 1
fi

echo "postgres config file: $CONFIG_FILE"
BACKUP_FILE="${CONFIG_FILE}.bak.$(date +%s)"

# Create a backup
sudo cp "$CONFIG_FILE" "$BACKUP_FILE"
sudo chown postgres:postgres "$BACKUP_FILE"
chmod 600 "$BACKUP_FILE"

echo "Backup written to $BACKUP_FILE"

# Use sed to update (or add) setting
# If the setting is present (commented or uncommented), replace the line. Otherwise append.
if sudo grep -q "^\s*max_locks_per_transaction\s*=\s*" "$CONFIG_FILE"; then
  echo "Updating existing max_locks_per_transaction to $NEW_VALUE"
  sudo sed -ri "s#^\s*#?max_locks_per_transaction\s*=.*#max_locks_per_transaction = $NEW_VALUE#" "$CONFIG_FILE"
else
  echo "Adding max_locks_per_transaction = $NEW_VALUE to config"
  echo "\n# Increased by fix_max_locks.sh on $(date)\nmax_locks_per_transaction = $NEW_VALUE" | sudo tee -a "$CONFIG_FILE" >/dev/null
fi

# Ensure correct permissions
sudo chown postgres:postgres "$CONFIG_FILE"
sudo chmod 600 "$CONFIG_FILE"

# Restart PostgreSQL and verify
echo "Restarting PostgreSQL service..."
sudo systemctl restart postgresql
sleep 2

echo "Verifying new value:"
sudo -u postgres psql -c "SHOW max_locks_per_transaction;"

echo "If the restart fails, you can restore the previous config with:\n  sudo cp $BACKUP_FILE $CONFIG_FILE && sudo systemctl restart postgresql"

exit 0
