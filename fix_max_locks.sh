#!/usr/bin/env bash
# fix_max_locks.sh
# Safely update max_locks_per_transaction in postgresql.conf and restart PostgreSQL
# Usage: sudo ./fix_max_locks.sh [NEW_VALUE]

set -euo pipefail
NEW_VALUE=${1:-256}

CONFIG_FILE="/var/lib/pgsql/data/postgresql.conf"
BACKUP_FILE="${CONFIG_FILE}.bak.$(date +%s)"

echo "PostgreSQL config file: $CONFIG_FILE"

# Create a backup
sudo cp "$CONFIG_FILE" "$BACKUP_FILE"
echo "Backup written to $BACKUP_FILE"

# Check if setting exists (commented or not)
if sudo grep -qE "^\s*#?\s*max_locks_per_transaction\s*=" "$CONFIG_FILE"; then
  echo "Updating existing max_locks_per_transaction to $NEW_VALUE"
  # Replace the line (whether commented or not)
  sudo sed -i "s/^\s*#\?\s*max_locks_per_transaction\s*=.*/max_locks_per_transaction = $NEW_VALUE/" "$CONFIG_FILE"
else
  echo "Adding max_locks_per_transaction = $NEW_VALUE to config"
  # Append at the end
  echo "" | sudo tee -a "$CONFIG_FILE" >/dev/null
  echo "# Increased by fix_max_locks.sh on $(date)" | sudo tee -a "$CONFIG_FILE" >/dev/null
  echo "max_locks_per_transaction = $NEW_VALUE" | sudo tee -a "$CONFIG_FILE" >/dev/null
fi

# Ensure correct permissions
sudo chown postgres:postgres "$CONFIG_FILE"
sudo chmod 600 "$CONFIG_FILE"

# Test the config before restarting
echo "Testing PostgreSQL config..."
sudo -u postgres /usr/bin/postgres -D /var/lib/pgsql/data -C max_locks_per_transaction 2>&1 | head -5

# Restart PostgreSQL and verify
echo "Restarting PostgreSQL service..."
sudo systemctl restart postgresql
sleep 3

if sudo systemctl is-active --quiet postgresql; then
  echo "✅ PostgreSQL restarted successfully"
  sudo -u postgres psql -c "SHOW max_locks_per_transaction;"
else
  echo "❌ PostgreSQL failed to start!"
  echo "Restoring backup..."
  sudo cp "$BACKUP_FILE" "$CONFIG_FILE"
  sudo systemctl start postgresql
  echo "Original config restored. Check /var/log/postgresql for errors."
  exit 1
fi

echo ""
echo "Success! Backup available at: $BACKUP_FILE"
exit 0
