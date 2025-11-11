# Production-Ready Testing Plan

**Date**: November 11, 2025  
**Version**: 1.0  
**Goal**: Verify complete functionality for production deployment

---

## Test Environment Status

- ✅ 7.5GB test database created (`testdb_50gb`)
- ✅ Multiple test databases (17 total)
- ✅ Test roles and ownership configured (`testowner`)
- ✅ 107GB available disk space
- ✅ PostgreSQL cluster operational

---

## Phase 1: Command-Line Testing (Critical Path)

### 1.1 Cluster Backup - Full Test
**Priority**: CRITICAL  
**Status**: ⚠️ NEEDS COMPLETION

**Test Steps:**
```bash
# Clean environment
sudo rm -rf /var/lib/pgsql/db_backups/.cluster_*

# Execute cluster backup with compression level 6 (production default)
time sudo -u postgres ./dbbackup backup cluster

# Verify output
ls -lh /var/lib/pgsql/db_backups/cluster_*.tar.gz | tail -1
cat /var/lib/pgsql/db_backups/cluster_*.tar.gz.info
```

**Success Criteria:**
- [ ] All databases backed up successfully (0 failures)
- [ ] Archive created (>500MB expected)
- [ ] Completion time <15 minutes
- [ ] No memory errors in dmesg
- [ ] Metadata file created

---

### 1.2 Cluster Restore - Full Test with Ownership Verification
**Priority**: CRITICAL  
**Status**: ⚠️ NOT TESTED

**Pre-Test: Document Current Ownership**
```bash
# Check current ownership across key databases
sudo -u postgres psql -c "\l+" | grep -E "ownership_test|testdb"

# Check table ownership in ownership_test
sudo -u postgres psql -d ownership_test -c \
  "SELECT schemaname, tablename, tableowner FROM pg_tables WHERE schemaname = 'public';"

# Check roles
sudo -u postgres psql -c "\du"
```

**Test Steps:**
```bash
# Get latest cluster backup
BACKUP=$(ls -t /var/lib/pgsql/db_backups/cluster_*.tar.gz | head -1)

# Dry run first
sudo -u postgres ./dbbackup restore cluster "$BACKUP" --dry-run

# Execute restore with confirmation
time sudo -u postgres ./dbbackup restore cluster "$BACKUP" --confirm

# Verify restoration
sudo -u postgres psql -c "\l+" | wc -l
```

**Post-Test: Verify Ownership Preserved**
```bash
# Check database ownership restored
sudo -u postgres psql -c "\l+" | grep -E "ownership_test|testdb"

# Check table ownership preserved
sudo -u postgres psql -d ownership_test -c \
  "SELECT schemaname, tablename, tableowner FROM pg_tables WHERE schemaname = 'public';"

# Verify testowner role exists
sudo -u postgres psql -c "\du" | grep testowner

# Check access privileges
sudo -u postgres psql -l | grep -E "Access privileges"
```

**Success Criteria:**
- [ ] All databases restored successfully
- [ ] Database ownership matches original
- [ ] Table ownership preserved (testowner still owns test_data)
- [ ] Roles restored from globals.sql
- [ ] No permission errors
- [ ] Data integrity: row counts match
- [ ] Completion time <30 minutes

---

### 1.3 Large Database Operations
**Priority**: HIGH  
**Status**: ✅ COMPLETED (7.5GB single DB)

**Additional Test Needed:**
```bash
# Test single database restore with ownership
BACKUP=/var/lib/pgsql/db_backups/db_testdb_50gb_*.dump

# Drop and recreate to test full cycle
sudo -u postgres psql -c "DROP DATABASE IF EXISTS testdb_50gb_restored;"

# Restore
time sudo -u postgres ./dbbackup restore single "$BACKUP" \
  --target testdb_50gb_restored --create --confirm

# Verify size and data
sudo -u postgres psql -d testdb_50gb_restored -c \
  "SELECT pg_size_pretty(pg_database_size('testdb_50gb_restored'));"
```

**Success Criteria:**
- [ ] Restore completes successfully
- [ ] Database size matches original (~7.5GB)
- [ ] Row counts match (7M+ rows)
- [ ] Completion time <25 minutes

---

### 1.4 Authentication Methods Testing
**Priority**: HIGH  
**Status**: ⚠️ NEEDS VERIFICATION

**Test Cases:**
```bash
# Test 1: Peer authentication (current working method)
sudo -u postgres ./dbbackup status

# Test 2: Password authentication (if configured)
./dbbackup status --user postgres --password "$PGPASSWORD"

# Test 3: ~/.pgpass file (if exists)
cat ~/.pgpass
./dbbackup status --user postgres

# Test 4: Environment variable
export PGPASSWORD="test_password"
./dbbackup status --user postgres
unset PGPASSWORD
```

**Success Criteria:**
- [ ] At least one auth method works
- [ ] Error messages are clear and helpful
- [ ] Authentication detection working

---

### 1.5 Privilege Diagnostic Tool
**Priority**: MEDIUM  
**Status**: ✅ CREATED, ⚠️ NEEDS EXECUTION

**Test Steps:**
```bash
# Run diagnostic on current system
./privilege_diagnostic.sh > privilege_report_production.txt

# Review output
cat privilege_report_production.txt

# Compare with expectations
grep -A 10 "DATABASE PRIVILEGES" privilege_report_production.txt
```

**Success Criteria:**
- [ ] Script runs without errors
- [ ] Shows all database privileges
- [ ] Identifies roles correctly
- [ ] globals.sql content verified

---

## Phase 2: Interactive Mode Testing (TUI)

### 2.1 TUI Launch and Navigation
**Priority**: HIGH  
**Status**: ⚠️ NOT FULLY TESTED

**Test Steps:**
```bash
# Launch TUI
sudo -u postgres ./dbbackup interactive

# Test navigation:
# - Arrow keys: ↑ ↓ to move through menu
# - Enter: Select option
# - Esc/q: Go back/quit
# - Test all 10 main menu options
```

**Menu Items to Test:**
1. [ ] Single Database Backup
2. [ ] Sample Database Backup
3. [ ] Full Cluster Backup
4. [ ] Restore Single Database
5. [ ] Restore Cluster Backup
6. [ ] List Backups
7. [ ] View Operation History
8. [ ] Database Status
9. [ ] Settings
10. [ ] Exit

**Success Criteria:**
- [ ] TUI launches without errors
- [ ] Navigation works smoothly
- [ ] No terminal artifacts
- [ ] Can navigate back with Esc
- [ ] Exit works cleanly

---

### 2.2 TUI Cluster Backup
**Priority**: CRITICAL  
**Status**: ⚠️ ISSUE REPORTED (Enter key not working)

**Test Steps:**
```bash
# Launch TUI
sudo -u postgres ./dbbackup interactive

# Navigate to: Full Cluster Backup (option 3)
# Press Enter to start
# Observe progress indicators
# Wait for completion
```

**Known Issue:**
- User reported: "on cluster backup restore selection - i cant press enter to select the cluster backup - interactiv"

**Success Criteria:**
- [ ] Enter key works to select cluster backup
- [ ] Progress indicators show during backup
- [ ] Backup completes successfully
- [ ] Returns to main menu on completion
- [ ] Backup file listed in backup directory

---

### 2.3 TUI Cluster Restore
**Priority**: CRITICAL  
**Status**: ⚠️ NEEDS TESTING

**Test Steps:**
```bash
# Launch TUI
sudo -u postgres ./dbbackup interactive

# Navigate to: Restore Cluster Backup (option 5)
# Browse available cluster backups
# Select latest backup
# Press Enter to start restore
# Observe progress indicators
# Wait for completion
```

**Success Criteria:**
- [ ] Can browse cluster backups
- [ ] Enter key works to select backup
- [ ] Progress indicators show during restore
- [ ] Restore completes successfully
- [ ] Ownership preserved
- [ ] Returns to main menu on completion

---

### 2.4 TUI Database Selection
**Priority**: HIGH  
**Status**: ⚠️ NEEDS TESTING

**Test Steps:**
```bash
# Test single database backup selection
sudo -u postgres ./dbbackup interactive
# Navigate to: Single Database Backup (option 1)
# Browse database list
# Select testdb_50gb
# Press Enter to start
# Observe progress
```

**Success Criteria:**
- [ ] Database list displays correctly
- [ ] Can scroll through databases
- [ ] Selection works with Enter
- [ ] Progress shows during backup
- [ ] Backup completes successfully

---

## Phase 3: Edge Cases and Error Handling

### 3.1 Disk Space Exhaustion
**Priority**: MEDIUM  
**Status**: ⚠️ NEEDS TESTING

**Test Steps:**
```bash
# Check current space
df -h /

# Test with limited space (if safe)
# Create large file to fill disk to 90%
# Attempt backup
# Verify error handling
```

**Success Criteria:**
- [ ] Clear error message about disk space
- [ ] Graceful failure (no corruption)
- [ ] Cleanup of partial files

---

### 3.2 Interrupted Operations
**Priority**: MEDIUM  
**Status**: ⚠️ NEEDS TESTING

**Test Steps:**
```bash
# Start backup
sudo -u postgres ./dbbackup backup cluster &
PID=$!

# Wait 30 seconds
sleep 30

# Interrupt with Ctrl+C or kill
kill -INT $PID

# Check for cleanup
ls -la /var/lib/pgsql/db_backups/.cluster_*
```

**Success Criteria:**
- [ ] Graceful shutdown on SIGINT
- [ ] Temp directories cleaned up
- [ ] No corrupted files left
- [ ] Clear error message

---

### 3.3 Invalid Archive Files
**Priority**: LOW  
**Status**: ⚠️ NEEDS TESTING

**Test Steps:**
```bash
# Test with non-existent file
sudo -u postgres ./dbbackup restore single /tmp/nonexistent.dump

# Test with corrupted archive
echo "corrupted" > /tmp/bad.dump
sudo -u postgres ./dbbackup restore single /tmp/bad.dump

# Test with wrong format
sudo -u postgres ./dbbackup restore cluster /tmp/single_db.dump
```

**Success Criteria:**
- [ ] Clear error messages
- [ ] No crashes
- [ ] Proper format detection

---

## Phase 4: Performance and Scalability

### 4.1 Memory Usage Monitoring
**Priority**: HIGH  
**Status**: ⚠️ NEEDS MONITORING

**Test Steps:**
```bash
# Monitor during large backup
(
  while true; do
    ps aux | grep dbbackup | grep -v grep
    free -h
    sleep 10
  done
) > memory_usage.log &
MONITOR_PID=$!

# Run backup
sudo -u postgres ./dbbackup backup cluster

# Stop monitoring
kill $MONITOR_PID

# Review memory usage
grep -A 1 "dbbackup" memory_usage.log | grep -v grep
```

**Success Criteria:**
- [ ] Memory usage stays under 1.5GB
- [ ] No OOM errors
- [ ] Memory released after completion

---

### 4.2 Compression Performance
**Priority**: MEDIUM  
**Status**: ⚠️ NEEDS TESTING

**Test Different Compression Levels:**
```bash
# Test compression levels 1, 3, 6, 9
for LEVEL in 1 3 6 9; do
  echo "Testing compression level $LEVEL"
  time sudo -u postgres ./dbbackup backup single testdb_50gb \
    --compression=$LEVEL
done

# Compare sizes and times
ls -lh /var/lib/pgsql/db_backups/db_testdb_50gb_*.dump
```

**Success Criteria:**
- [ ] All compression levels work
- [ ] Higher compression = smaller file
- [ ] Higher compression = longer time
- [ ] Level 6 is good balance

---

## Phase 5: Documentation Verification

### 5.1 README Examples
**Priority**: HIGH  
**Status**: ⚠️ NEEDS VERIFICATION

**Test All README Examples:**
```bash
# Example 1: Single database backup
dbbackup backup single myapp_db

# Example 2: Sample backup
dbbackup backup sample myapp_db --sample-ratio 10

# Example 3: Full cluster backup
dbbackup backup cluster

# Example 4: With custom settings
dbbackup backup single myapp_db \
  --host db.example.com \
  --port 5432 \
  --user backup_user \
  --ssl-mode require

# Example 5: System commands
dbbackup status
dbbackup preflight
dbbackup list
dbbackup cpu
```

**Success Criteria:**
- [ ] All examples work as documented
- [ ] No syntax errors
- [ ] Output matches expectations

---

### 5.2 Authentication Examples
**Priority**: HIGH  
**Status**: ⚠️ NEEDS VERIFICATION

**Test All Auth Methods from README:**
```bash
# Method 1: Peer auth
sudo -u postgres dbbackup status

# Method 2: ~/.pgpass
echo "localhost:5432:*:postgres:password" > ~/.pgpass
chmod 0600 ~/.pgpass
dbbackup status --user postgres

# Method 3: PGPASSWORD
export PGPASSWORD=password
dbbackup status --user postgres

# Method 4: --password flag
dbbackup status --user postgres --password password
```

**Success Criteria:**
- [ ] All methods work or fail with clear errors
- [ ] Documentation matches reality

---

## Phase 6: Cross-Platform Testing

### 6.1 Binary Verification
**Priority**: LOW  
**Status**: ⚠️ NOT TESTED

**Test Binary Compatibility:**
```bash
# List all binaries
ls -lh bin/

# Test each binary (if platform available)
# - dbbackup_linux_amd64
# - dbbackup_linux_arm64
# - dbbackup_darwin_amd64
# - dbbackup_darwin_arm64
# etc.

# At minimum, test current platform
./dbbackup --version
```

**Success Criteria:**
- [ ] Current platform binary works
- [ ] Binaries are not corrupted
- [ ] Reasonable file sizes

---

## Test Execution Checklist

### Pre-Flight
- [ ] Backup current databases before testing
- [ ] Document current system state
- [ ] Ensure sufficient disk space (>50GB free)
- [ ] Check no other backups running
- [ ] Clean temp directories

### Critical Path Tests (Must Pass)
1. [ ] Cluster Backup completes successfully
2. [ ] Cluster Restore completes successfully
3. [ ] Ownership preserved after cluster restore
4. [ ] Large database backup/restore works
5. [ ] TUI launches and navigates correctly
6. [ ] TUI cluster backup works (fix Enter key issue)
7. [ ] Authentication works with at least one method

### High Priority Tests
- [ ] Privilege diagnostic tool runs successfully
- [ ] All README examples work
- [ ] Memory usage is acceptable
- [ ] Progress indicators work correctly
- [ ] Error messages are clear

### Medium Priority Tests
- [ ] Compression levels work correctly
- [ ] Interrupted operations clean up properly
- [ ] Disk space errors handled gracefully
- [ ] Invalid archives detected properly

### Low Priority Tests
- [ ] Cross-platform binaries verified
- [ ] All documentation examples tested
- [ ] Performance benchmarks recorded

---

## Known Issues to Resolve

### Issue #1: TUI Cluster Backup Enter Key
**Reported**: "on cluster backup restore selection - i cant press enter to select the cluster backup - interactiv"  
**Status**: NOT FIXED  
**Priority**: CRITICAL  
**Action**: Debug TUI event handling for cluster restore selection

### Issue #2: Large Database Plain Format Not Compressed
**Discovered**: Plain format dumps are 84GB+ uncompressed, causing slow tar compression  
**Status**: IDENTIFIED  
**Priority**: HIGH  
**Action**: Fix external compression for plain format dumps (pipe through pigz properly)

### Issue #3: Privilege Display Shows NULL
**Reported**: "If i list Databases on Host - i see Access Privilleges are not set"  
**Status**: INVESTIGATING  
**Priority**: MEDIUM  
**Action**: Run privilege_diagnostic.sh on production host and compare

---

## Success Criteria Summary

### Production Ready Checklist
- [ ] ✅ All Critical Path tests pass
- [ ] ✅ No data loss in any scenario
- [ ] ✅ Ownership preserved correctly
- [ ] ✅ Memory usage <2GB for any operation
- [ ] ✅ Clear error messages for all failures
- [ ] ✅ TUI fully functional
- [ ] ✅ README examples all work
- [ ] ✅ Large database support verified (7.5GB+)
- [ ] ✅ Authentication methods work
- [ ] ✅ Backup/restore cycle completes successfully

### Performance Targets
- Single DB Backup (7.5GB): <10 minutes
- Single DB Restore (7.5GB): <25 minutes
- Cluster Backup (16 DBs): <15 minutes
- Cluster Restore (16 DBs): <35 minutes
- Memory Usage: <1.5GB peak
- Compression Ratio: >90% for test data

---

## Test Execution Timeline

**Estimated Time**: 4-6 hours for complete testing

1. **Phase 1**: Command-Line Testing (2-3 hours)
   - Cluster backup/restore cycle
   - Ownership verification
   - Large database operations

2. **Phase 2**: Interactive Mode (1-2 hours)
   - TUI navigation
   - Cluster backup via TUI (fix Enter key)
   - Cluster restore via TUI

3. **Phase 3-4**: Edge Cases & Performance (1 hour)
   - Error handling
   - Memory monitoring
   - Compression testing

4. **Phase 5-6**: Documentation & Cross-Platform (30 minutes)
   - Verify examples
   - Test binaries

---

## Next Immediate Actions

1. **CRITICAL**: Complete cluster backup successfully
   - Clean environment
   - Execute with default compression (6)
   - Verify completion

2. **CRITICAL**: Test cluster restore with ownership
   - Document pre-restore state
   - Execute restore
   - Verify ownership preserved

3. **CRITICAL**: Fix TUI Enter key issue
   - Debug cluster restore selection
   - Test fix thoroughly

4. **HIGH**: Run privilege diagnostic on both hosts
   - Execute on test host
   - Execute on production host
   - Compare results

5. **HIGH**: Complete TUI testing
   - All menu items
   - All operations
   - Error scenarios

---

## Test Results Log

**To be filled during execution:**

```
Date: ___________
Tester: ___________

Phase 1.1 - Cluster Backup: PASS / FAIL
  Time: _______ File Size: _______ Notes: _______

Phase 1.2 - Cluster Restore: PASS / FAIL
  Time: _______ Ownership OK: YES / NO Notes: _______

Phase 1.3 - Large DB Restore: PASS / FAIL
  Time: _______ Size Match: YES / NO Notes: _______

[Continue for all phases...]
```

---

**Document Status**: Draft - Ready for Execution  
**Last Updated**: November 11, 2025  
**Next Review**: After test execution completion
