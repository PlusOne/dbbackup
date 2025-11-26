# Point-in-Time Recovery (PITR) Guide

Complete guide to Point-in-Time Recovery in dbbackup v3.1.

## Table of Contents

- [Overview](#overview)
- [How PITR Works](#how-pitr-works)
- [Setup Instructions](#setup-instructions)
- [Recovery Operations](#recovery-operations)
- [Advanced Features](#advanced-features)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## Overview

Point-in-Time Recovery (PITR) allows you to restore your PostgreSQL database to any specific moment in time, not just to the time of your last backup. This is crucial for:

- **Disaster Recovery**: Recover from accidental data deletion, corruption, or malicious changes
- **Compliance**: Meet regulatory requirements for data retention and recovery
- **Testing**: Create snapshots at specific points for testing or analysis
- **Time Travel**: Investigate database state at any historical moment

### Use Cases

1. **Accidental DELETE**: User accidentally deletes important data at 2:00 PM. Restore to 1:59 PM.
2. **Bad Migration**: Deploy breaks production at 3:00 PM. Restore to 2:55 PM (before deploy).
3. **Audit Investigation**: Need to see exact database state on Nov 15 at 10:30 AM.
4. **Testing Scenarios**: Create multiple recovery branches to test different outcomes.

## How PITR Works

PITR combines three components:

### 1. Base Backup
A full snapshot of your database at a specific point in time.

```bash
# Take a base backup
pg_basebackup -D /backups/base.tar.gz -Ft -z -P
```

### 2. WAL Archives
PostgreSQL's Write-Ahead Log (WAL) files contain all database changes. These are continuously archived.

```
Base Backup (9 AM)  →  WAL Files (9 AM - 5 PM)  →  Current State
        ↓                        ↓
    Snapshot            All changes since backup
```

### 3. Recovery Target
The specific point in time you want to restore to. Can be:
- **Timestamp**: `2024-11-26 14:30:00`
- **Transaction ID**: `1000000`
- **LSN**: `0/3000000` (Log Sequence Number)
- **Named Point**: `before_migration`
- **Immediate**: Earliest consistent point

## Setup Instructions

### Prerequisites

- PostgreSQL 9.5+ (12+ recommended for modern recovery format)
- Sufficient disk space for WAL archives (~10-50 GB/day typical)
- dbbackup v3.1 or later

### Step 1: Enable WAL Archiving

```bash
# Configure PostgreSQL for PITR
./dbbackup pitr enable --archive-dir /backups/wal_archive

# This modifies postgresql.conf:
# wal_level = replica
# archive_mode = on
# archive_command = 'dbbackup wal archive %p %f --archive-dir /backups/wal_archive'
```

**Manual Configuration** (alternative):

Edit `/etc/postgresql/14/main/postgresql.conf`:

```ini
# WAL archiving for PITR
wal_level = replica              # Minimum required for PITR
archive_mode = on                # Enable WAL archiving
archive_command = '/usr/local/bin/dbbackup wal archive %p %f --archive-dir /backups/wal_archive'
max_wal_senders = 3              # For replication (optional)
wal_keep_size = 1GB              # Retain WAL on server (optional)
```

**Restart PostgreSQL:**

```bash
# Restart to apply changes
sudo systemctl restart postgresql

# Verify configuration
./dbbackup pitr status
```

### Step 2: Take a Base Backup

```bash
# Option 1: pg_basebackup (recommended)
pg_basebackup -D /backups/base_$(date +%Y%m%d_%H%M%S).tar.gz -Ft -z -P

# Option 2: Regular pg_dump backup
./dbbackup backup single mydb --output /backups/base.dump.gz

# Option 3: File-level copy (PostgreSQL stopped)
sudo service postgresql stop
tar -czf /backups/base.tar.gz -C /var/lib/postgresql/14/main .
sudo service postgresql start
```

### Step 3: Verify WAL Archiving

```bash
# Check that WAL files are being archived
./dbbackup wal list --archive-dir /backups/wal_archive

# Expected output:
# 000000010000000000000001  Timeline 1  Segment 0x00000001  16 MB  2024-11-26 09:00
# 000000010000000000000002  Timeline 1  Segment 0x00000002  16 MB  2024-11-26 09:15
# 000000010000000000000003  Timeline 1  Segment 0x00000003  16 MB  2024-11-26 09:30

# Check archive statistics
./dbbackup pitr status
```

### Step 4: Create Restore Points (Optional)

```sql
-- Create named restore points before major operations
SELECT pg_create_restore_point('before_schema_migration');
SELECT pg_create_restore_point('before_data_import');
SELECT pg_create_restore_point('end_of_day_2024_11_26');
```

## Recovery Operations

### Basic Recovery

**Restore to Specific Time:**

```bash
./dbbackup restore pitr \
  --base-backup /backups/base_20241126_090000.tar.gz \
  --wal-archive /backups/wal_archive \
  --target-time "2024-11-26 14:30:00" \
  --target-dir /var/lib/postgresql/14/restored
```

**What happens:**
1. Extracts base backup to target directory
2. Creates recovery configuration (postgresql.auto.conf + recovery.signal)
3. Provides instructions to start PostgreSQL
4. PostgreSQL replays WAL files until target time reached
5. Automatically promotes to primary (default action)

### Recovery Target Types

**1. Timestamp Recovery**
```bash
--target-time "2024-11-26 14:30:00"
--target-time "2024-11-26T14:30:00Z"  # ISO 8601
--target-time "2024-11-26 14:30:00.123456"  # Microseconds
```

**2. Transaction ID (XID) Recovery**
```bash
# Find XID from logs or pg_stat_activity
--target-xid 1000000

# Use case: Rollback specific transaction
# Check transaction ID: SELECT txid_current();
```

**3. LSN (Log Sequence Number) Recovery**
```bash
--target-lsn "0/3000000"

# Find LSN: SELECT pg_current_wal_lsn();
# Use case: Precise replication catchup
```

**4. Named Restore Point**
```bash
--target-name before_migration

# Use case: Restore to pre-defined checkpoint
```

**5. Immediate (Earliest Consistent)**
```bash
--target-immediate

# Use case: Restore to end of base backup
```

### Recovery Actions

Control what happens after recovery target is reached:

**1. Promote (default)**
```bash
--target-action promote

# PostgreSQL becomes primary, accepts writes
# Use case: Normal disaster recovery
```

**2. Pause**
```bash
--target-action pause

# PostgreSQL pauses at target, read-only
# Inspect data before committing
# Manually promote: pg_ctl promote -D /path
```

**3. Shutdown**
```bash
--target-action shutdown

# PostgreSQL shuts down at target
# Use case: Take filesystem snapshot
```

### Advanced Recovery Options

**Skip Base Backup Extraction:**
```bash
# If data directory already exists
./dbbackup restore pitr \
  --base-backup /backups/base.tar.gz \
  --wal-archive /backups/wal_archive \
  --target-time "2024-11-26 14:30:00" \
  --target-dir /var/lib/postgresql/14/main \
  --skip-extraction
```

**Auto-Start PostgreSQL:**
```bash
# Automatically start PostgreSQL after setup
./dbbackup restore pitr \
  --base-backup /backups/base.tar.gz \
  --wal-archive /backups/wal_archive \
  --target-time "2024-11-26 14:30:00" \
  --target-dir /var/lib/postgresql/14/restored \
  --auto-start
```

**Monitor Recovery Progress:**
```bash
# Monitor recovery in real-time
./dbbackup restore pitr \
  --base-backup /backups/base.tar.gz \
  --wal-archive /backups/wal_archive \
  --target-time "2024-11-26 14:30:00" \
  --target-dir /var/lib/postgresql/14/restored \
  --auto-start \
  --monitor

# Or manually monitor logs:
tail -f /var/lib/postgresql/14/restored/logfile
```

**Non-Inclusive Recovery:**
```bash
# Exclude target transaction/time
./dbbackup restore pitr \
  --base-backup /backups/base.tar.gz \
  --wal-archive /backups/wal_archive \
  --target-time "2024-11-26 14:30:00" \
  --target-dir /var/lib/postgresql/14/restored \
  --inclusive=false
```

**Timeline Selection:**
```bash
# Recover along specific timeline
--timeline 2

# Recover along latest timeline (default)
--timeline latest

# View available timelines:
./dbbackup wal timeline --archive-dir /backups/wal_archive
```

## Advanced Features

### WAL Compression

Save 70-80% storage space:

```bash
# Enable compression in archive_command
archive_command = 'dbbackup wal archive %p %f --archive-dir /backups/wal_archive --compress'

# Or compress during manual archive:
./dbbackup wal archive /path/to/wal/file %f \
  --archive-dir /backups/wal_archive \
  --compress
```

### WAL Encryption

Encrypt WAL files for compliance:

```bash
# Generate encryption key
openssl rand -hex 32 > /secure/wal_encryption.key

# Enable encryption in archive_command
archive_command = 'dbbackup wal archive %p %f --archive-dir /backups/wal_archive --encrypt --encryption-key-file /secure/wal_encryption.key'

# Or encrypt during manual archive:
./dbbackup wal archive /path/to/wal/file %f \
  --archive-dir /backups/wal_archive \
  --encrypt \
  --encryption-key-file /secure/wal_encryption.key
```

### Timeline Management

PostgreSQL creates a new timeline each time you perform PITR. This allows parallel recovery paths.

**View Timeline History:**
```bash
./dbbackup wal timeline --archive-dir /backups/wal_archive

# Output:
# Timeline Branching Structure:
# ● Timeline 1
#    WAL segments: 100 files
#   ├─ Timeline 2 (switched at 0/3000000)
#      WAL segments: 50 files
#     ├─ Timeline 3 [CURRENT] (switched at 0/5000000)
#        WAL segments: 25 files
```

**Recover to Specific Timeline:**
```bash
# Recover to timeline 2 instead of latest
./dbbackup restore pitr \
  --base-backup /backups/base.tar.gz \
  --wal-archive /backups/wal_archive \
  --target-time "2024-11-26 14:30:00" \
  --target-dir /var/lib/postgresql/14/restored \
  --timeline 2
```

### WAL Cleanup

Manage WAL archive growth:

```bash
# Clean up WAL files older than 7 days
./dbbackup wal cleanup \
  --archive-dir /backups/wal_archive \
  --retention-days 7

# Dry run (preview what would be deleted)
./dbbackup wal cleanup \
  --archive-dir /backups/wal_archive \
  --retention-days 7 \
  --dry-run
```

## Troubleshooting

### Common Issues

**1. WAL Archiving Not Working**

```bash
# Check PITR status
./dbbackup pitr status

# Verify PostgreSQL configuration
psql -c "SHOW archive_mode;"
psql -c "SHOW wal_level;"
psql -c "SHOW archive_command;"

# Check PostgreSQL logs
tail -f /var/log/postgresql/postgresql-14-main.log | grep archive

# Test archive command manually
su - postgres -c "dbbackup wal archive /test/path test_file --archive-dir /backups/wal_archive"
```

**2. Recovery Target Not Reached**

```bash
# Check if required WAL files exist
./dbbackup wal list --archive-dir /backups/wal_archive | grep "2024-11-26"

# Verify timeline consistency
./dbbackup wal timeline --archive-dir /backups/wal_archive

# Review recovery logs
tail -f /var/lib/postgresql/14/restored/logfile
```

**3. Permission Errors**

```bash
# Fix data directory ownership
sudo chown -R postgres:postgres /var/lib/postgresql/14/restored

# Fix WAL archive permissions
sudo chown -R postgres:postgres /backups/wal_archive
sudo chmod 700 /backups/wal_archive
```

**4. Disk Space Issues**

```bash
# Check WAL archive size
du -sh /backups/wal_archive

# Enable compression to save space
# Add --compress to archive_command

# Clean up old WAL files
./dbbackup wal cleanup --archive-dir /backups/wal_archive --retention-days 7
```

**5. PostgreSQL Won't Start After Recovery**

```bash
# Check PostgreSQL logs
tail -50 /var/lib/postgresql/14/restored/logfile

# Verify recovery configuration
cat /var/lib/postgresql/14/restored/postgresql.auto.conf
ls -la /var/lib/postgresql/14/restored/recovery.signal

# Check permissions
ls -ld /var/lib/postgresql/14/restored
```

### Debugging Tips

**Enable Verbose Logging:**
```bash
# Add to postgresql.conf
log_min_messages = debug2
log_error_verbosity = verbose
log_statement = 'all'
```

**Check WAL File Integrity:**
```bash
# Verify compressed WAL
gunzip -t /backups/wal_archive/000000010000000000000001.gz

# Verify encrypted WAL
./dbbackup wal verify /backups/wal_archive/000000010000000000000001.enc \
  --encryption-key-file /secure/key.bin
```

**Monitor Recovery Progress:**
```sql
-- In PostgreSQL during recovery
SELECT * FROM pg_stat_recovery_prefetch;
SELECT pg_is_in_recovery();
SELECT pg_last_wal_replay_lsn();
```

## Best Practices

### 1. Regular Base Backups

```bash
# Schedule daily base backups
0 2 * * * /usr/local/bin/pg_basebackup -D /backups/base_$(date +\%Y\%m\%d).tar.gz -Ft -z
```

**Why**: Limits WAL archive size, faster recovery.

### 2. Monitor WAL Archive Growth

```bash
# Add monitoring
du -sh /backups/wal_archive | mail -s "WAL Archive Size" admin@example.com

# Alert on >100 GB
if [ $(du -s /backups/wal_archive | cut -f1) -gt 100000000 ]; then
  echo "WAL archive exceeds 100 GB" | mail -s "ALERT" admin@example.com
fi
```

### 3. Test Recovery Regularly

```bash
# Monthly recovery test
./dbbackup restore pitr \
  --base-backup /backups/base_latest.tar.gz \
  --wal-archive /backups/wal_archive \
  --target-immediate \
  --target-dir /tmp/recovery_test \
  --auto-start

# Verify database accessible
psql -h localhost -p 5433 -d postgres -c "SELECT version();"

# Cleanup
pg_ctl stop -D /tmp/recovery_test
rm -rf /tmp/recovery_test
```

### 4. Document Restore Points

```bash
# Create log of restore points
echo "$(date '+%Y-%m-%d %H:%M:%S') - before_migration - Schema version 2.5 to 3.0" >> /backups/restore_points.log

# In PostgreSQL
SELECT pg_create_restore_point('before_migration');
```

### 5. Compression & Encryption

```bash
# Always compress (70-80% savings)
--compress

# Encrypt for compliance
--encrypt --encryption-key-file /secure/key.bin

# Combined (compress first, then encrypt)
--compress --encrypt --encryption-key-file /secure/key.bin
```

### 6. Retention Policy

```bash
# Keep base backups: 30 days
# Keep WAL archives: 7 days (between base backups)

# Cleanup script
#!/bin/bash
find /backups/base_* -mtime +30 -delete
./dbbackup wal cleanup --archive-dir /backups/wal_archive --retention-days 7
```

### 7. Monitoring & Alerting

```bash
# Check WAL archiving status
psql -c "SELECT last_archived_wal, last_archived_time FROM pg_stat_archiver;"

# Alert if archiving fails
if psql -tAc "SELECT last_failed_wal FROM pg_stat_archiver WHERE last_failed_wal IS NOT NULL;"; then
  echo "WAL archiving failed" | mail -s "ALERT" admin@example.com
fi
```

### 8. Disaster Recovery Plan

Document your recovery procedure:

```markdown
## Disaster Recovery Steps

1. Stop application traffic
2. Identify recovery target (time/XID/LSN)
3. Prepare clean data directory
4. Run PITR restore:
   ./dbbackup restore pitr \
     --base-backup /backups/base_latest.tar.gz \
     --wal-archive /backups/wal_archive \
     --target-time "YYYY-MM-DD HH:MM:SS" \
     --target-dir /var/lib/postgresql/14/main
5. Start PostgreSQL
6. Verify data integrity
7. Update application configuration
8. Resume application traffic
9. Create new base backup
```

## Performance Considerations

### WAL Archive Size

- Typical: 16 MB per WAL file
- High-traffic database: 1-5 GB/hour
- Low-traffic database: 100-500 MB/day

### Recovery Time

- Base backup restoration: 5-30 minutes (depends on size)
- WAL replay: 10-100 MB/sec (depends on disk I/O)
- Total recovery time: backup size / disk speed + WAL replay time

### Compression Performance

- CPU overhead: 5-10%
- Storage savings: 70-80%
- Recommended: Use unless CPU constrained

### Encryption Performance

- CPU overhead: 2-5%
- Storage overhead: ~1% (header + nonce)
- Recommended: Use for compliance

## Compliance & Security

### Regulatory Requirements

PITR helps meet:
- **GDPR**: Data recovery within 72 hours
- **SOC 2**: Backup and recovery procedures
- **HIPAA**: Data integrity and availability
- **PCI DSS**: Backup retention and testing

### Security Best Practices

1. **Encrypt WAL archives** containing sensitive data
2. **Secure encryption keys** (HSM, KMS, or secure filesystem)
3. **Limit access** to WAL archive directory (chmod 700)
4. **Audit logs** for recovery operations
5. **Test recovery** from encrypted backups regularly

## Additional Resources

- PostgreSQL PITR Documentation: https://www.postgresql.org/docs/current/continuous-archiving.html
- dbbackup GitHub: https://github.com/uuxo/dbbackup
- Report Issues: https://github.com/uuxo/dbbackup/issues

---

**dbbackup v3.1** | Point-in-Time Recovery for PostgreSQL
