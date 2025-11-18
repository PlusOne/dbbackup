# Interactive TUI Experience Improvements

## Current Issues & Solutions

### 1. **Progress Visibility During Long Operations**

**Problem**: Cluster backup/restore with large databases (40GB+) takes 30+ minutes with minimal feedback.

**Solutions**:
- ✅ Show current database being processed
- ✅ Display database size before backup/restore starts
- ✅ ETA estimator for multi-database operations
- 🔄 **NEW**: Real-time progress bar per database (bytes processed / total bytes)
- 🔄 **NEW**: Show current operation speed (MB/s)
- 🔄 **NEW**: Percentage complete for entire cluster operation

### 2. **Error Handling & Recovery**

**Problem**: When restore fails (like resydb with 2.5M errors), user has no context about WHY or WHAT to do.

**Solutions**:
- ✅ Distinguish ignorable errors (already exists) from critical errors
- 🔄 **NEW**: Show error classification in TUI:
  ```
  ⚠️  WARNING: 5 ignorable errors (objects already exist)
  ❌ CRITICAL: Syntax errors detected - dump file may be corrupted
  💡 HINT: Re-create backup with: dbbackup backup single resydb
  ```
- 🔄 **NEW**: Offer retry option for failed databases
- 🔄 **NEW**: Skip vs Abort choice for non-critical failures

### 3. **Large Object Detection Feedback**

**Problem**: User doesn't know WHY parallelism was reduced.

**Solution**:
```
🔍 Scanning cluster backup for large objects...
   ✓ postgres: No large objects
   ⚠️  d7030: 35,000 BLOBs detected (42GB)
   
⚙️  Automatically reducing parallelism: 2 → 1 (sequential)
💡 Reason: Large objects require exclusive lock table access
```

### 4. **Disk Space Warnings**

**Problem**: Backup fails silently when disk is full.

**Solutions**:
- 🔄 **NEW**: Pre-flight check before backup:
  ```
  📊 Disk Space Check:
     Database size: 42GB
     Available space: 66GB
     Estimated backup: ~15GB (compressed)
     ✓ Sufficient space available
  ```
- 🔄 **NEW**: Warning at 80% disk usage
- 🔄 **NEW**: Block operation at 95% disk usage

### 5. **Cancellation Handling (Ctrl+C)**

**Problem**: Users don't know if Ctrl+C will work or leave partial backups.

**Solutions**:
- ✅ Graceful cancellation on Ctrl+C
- 🔄 **NEW**: Show cleanup message:
  ```
  ^C received - Cancelling backup...
  🧹 Cleaning up temporary files...
  ✓ Cleanup complete - no partial backups left
  ```
- 🔄 **NEW**: Confirmation prompt for cluster operations:
  ```
  ⚠️  Cluster backup in progress (3/10 databases)
  Are you sure you want to cancel? (y/N)
  ```

### 6. **Interactive Mode Navigation**

**Problem**: TUI menu is basic, no keyboard shortcuts, no search.

**Solutions**:
- 🔄 **NEW**: Keyboard shortcuts:
  - `1-9`: Quick jump to menu items
  - `q`: Quit
  - `r`: Refresh status
  - `/`: Search backups
- 🔄 **NEW**: Backup list improvements:
  ```
  📦 Available Backups:
  
  1. cluster_20251118_103045.tar.gz  [45GB]  ⏱ 2 hours ago
     ├─ postgres (325MB)
     ├─ d7030 (42GB) ⚠️ 35K BLOBs
     └─ template1 (8MB)
  
  2. cluster_20251112_084329.tar.gz  [38GB]  ⏱ 6 days ago
     └─ ⚠️ WARNING: May contain corrupted resydb dump
  ```
- 🔄 **NEW**: Filter/sort options: by date, by size, by status

### 7. **Configuration Recommendations**

**Problem**: Users don't know optimal settings for their workload.

**Solutions**:
- 🔄 **NEW**: Auto-detect and suggest settings on first run:
  ```
  🔧 System Configuration Detected:
     RAM: 32GB → Recommended: shared_buffers=8GB
     CPUs: 4 cores → Recommended: parallel_jobs=3
     Disk: 66GB free → Recommended: max backup size: 50GB
     
  Apply these settings? (Y/n)
  ```
- 🔄 **NEW**: Show current vs recommended config in menu:
  ```
  ⚙️  Configuration Status:
     max_locks_per_transaction: 256 ✓ (sufficient for 35K objects)
     maintenance_work_mem: 64MB ⚠️ (recommend: 1GB for faster restores)
     shared_buffers: 128MB ⚠️ (recommend: 8GB with 32GB RAM)
  ```

### 8. **Backup Verification & Health**

**Problem**: No way to verify backup integrity before restore.

**Solutions**:
- 🔄 **NEW**: Add "Verify Backup" menu option:
  ```
  🔍 Verifying backup: cluster_20251118_103045.tar.gz
     ✓ Archive integrity: OK
     ✓ Extracting metadata...
     ✓ Checking dump formats...
     
  Databases found:
     ✓ postgres: Custom format, 325MB
     ✓ d7030: Custom format, 42GB, 35,000 BLOBs
     ⚠️  resydb: CORRUPTED - 2.5M syntax errors detected
     
  Overall: ⚠️ Partial (2/3 databases healthy)
  ```
- 🔄 **NEW**: Show last backup status in main menu

### 9. **Restore Dry Run**

**Problem**: No preview of what will be restored.

**Solution**:
```
🎬 Restore Preview (Dry Run):

Target: cluster_20251118_103045.tar.gz
Databases to restore:
  1. postgres (325MB)
     - Will overwrite: 5 existing objects
     - New objects: 120
     
  2. d7030 (42GB, 35K BLOBs)
     - Will DROP and recreate database
     - Estimated time: 25-30 minutes
     - Required locks: 35,000 (available: 25,600) ⚠️
     
⚠️  WARNING: Insufficient locks for d7030
💡 Solution: Increase max_locks_per_transaction to 512

Proceed with restore? (y/N)
```

### 10. **Multi-Step Wizards**

**Problem**: Complex operations (like cluster restore with --clean) need multiple confirmations.

**Solution**: Step-by-step wizard:
```
Step 1/4: Select backup
Step 2/4: Review databases to restore
Step 3/4: Check prerequisites (disk space, locks, etc.)
Step 4/4: Confirm and execute
```

## Implementation Priority

### Phase 1 (High Impact, Low Effort) ✅
- ✅ ETA estimators
- ✅ Large object detection warnings
- ✅ Ctrl+C handling
- ✅ Ignorable error detection

### Phase 2 (High Impact, Medium Effort) 🔄
- Real-time progress bars with MB/s
- Disk space pre-flight checks
- Backup verification tool
- Error hints and suggestions

### Phase 3 (Quality of Life) 🔄
- Keyboard shortcuts
- Backup list with metadata
- Configuration recommendations
- Restore dry run

### Phase 4 (Advanced) 📋
- Multi-step wizards
- Search/filter backups
- Auto-retry failed databases
- Parallel restore progress split-view

## Code Structure

```
internal/tui/
  menu.go          - Main interactive menu
  backup_menu.go   - Backup wizard
  restore_menu.go  - Restore wizard
  verify_menu.go   - Backup verification (NEW)
  config_menu.go   - Configuration tuning (NEW)
  progress_view.go - Real-time progress display (ENHANCED)
  errors.go        - Error classification & hints (NEW)
```

## Testing Plan

1. **Large Database Test** (In Progress)
   - 42GB d7030 with 35K BLOBs
   - Verify progress updates
   - Verify large object detection
   - Verify successful restore

2. **Error Scenarios**
   - Corrupted dump file
   - Insufficient disk space
   - Insufficient locks
   - Network interruption
   - Ctrl+C during operations

3. **Performance**
   - Backup time vs raw pg_dump
   - Restore time vs raw pg_restore
   - Memory usage during 40GB+ operations
   - CPU utilization with parallel workers

## Success Metrics

- ✅ No "black box" operations - user always knows what's happening
- ✅ Errors are actionable - user knows what to fix
- ✅ Safe operations - confirmations for destructive actions
- ✅ Fast feedback - progress updates every 1-2 seconds
- ✅ Professional feel - polished, consistent, intuitive
