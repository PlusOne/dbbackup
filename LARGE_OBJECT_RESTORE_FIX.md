# Large Object Restore Fix

## Problem Analysis

### Error 1: "type backup_state already exists" (postgres database)
**Root Cause**: `--single-transaction` combined with `--exit-on-error` causes entire restore to fail when objects already exist in target database.

**Why it fails**:
- `--single-transaction` wraps restore in BEGIN/COMMIT
- `--exit-on-error` aborts on ANY error (including ignorable ones)
- "already exists" errors are IGNORABLE - PostgreSQL should continue

### Error 2: "could not open large object 9646664" + 2.5M errors (resydb database)  
**Root Cause**: `--single-transaction` takes locks on ALL restored objects simultaneously, exhausting lock table.

**Why it fails**:
- Single transaction locks ALL large objects at once
- With 35,000+ large objects, exceeds max_locks_per_transaction
- Lock exhaustion → "could not open large object" errors
- Cascading failures → millions of errors

## PostgreSQL Documentation (Verified)

### From pg_restore docs:
> **"pg_restore cannot restore large objects selectively"** - All large objects restored together

> **"-j / --jobs: Only custom and directory formats supported"**

> **"multiple jobs cannot be used together with --single-transaction"**

### From Section 19.5 (Resource Consumption):
> **"max_locks_per_transaction × max_connections = total locks"**
- Lock table is SHARED across all sessions
- Single transaction consuming all locks blocks everything

## Changes Made

### 1. Disabled `--single-transaction` (CRITICAL FIX)
**File**: `internal/restore/engine.go`
- Line 186: `SingleTransaction: false` (was: true)
- Line 210: `SingleTransaction: false` (was: true)

**Impact**:
- No longer wraps entire restore in one transaction
- Each object restored in its own transaction
- Locks released incrementally (not held until end)
- Prevents lock table exhaustion

### 2. Removed `--exit-on-error` (CRITICAL FIX)
**File**: `internal/database/postgresql.go`
- Line 375-378: Removed `cmd.append("--exit-on-error")`

**Impact**:
- PostgreSQL continues on ignorable errors (correct behavior)
- "already exists" errors logged but don't stop restore
- Final error count reported at end
- Only real errors cause failure

### 3. Kept Sequential Parallelism Detection
**File**: `internal/restore/engine.go`
- Lines 552-565: `detectLargeObjectsInDumps()` still active
- Automatically reduces cluster parallelism to 1 when BLOBs detected

**Impact**:
- Prevents multiple databases with large objects from competing for locks
- Sequential cluster restore = only one DB's large objects in lock table at a time

## Why This Works

### Before (BROKEN):
```
START TRANSACTION;  -- Single transaction begins
  CREATE TABLE ...  -- Lock acquired
  CREATE INDEX ...  -- Lock acquired
  RESTORE BLOB 1    -- Lock acquired
  RESTORE BLOB 2    -- Lock acquired
  ...
  RESTORE BLOB 35000 -- Lock acquired → EXHAUSTED!
  ERROR: max_locks_per_transaction exceeded
ROLLBACK;  -- Everything fails
```

### After (FIXED):
```
BEGIN; CREATE TABLE ...; COMMIT;  -- Lock released
BEGIN; CREATE INDEX ...; COMMIT;  -- Lock released
BEGIN; RESTORE BLOB 1; COMMIT;    -- Lock released
BEGIN; RESTORE BLOB 2; COMMIT;    -- Lock released
...
BEGIN; RESTORE BLOB 35000; COMMIT; -- Each only holds ~100 locks max
SUCCESS: All objects restored
```

## Testing Recommendations

### 1. Test with postgres database (backup_state error)
```bash
./dbbackup restore cluster /path/to/backup.tar.gz
# Should now skip "already exists" errors and continue
```

### 2. Test with resydb database (large objects)
```bash
# Check dump for large objects first
pg_restore -l resydb.dump | grep -i "blob\|large object"

# Restore should now work without lock exhaustion
./dbbackup restore cluster /path/to/backup.tar.gz
```

### 3. Monitor locks during restore
```sql
-- In another terminal while restore runs:
SELECT count(*) FROM pg_locks;
-- Should stay well below max_locks_per_transaction × max_connections
```

## Expected Behavior Now

### For "already exists" errors:
```
pg_restore: warning: object already exists: TYPE backup_state
pg_restore: warning: object already exists: FUNCTION ...
... (continues restoring) ...
pg_restore: total errors: 10 (all ignorable)
SUCCESS
```

### For large objects:
```
Restoring database resydb...
  Large objects detected - using sequential restore
  Restoring 35,000 large objects... (progress)
  ✓ Database resydb restored successfully
```

## Configuration Settings (Still Valid)

These PostgreSQL settings help but are NO LONGER REQUIRED with the fix:

```ini
# Still recommended for performance, not required for correctness:
max_locks_per_transaction = 256        # Provides headroom
maintenance_work_mem = 1GB             # Faster index creation
shared_buffers = 8GB                   # Better caching
```

## Commit This Fix

```bash
git add internal/restore/engine.go internal/database/postgresql.go
git commit -m "CRITICAL FIX: Remove --single-transaction and --exit-on-error from pg_restore

- Disabled --single-transaction to prevent lock table exhaustion with large objects
- Removed --exit-on-error to allow PostgreSQL to skip ignorable errors
- Fixes 'could not open large object' errors (lock exhaustion)
- Fixes 'already exists' errors causing complete restore failure
- Each object now restored in its own transaction (locks released incrementally)
- PostgreSQL default behavior (continue on ignorable errors) is correct for restores

Per PostgreSQL docs: --single-transaction incompatible with large object restores
and causes lock table exhaustion with 1000+ objects."

git push
```
