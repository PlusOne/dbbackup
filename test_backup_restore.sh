#!/bin/bash
set -e

LOG="/var/lib/pgsql/dbbackup_test.log"

echo "=== Database Backup/Restore Test ===" | tee $LOG
echo "Started: $(date)" | tee -a $LOG
echo "" | tee -a $LOG

cd /root/dbbackup

# Step 1: Cluster Backup
echo "STEP 1: Creating cluster backup..." | tee -a $LOG
sudo -u postgres ./dbbackup backup cluster --backup-dir /var/lib/pgsql/db_backups 2>&1 | tee -a $LOG
BACKUP_FILE=$(ls -t /var/lib/pgsql/db_backups/cluster_*.tar.gz | head -1)
echo "Backup created: $BACKUP_FILE" | tee -a $LOG
echo "Backup size: $(ls -lh $BACKUP_FILE | awk '{print $5}')" | tee -a $LOG
echo "" | tee -a $LOG

# Step 2: Drop d7030 database to prepare for restore test
echo "STEP 2: Dropping d7030 database for clean restore test..." | tee -a $LOG
sudo -u postgres psql -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'd7030' AND pid <> pg_backend_pid();" 2>&1 | tee -a $LOG
sudo -u postgres psql -d postgres -c "DROP DATABASE IF EXISTS d7030;" 2>&1 | tee -a $LOG
echo "d7030 database dropped" | tee -a $LOG
echo "" | tee -a $LOG

# Step 3: Cluster Restore
echo "STEP 3: Restoring cluster from backup..." | tee -a $LOG
sudo -u postgres ./dbbackup restore cluster $BACKUP_FILE --backup-dir /var/lib/pgsql/db_backups 2>&1 | tee -a $LOG
echo "Restore completed" | tee -a $LOG
echo "" | tee -a $LOG

# Step 4: Verify restored data
echo "STEP 4: Verifying restored databases..." | tee -a $LOG
sudo -u postgres psql -d postgres -c "\l" 2>&1 | tee -a $LOG
echo "" | tee -a $LOG
echo "Checking d7030 large objects..." | tee -a $LOG
BLOB_COUNT=$(sudo -u postgres psql -d d7030 -t -c "SELECT count(*) FROM pg_largeobject_metadata;" 2>/dev/null || echo "0")
echo "Large objects in d7030: $BLOB_COUNT" | tee -a $LOG
echo "" | tee -a $LOG

# Step 5: Cleanup
echo "STEP 5: Cleaning up test backup..." | tee -a $LOG
rm -f $BACKUP_FILE
echo "Backup file deleted: $BACKUP_FILE" | tee -a $LOG
echo "" | tee -a $LOG

echo "=== TEST COMPLETE ===" | tee -a $LOG
echo "Finished: $(date)" | tee -a $LOG
echo "" | tee -a $LOG
echo "✅ Full test log available at: $LOG"
