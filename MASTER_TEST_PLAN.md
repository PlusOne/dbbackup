# Master Test Plan - dbbackup v1.2.0
## Comprehensive Command-Line and Interactive Testing

---

## Test Environment Setup

### Prerequisites
```bash
# 1. Ensure PostgreSQL is running
systemctl status postgresql

# 2. Create test databases with varied characteristics
psql -U postgres <<EOF
CREATE DATABASE test_small;        -- ~10MB
CREATE DATABASE test_medium;       -- ~100MB
CREATE DATABASE test_large;        -- ~1GB
CREATE DATABASE test_empty;        -- Empty database
EOF

# 3. Setup test directories
export TEST_BACKUP_DIR="/tmp/dbbackup_master_test_$(date +%s)"
mkdir -p $TEST_BACKUP_DIR

# 4. Verify tools
which pg_dump pg_restore pg_dumpall pigz gzip tar
```

---

## PART 1: Command-Line Flag Testing

### 1.1 Global Flags (Apply to All Commands)

| Flag | Test Command | Expected Result | Verification |
|------|-------------|-----------------|--------------|
| `--help` | `./dbbackup --help` | Show main help | Check output contains "Usage:" |
| `--version` | `./dbbackup --version` | Show version | Check version string |
| `--debug` | `./dbbackup --debug status cpu` | Enable debug logs | Check for DEBUG lines in output |
| `--no-color` | `./dbbackup --no-color status cpu` | Disable ANSI colors | No escape sequences in output |
| `--backup-dir <path>` | `./dbbackup backup single postgres --backup-dir /tmp/test` | Use custom dir | Check file created in /tmp/test |
| `--compression 0` | `./dbbackup backup single postgres --compression 0` | No compression | Check file size vs compressed |
| `--compression 1` | `./dbbackup backup single postgres --compression 1` | Low compression | File size check |
| `--compression 6` | `./dbbackup backup single postgres --compression 6` | Default compression | File size check |
| `--compression 9` | `./dbbackup backup single postgres --compression 9` | Max compression | Smallest file size |
| `--jobs 1` | `./dbbackup backup cluster --jobs 1` | Single threaded | Slower execution |
| `--jobs 8` | `./dbbackup backup cluster --jobs 8` | 8 parallel jobs | Faster execution |
| `--dump-jobs 4` | `./dbbackup backup single postgres --dump-jobs 4` | 4 dump threads | Check pg_dump --jobs |
| `--auto-detect-cores` | `./dbbackup backup cluster --auto-detect-cores` | Auto CPU detection | Default behavior |
| `--max-cores 4` | `./dbbackup backup cluster --max-cores 4` | Limit to 4 cores | Check CPU usage |
| `--cpu-workload cpu-intensive` | `./dbbackup backup cluster --cpu-workload cpu-intensive` | Adjust for CPU work | Performance profile |
| `--cpu-workload io-intensive` | `./dbbackup backup cluster --cpu-workload io-intensive` | Adjust for I/O work | Performance profile |
| `--cpu-workload balanced` | `./dbbackup backup cluster --cpu-workload balanced` | Balanced profile | Default behavior |

### 1.2 Database Connection Flags

| Flag | Test Command | Expected Result | Verification |
|------|-------------|-----------------|--------------|
| `-d postgres` / `--db-type postgres` | `./dbbackup status host -d postgres` | Connect to PostgreSQL | Success |
| `-d mysql` | `./dbbackup status host -d mysql` | Connect to MySQL | Success (if MySQL available) |
| `--host localhost` | `./dbbackup status host --host localhost` | Local connection | Success |
| `--host 127.0.0.1` | `./dbbackup status host --host 127.0.0.1` | TCP connection | Success |
| `--port 5432` | `./dbbackup status host --port 5432` | Default port | Success |
| `--port 5433` | `./dbbackup status host --port 5433` | Custom port | Connection error expected |
| `--user postgres` | `./dbbackup status host --user postgres` | Custom user | Success |
| `--user invalid_user` | `./dbbackup status host --user invalid_user` | Invalid user | Auth failure expected |
| `--password <pwd>` | `./dbbackup status host --password secretpass` | Password auth | Success |
| `--database postgres` | `./dbbackup status host --database postgres` | Connect to DB | Success |
| `--insecure` | `./dbbackup status host --insecure` | Disable SSL | Success |
| `--ssl-mode disable` | `./dbbackup status host --ssl-mode disable` | SSL disabled | Success |
| `--ssl-mode require` | `./dbbackup status host --ssl-mode require` | SSL required | Success/failure based on server |
| `--ssl-mode verify-ca` | `./dbbackup status host --ssl-mode verify-ca` | Verify CA cert | Success/failure based on certs |
| `--ssl-mode verify-full` | `./dbbackup status host --ssl-mode verify-full` | Full verification | Success/failure based on certs |

### 1.3 Backup Command Flags

#### 1.3.1 `backup single` Command
```bash
./dbbackup backup single [database] [flags]
```

| Flag | Test Command | Expected Result | Verification |
|------|-------------|-----------------|--------------|
| `<database>` (positional) | `./dbbackup backup single postgres` | Backup postgres DB | File created |
| `--database <name>` | `./dbbackup backup single --database postgres` | Same as positional | File created |
| `--compression 1` | `./dbbackup backup single postgres --compression 1` | Fast compression | Larger file |
| `--compression 9` | `./dbbackup backup single postgres --compression 9` | Best compression | Smaller file |
| No database specified | `./dbbackup backup single` | Error message | "database name required" |
| Invalid database | `./dbbackup backup single nonexistent_db` | Error message | "database does not exist" |
| Large database | `./dbbackup backup single test_large --compression 1` | Streaming compression | No huge temp files |
| Empty database | `./dbbackup backup single test_empty` | Small backup | File size ~KB |

#### 1.3.2 `backup cluster` Command
```bash
./dbbackup backup cluster [flags]
```

| Flag | Test Command | Expected Result | Verification |
|------|-------------|-----------------|--------------|
| No flags | `./dbbackup backup cluster` | Backup all DBs | cluster_*.tar.gz created |
| `--compression 1` | `./dbbackup backup cluster --compression 1` | Fast cluster backup | Larger archive |
| `--compression 9` | `./dbbackup backup cluster --compression 9` | Best compression | Smaller archive |
| `--jobs 1` | `./dbbackup backup cluster --jobs 1` | Sequential backup | Slower |
| `--jobs 8` | `./dbbackup backup cluster --jobs 8` | Parallel backup | Faster |
| Large DBs present | `./dbbackup backup cluster --compression 3` | Streaming compression | No huge temp files |
| Check globals backup | Extract and verify `globals.sql` | Roles/tablespaces | globals.sql present |
| Check all DBs backed up | Extract and count dumps | All non-template DBs | Verify count |

#### 1.3.3 `backup sample` Command
```bash
./dbbackup backup sample [database] [flags]
```

| Flag | Test Command | Expected Result | Verification |
|------|-------------|-----------------|--------------|
| Default sample | `./dbbackup backup sample test_large` | Sample backup | Smaller than full |
| Custom strategy | Check if sample flags exist | Sample strategy options | TBD based on implementation |

### 1.4 Restore Command Flags

#### 1.4.1 `restore single` Command
```bash
./dbbackup restore single [backup-file] [flags]
```

| Flag | Test Command | Expected Result | Verification |
|------|-------------|-----------------|--------------|
| `<backup-file>` (positional) | `./dbbackup restore single /path/to/backup.dump` | Restore to original DB | Success |
| `--target-db <name>` | `./dbbackup restore single backup.dump --target-db restored_db` | Restore to new DB | New DB created |
| `--create` | `./dbbackup restore single backup.dump --target-db newdb --create` | Create DB if missing | DB created + restored |
| `--no-owner` | `./dbbackup restore single backup.dump --no-owner` | Skip ownership | No SET OWNER commands |
| `--clean` | `./dbbackup restore single backup.dump --clean` | Drop existing objects | Clean restore |
| `--jobs 4` | `./dbbackup restore single backup.dump --jobs 4` | Parallel restore | Faster |
| Missing backup file | `./dbbackup restore single nonexistent.dump` | Error message | "file not found" |
| Invalid backup file | `./dbbackup restore single /etc/hosts` | Error message | "invalid backup file" |
| Without --create, DB missing | `./dbbackup restore single backup.dump --target-db missing_db` | Error message | "database does not exist" |
| With --create, DB missing | `./dbbackup restore single backup.dump --target-db missing_db --create` | Success | DB created |

#### 1.4.2 `restore cluster` Command
```bash
./dbbackup restore cluster [backup-file] [flags]
```

| Flag | Test Command | Expected Result | Verification |
|------|-------------|-----------------|--------------|
| `<backup-file>` (positional) | `./dbbackup restore cluster cluster_*.tar.gz` | Restore all DBs | All DBs restored |
| `--create` | `./dbbackup restore cluster backup.tar.gz --create` | Create missing DBs | DBs created |
| `--globals-only` | `./dbbackup restore cluster backup.tar.gz --globals-only` | Restore roles only | Only globals restored |
| `--jobs 4` | `./dbbackup restore cluster backup.tar.gz --jobs 4` | Parallel restore | Faster |
| Missing archive | `./dbbackup restore cluster nonexistent.tar.gz` | Error message | "file not found" |
| Invalid archive | `./dbbackup restore cluster /etc/hosts` | Error message | "invalid archive" |
| Corrupted archive | Create corrupted .tar.gz | Error message | "extraction failed" |
| Ownership preservation | Restore and check owners | Correct ownership | GRANT/REVOKE present |

### 1.5 Status Command Flags

#### 1.5.1 `status host` Command
```bash
./dbbackup status host [flags]
```

| Flag | Test Command | Expected Result | Verification |
|------|-------------|-----------------|--------------|
| No flags | `./dbbackup status host` | Show host status | Version, size, DBs listed |
| `--database <name>` | `./dbbackup status host --database postgres` | Connect to specific DB | Success |
| Invalid connection | `./dbbackup status host --port 9999` | Error message | Connection failure |
| Show version | Check output | PostgreSQL version | Version string present |
| List databases | Check output | Database list | All DBs shown |
| Show sizes | Check output | Database sizes | Sizes in human format |

#### 1.5.2 `status cpu` Command
```bash
./dbbackup status cpu [flags]
```

| Flag | Test Command | Expected Result | Verification |
|------|-------------|-----------------|--------------|
| No flags | `./dbbackup status cpu` | Show CPU info | Cores, workload shown |
| CPU detection | Check output | Auto-detected cores | Matches system |
| Workload info | Check output | Current workload | Workload type shown |

---

## PART 2: Interactive TUI Testing

### 2.1 TUI Launch and Navigation

| Test Case | Steps | Expected Result | Verification Method |
|-----------|-------|-----------------|---------------------|
| Launch TUI | `./dbbackup` | Main menu appears | Visual check |
| Navigate with arrows | Press ↑/↓ | Selection moves | Visual check |
| Navigate with j/k | Press j/k (vim keys) | Selection moves | Visual check |
| Navigate with numbers | Press 1-4 | Jump to option | Visual check |
| Press ESC | ESC key | Exit confirmation | Visual check |
| Press q | q key | Exit confirmation | Visual check |
| Press Ctrl+C | Ctrl+C | Immediate exit | Visual check |

### 2.2 Main Menu Options

```
Main Menu:
1. Single Database Backup
2. Cluster Backup (All Databases)
3. Restore Database
4. System Status
5. Settings
6. Exit
```

#### Test: Select Each Menu Option

| Menu Item | Key Press | Expected Result | Verification |
|-----------|-----------|-----------------|--------------|
| Single Database Backup | Enter on option 1 | Database selection screen | Visual check |
| Cluster Backup | Enter on option 2 | Cluster backup screen | Visual check |
| Restore Database | Enter on option 3 | Restore file selection | Visual check |
| System Status | Enter on option 4 | Status screen | Visual check |
| Settings | Enter on option 5 | Settings menu | Visual check |
| Exit | Enter on option 6 | Exit application | Application closes |

### 2.3 Single Database Backup Flow

**Entry**: Main Menu → Single Database Backup

| Step | Action | Expected Screen | Verification |
|------|--------|-----------------|--------------|
| 1. Database list appears | - | List of databases shown | Check postgres, template0, template1 excluded or marked |
| 2. Navigate databases | ↑/↓ arrows | Selection moves | Visual check |
| 3. Search databases | Type filter text | List filters | Only matching DBs shown |
| 4. Select database | Press Enter | Backup options screen | Options screen appears |
| 5. Compression level | Select 1-9 | Level selected | Selected level highlighted |
| 6. Backup directory | Enter or use default | Directory shown | Path displayed |
| 7. Start backup | Confirm | Progress indicator | Spinner/progress bar |
| 8. Backup completes | Wait | Success message | File path shown |
| 9. Return to menu | Press key | Back to main menu | Main menu shown |

**Error Scenarios**:
- No database selected → Error message
- Invalid backup directory → Error message
- Insufficient permissions → Error message
- Disk full → Error message with space info

### 2.4 Cluster Backup Flow

**Entry**: Main Menu → Cluster Backup

| Step | Action | Expected Screen | Verification |
|------|--------|-----------------|--------------|
| 1. Cluster options | - | Options screen | Compression, directory shown |
| 2. Set compression | Select level | Level selected | 1-9 |
| 3. Set directory | Enter path | Directory set | Path shown |
| 4. Start backup | Confirm | Progress screen | Per-DB progress |
| 5. Monitor progress | Watch | Database names + progress | Real-time updates |
| 6. Backup completes | Wait | Summary screen | Success/failed counts |
| 7. Review results | - | Archive info shown | Size, location, duration |
| 8. Return to menu | Press key | Main menu | Menu shown |

**Error Scenarios**:
- Connection lost mid-backup → Error message, partial cleanup
- Disk full during backup → Error message, cleanup temp files
- Individual DB failure → Continue with others, show warning

### 2.5 Restore Database Flow

**Entry**: Main Menu → Restore Database

| Step | Action | Expected Screen | Verification |
|------|--------|-----------------|--------------|
| 1. Restore type selection | - | Single or Cluster | Two options |
| 2. Select restore type | Enter on option | File browser | Backup files listed |
| 3. Browse backup files | Navigate | File list | .dump, .tar.gz files shown |
| 4. Filter files | Type filter | List filters | Matching files shown |
| 5. Select backup file | Enter | Restore options | Options screen |
| 6. Set target database | Enter name | DB name set | Name shown |
| 7. Set options | Toggle flags | Options selected | Checkboxes/toggles |
| 8. Confirm restore | Press Enter | Warning prompt | "This will modify database" |
| 9. Start restore | Confirm | Progress indicator | Spinner/progress bar |
| 10. Restore completes | Wait | Success message | Duration, objects restored |
| 11. Return to menu | Press key | Main menu | Menu shown |

**Critical Test**: Cluster Restore Selection
- **KNOWN BUG**: Enter key may not work on cluster backup selection
- Test: Navigate to cluster backup, press Enter
- Expected: File selected and restore proceeds
- Verify: Fix if Enter key doesn't register

**Options to Test**:
- `--create`: Create database if missing
- `--no-owner`: Skip ownership restoration
- `--clean`: Drop existing objects first
- Target database name field

### 2.6 System Status Flow

**Entry**: Main Menu → System Status

| Step | Action | Expected Screen | Verification |
|------|--------|-----------------|--------------|
| 1. Status options | - | Host or CPU status | Two tabs/options |
| 2. Host status | Select | Connection info + DBs | Database list with sizes |
| 3. Navigate DB list | ↑/↓ arrows | Selection moves | Visual check |
| 4. View DB details | Enter on DB | DB details | Tables, size, owner |
| 5. CPU status | Select | CPU info | Cores, workload type |
| 6. Return | ESC/Back | Main menu | Menu shown |

### 2.7 Settings Flow

**Entry**: Main Menu → Settings

| Step | Action | Expected Screen | Verification |
|------|--------|-----------------|--------------|
| 1. Settings menu | - | Options list | All settings shown |
| 2. Connection settings | Select | Host, port, user, etc. | Form fields |
| 3. Edit connection | Change values | Values update | Changes persist |
| 4. Test connection | Press test button | Connection result | Success/failure message |
| 5. Backup settings | Select | Compression, jobs, etc. | Options shown |
| 6. Change compression | Set level | Level updated | Value changes |
| 7. Change jobs | Set count | Count updated | Value changes |
| 8. Save settings | Confirm | Settings saved | "Saved" message |
| 9. Cancel changes | Cancel | Settings reverted | Original values |
| 10. Return | Back | Main menu | Menu shown |

**Settings to Test**:
- Database host (localhost, IP address)
- Database port (5432, custom)
- Database user (postgres, custom)
- Database password (set, change, clear)
- Database type (postgres, mysql)
- SSL mode (disable, require, verify-ca, verify-full)
- Backup directory (default, custom)
- Compression level (0-9)
- Parallel jobs (1-16)
- Dump jobs (1-16)
- Auto-detect cores (on/off)
- CPU workload type (balanced, cpu-intensive, io-intensive)

### 2.8 TUI Error Handling

| Error Scenario | Trigger | Expected Behavior | Verification |
|----------------|---------|-------------------|--------------|
| Database connection failure | Wrong credentials | Error modal with details | Clear error message |
| Backup file not found | Delete file mid-operation | Error message | Graceful handling |
| Invalid backup file | Select non-backup file | Error message | "Invalid backup format" |
| Insufficient permissions | Backup to read-only dir | Error message | Permission denied |
| Disk full | Fill disk during backup | Error message + cleanup | Temp files removed |
| Network interruption | Disconnect during remote backup | Error message | Connection lost message |
| Keyboard interrupt | Press Ctrl+C | Confirmation prompt | "Cancel operation?" |
| Window resize | Resize terminal | UI adapts | No crashes, redraws correctly |
| Invalid input | Enter invalid characters | Input rejected or sanitized | No crashes |
| Concurrent operations | Try two backups at once | Error or queue | "Operation in progress" |

### 2.9 TUI Visual/UX Tests

| Test | Action | Expected Result | Pass/Fail |
|------|--------|-----------------|-----------|
| Color theme | Launch TUI | Colors render correctly | Visual check |
| No-color mode | `--no-color` flag | Plain text only | No ANSI codes |
| Progress indicators | Start backup | Spinner/progress bar animates | Visual check |
| Help text | Press ? or h | Help overlay | Help displayed |
| Modal dialogs | Trigger error | Modal appears centered | Visual check |
| Modal close | ESC or Enter | Modal closes | Returns to previous screen |
| Text wrapping | Long database names | Text wraps or truncates | Readable |
| Scrolling | Long lists | List scrolls | Arrow keys work |
| Keyboard shortcuts | Press shortcuts | Actions trigger | Quick actions work |
| Mouse support | Click options (if supported) | Selection changes | Visual check |

---

## PART 3: Integration Testing

### 3.1 End-to-End Workflows

#### Workflow 1: Complete Backup and Restore Cycle
```bash
# 1. Create test database
psql -U postgres -c "CREATE DATABASE e2e_test_db;"
psql -U postgres e2e_test_db <<EOF
CREATE TABLE test_data (id SERIAL PRIMARY KEY, data TEXT);
INSERT INTO test_data (data) SELECT 'test_' || generate_series(1, 1000);
EOF

# 2. Backup via CLI
./dbbackup backup single e2e_test_db --backup-dir /tmp/e2e_test

# 3. Drop database
psql -U postgres -c "DROP DATABASE e2e_test_db;"

# 4. Restore via CLI
backup_file=$(ls -t /tmp/e2e_test/db_e2e_test_db_*.dump | head -1)
./dbbackup restore single "$backup_file" --target-db e2e_test_db --create

# 5. Verify data
count=$(psql -U postgres e2e_test_db -tAc "SELECT COUNT(*) FROM test_data;")
if [ "$count" = "1000" ]; then
    echo "✅ E2E Test PASSED"
else
    echo "❌ E2E Test FAILED: Expected 1000 rows, got $count"
fi

# 6. Cleanup
psql -U postgres -c "DROP DATABASE e2e_test_db;"
rm -rf /tmp/e2e_test
```

#### Workflow 2: Cluster Backup and Selective Restore
```bash
# 1. Backup entire cluster
./dbbackup backup cluster --backup-dir /tmp/cluster_test --compression 3

# 2. Extract and verify contents
cluster_file=$(ls -t /tmp/cluster_test/cluster_*.tar.gz | head -1)
tar -tzf "$cluster_file" | head -20

# 3. Verify globals.sql exists
tar -xzf "$cluster_file" -C /tmp/extract_test globals.sql

# 4. Count database dumps
dump_count=$(tar -tzf "$cluster_file" | grep "\.dump$" | wc -l)
echo "Found $dump_count database dumps"

# 5. Full cluster restore
./dbbackup restore cluster "$cluster_file" --create

# 6. Verify all databases restored
psql -U postgres -l

# 7. Cleanup
rm -rf /tmp/cluster_test /tmp/extract_test
```

#### Workflow 3: Large Database with Streaming Compression
```bash
# 1. Check if testdb_50gb exists
if psql -U postgres -lqt | cut -d \| -f 1 | grep -qw "testdb_50gb"; then
    echo "Testing with testdb_50gb"
    
    # 2. Backup with compression=1 (should use streaming)
    ./dbbackup backup single testdb_50gb --compression 1 --backup-dir /tmp/large_test
    
    # 3. Verify no huge uncompressed temp files were created
    find /var/lib/pgsql/db_backups -name "*.dump" -size +10G && echo "❌ FAILED: Large uncompressed file found" || echo "✅ PASSED: No large uncompressed files"
    
    # 4. Check backup file size (should be ~500-900MB compressed)
    backup_file=$(ls -t /tmp/large_test/db_testdb_50gb_*.dump | head -1)
    size=$(stat -c%s "$backup_file" 2>/dev/null || stat -f%z "$backup_file")
    size_mb=$((size / 1024 / 1024))
    echo "Backup size: ${size_mb}MB"
    
    if [ $size_mb -lt 2000 ]; then
        echo "✅ PASSED: Streaming compression worked"
    else
        echo "❌ FAILED: File too large, streaming compression may have failed"
    fi
    
    # 5. Cleanup
    rm -rf /tmp/large_test
else
    echo "⊘ SKIPPED: testdb_50gb not available"
fi
```

### 3.2 Permission and Authentication Tests

| Test | Setup | Command | Expected Result |
|------|-------|---------|-----------------|
| Peer authentication | Run as postgres user | `sudo -u postgres ./dbbackup status host` | Success |
| Password authentication | Set PGPASSWORD | `PGPASSWORD=xxx ./dbbackup status host --password xxx` | Success |
| .pgpass authentication | Create ~/.pgpass | `./dbbackup status host` | Success |
| Failed authentication | Wrong password | `./dbbackup status host --password wrong` | Auth failure |
| Insufficient privileges | Non-superuser restore | `./dbbackup restore cluster ...` | Error or warning |
| SSL connection | SSL enabled server | `./dbbackup status host --ssl-mode require` | Success |
| SSL required but unavailable | SSL disabled server | `./dbbackup status host --ssl-mode require` | Connection failure |

### 3.3 Error Recovery Tests

| Scenario | Trigger | Expected Behavior | Verification |
|----------|---------|-------------------|--------------|
| Interrupted backup | Kill process mid-backup | Temp files cleaned up | No leftover .cluster_* dirs |
| Interrupted restore | Kill process mid-restore | Partial objects, clear error | Database in consistent state |
| Out of disk space | Fill disk during backup | Error message, cleanup | Temp files removed |
| Out of memory | Very large database | Streaming compression used | No OOM kills |
| Database locked | Backup during heavy load | Backup waits or times out | Clear timeout message |
| Corrupted backup file | Manually corrupt file | Error during restore | "Invalid backup file" |
| Missing dependencies | Remove pg_dump | Error at startup | "Required tool not found" |
| Network timeout | Slow/interrupted connection | Timeout with retry option | Clear error message |

---

## PART 4: Performance and Stress Testing

### 4.1 Performance Benchmarks

#### Test: Compression Speed vs Size Trade-off
```bash
# Backup same database with different compression levels, measure time and size
for level in 1 3 6 9; do
    echo "Testing compression level $level"
    start=$(date +%s)
    ./dbbackup backup single postgres --compression $level --backup-dir /tmp/perf_test
    end=$(date +%s)
    duration=$((end - start))
    backup_file=$(ls -t /tmp/perf_test/db_postgres_*.dump | head -1)
    size=$(stat -c%s "$backup_file" 2>/dev/null || stat -f%z "$backup_file")
    size_mb=$((size / 1024 / 1024))
    echo "Level $level: ${duration}s, ${size_mb}MB"
    rm "$backup_file"
done
```

Expected results:
- Level 1: Fastest, largest file
- Level 9: Slowest, smallest file
- Level 6: Good balance

#### Test: Parallel vs Sequential Performance
```bash
# Cluster backup with different --jobs settings
for jobs in 1 4 8; do
    echo "Testing with $jobs parallel jobs"
    start=$(date +%s)
    ./dbbackup backup cluster --jobs $jobs --compression 3 --backup-dir /tmp/parallel_test
    end=$(date +%s)
    duration=$((end - start))
    echo "$jobs jobs: ${duration}s"
    rm /tmp/parallel_test/cluster_*.tar.gz
done
```

Expected results:
- 1 job: Slowest
- 8 jobs: Fastest (up to CPU core count)

### 4.2 Stress Tests

#### Test: Multiple Concurrent Operations
```bash
# Try to trigger race conditions
./dbbackup backup single postgres --backup-dir /tmp/stress1 &
./dbbackup backup single postgres --backup-dir /tmp/stress2 &
./dbbackup status host &
wait
# Verify: All operations should complete successfully without conflicts
```

#### Test: Very Large Database (if available)
- Use testdb_50gb or larger
- Verify streaming compression activates
- Monitor memory usage (should stay reasonable)
- Verify no disk space exhaustion

#### Test: Many Small Databases
```bash
# Create 50 small databases
for i in {1..50}; do
    psql -U postgres -c "CREATE DATABASE stress_test_$i;"
done

# Backup cluster
./dbbackup backup cluster --backup-dir /tmp/stress_cluster

# Verify: All 50+ databases backed up, archive created successfully

# Cleanup
for i in {1..50}; do
    psql -U postgres -c "DROP DATABASE stress_test_$i;"
done
```

---

## PART 5: Regression Testing

### 5.1 Known Issue Verification

| Issue | Test | Expected Behavior | Status |
|-------|------|-------------------|--------|
| TUI Enter key on cluster restore | Launch TUI, select cluster backup, press Enter | Backup selected, restore proceeds | ⚠️ Known issue - retest |
| Debug logging not working | Run with `--debug` | DEBUG lines in output | ⚠️ Known issue - retest |
| Streaming compression for large DBs | Backup testdb_50gb | No huge temp files | ✅ Fixed in v1.2.0 |
| Disk space exhaustion | Backup large DBs | Streaming compression prevents disk fill | ✅ Fixed in v1.2.0 |

### 5.2 Previous Bug Verification

Test all previously fixed bugs to ensure no regressions:

1. **Ownership preservation** (fixed earlier)
   - Backup database with custom owners
   - Restore to new cluster
   - Verify ownership preserved

2. **restore --create flag** (fixed earlier)
   - Restore to non-existent database with --create
   - Verify database created and populated

3. **Streaming compression** (fixed v1.2.0)
   - Backup large database
   - Verify no huge temp files
   - Verify compressed output

---

## PART 6: Cross-Platform Testing (if applicable)

Test on each supported platform:
- ✅ Linux (amd64) - Primary platform
- ⏹ Linux (arm64)
- ⏹ Linux (armv7)
- ⏹ macOS (Intel)
- ⏹ macOS (Apple Silicon)
- ⏹ FreeBSD
- ⏹ Windows (if PostgreSQL tools available)

For each platform:
1. Binary executes
2. Help/version commands work
3. Basic backup works
4. Basic restore works
5. TUI launches (if terminal supports it)

---

## PART 7: Automated Test Script

### Master Test Execution Script
```bash
#!/bin/bash
# Save as: run_master_tests.sh

source ./master_test_functions.sh  # Helper functions

echo "================================================"
echo "dbbackup Master Test Suite"
echo "================================================"
echo ""

# Initialize
init_test_environment

# PART 1: CLI Flags
echo "=== PART 1: Command-Line Flag Testing ==="
test_global_flags
test_connection_flags
test_backup_flags
test_restore_flags
test_status_flags

# PART 2: Interactive (manual)
echo "=== PART 2: Interactive TUI Testing ==="
echo "⚠️  This section requires manual testing"
echo "Launch: ./dbbackup"
echo "Follow test cases in MASTER_TEST_PLAN.md section 2"
echo ""
read -p "Press Enter after completing TUI tests..."

# PART 3: Integration
echo "=== PART 3: Integration Testing ==="
test_e2e_workflow
test_cluster_workflow
test_large_database_workflow

# PART 4: Performance
echo "=== PART 4: Performance Testing ==="
test_compression_performance
test_parallel_performance

# PART 5: Regression
echo "=== PART 5: Regression Testing ==="
test_known_issues
test_previous_bugs

# Summary
print_test_summary
```

---

## Test Execution Checklist

### Pre-Testing
- [ ] Build all binaries: `./build_all.sh`
- [ ] Verify PostgreSQL running: `systemctl status postgresql`
- [ ] Create test databases
- [ ] Ensure adequate disk space (20GB+)
- [ ] Install pigz: `yum install pigz` or `apt-get install pigz`
- [ ] Set up test user if needed

### Command-Line Testing (Automated)
- [ ] Run: `./run_master_tests.sh`
- [ ] Review automated test output
- [ ] Check all tests passed
- [ ] Review log file for errors

### Interactive TUI Testing (Manual)
- [ ] Launch TUI: `./dbbackup`
- [ ] Test all main menu options
- [ ] Test all navigation methods (arrows, vim keys, numbers)
- [ ] Test single database backup flow
- [ ] Test cluster backup flow
- [ ] **[CRITICAL]** Test restore cluster selection (Enter key bug)
- [ ] Test restore single flow
- [ ] Test status displays
- [ ] Test settings menu
- [ ] Test all error scenarios
- [ ] Test ESC/quit functionality
- [ ] Test help displays

### Integration Testing
- [ ] Run E2E workflow script
- [ ] Run cluster workflow script
- [ ] Run large database workflow (if available)
- [ ] Verify data integrity after restores

### Performance Testing
- [ ] Run compression benchmarks
- [ ] Run parallel job benchmarks
- [ ] Monitor resource usage (htop/top)

### Stress Testing
- [ ] Run concurrent operations test
- [ ] Run many-database test
- [ ] Monitor for crashes or hangs

### Regression Testing
- [ ] Verify all known issues
- [ ] Test all previously fixed bugs
- [ ] Check for new regressions

### Post-Testing
- [ ] Review all test results
- [ ] Document any failures
- [ ] Create GitHub issues for new bugs
- [ ] Update test plan with new test cases
- [ ] Clean up test databases and files

---

## Success Criteria

### Minimum Requirements for Production Release
- ✅ All critical CLI commands work (backup/restore/status)
- ✅ No data loss in backup/restore cycle
- ✅ Streaming compression works for large databases
- ✅ No disk space exhaustion
- ✅ TUI launches and main menu navigates
- ✅ Error messages are clear and helpful

### Nice-to-Have (Can be fixed in minor releases)
- ⚠️ TUI Enter key on cluster restore
- ⚠️ Debug logging functionality
- ⚠️ All TUI error scenarios handled gracefully
- ⚠️ All performance optimizations tested

### Test Coverage Goals
- [ ] 100% of CLI flags tested
- [ ] 90%+ of TUI flows tested (manual)
- [ ] 100% of critical workflows tested
- [ ] 80%+ success rate on all tests

---

## Test Result Documentation

### Test Execution Log Template
```
Test Execution: MASTER_TEST_PLAN v1.0
Date: YYYY-MM-DD
Tester: <name>
Version: dbbackup v1.2.0
Environment: <OS, PostgreSQL version>

PART 1: CLI Flags
- Global Flags: X/Y passed
- Connection Flags: X/Y passed
- Backup Flags: X/Y passed
- Restore Flags: X/Y passed
- Status Flags: X/Y passed

PART 2: TUI Testing
- Navigation: PASS/FAIL
- Main Menu: PASS/FAIL
- Backup Flows: PASS/FAIL
- Restore Flows: PASS/FAIL
- Status: PASS/FAIL
- Settings: PASS/FAIL
- Error Handling: PASS/FAIL
- Known Issues: <list>

PART 3: Integration
- E2E Workflow: PASS/FAIL
- Cluster Workflow: PASS/FAIL
- Large DB Workflow: PASS/FAIL

PART 4: Performance
- Compression: PASS/FAIL
- Parallel: PASS/FAIL

PART 5: Regression
- Known Issues: X/Y verified
- Previous Bugs: X/Y verified

SUMMARY
- Total Tests: X
- Passed: Y
- Failed: Z
- Success Rate: N%
- Production Ready: YES/NO

FAILED TESTS:
1. <description>
2. <description>

NOTES:
<any additional observations>
```

---

## Appendix: Quick Reference

### Essential Test Commands
```bash
# Quick smoke test
./dbbackup --version
./dbbackup backup single postgres --insecure
./dbbackup status host --insecure

# Full validation
./production_validation.sh

# Interactive testing
./dbbackup

# Check for leftover processes
ps aux | grep -E 'pg_dump|pigz|dbbackup'

# Check for temp files
find /var/lib/pgsql/db_backups -name ".cluster_*"
find /tmp -name "dbbackup_*"

# Monitor resources
htop
df -h
```

### Useful Debugging Commands
```bash
# Enable debug logging (if working)
./dbbackup --debug backup single postgres --insecure

# Verbose PostgreSQL
PGOPTIONS='-c log_statement=all' ./dbbackup status host --insecure

# Trace system calls
strace -o trace.log ./dbbackup backup single postgres --insecure

# Check backup file integrity
pg_restore --list backup.dump | head -20
tar -tzf cluster_backup.tar.gz | head -20
```

---

**END OF MASTER TEST PLAN**

**Estimated Testing Time**: 4-6 hours (2 hours CLI, 2 hours TUI, 1-2 hours integration/performance)

**Minimum Testing Time**: 2 hours (critical paths only)

**Recommended**: Full execution before each major release
