# Restore Implementation Plan

## Current State

### Existing Infrastructure
- ✅ Database interface with `BuildRestoreCommand()` method
- ✅ RestoreOptions struct defined
- ✅ Placeholder `restore` command in CLI (preview mode only)
- ✅ Archive type detection
- ✅ PostgreSQL and MySQL restore command builders
- ✅ Config with `RestoreDBName` field

### Missing Components
- ❌ Restore engine implementation
- ❌ Progress tracking for restore operations
- ❌ TUI integration for restore
- ❌ Error handling and rollback strategies
- ❌ Pre-restore validation
- ❌ Post-restore verification

## Implementation Plan

### Phase 1: Core Restore Engine (Priority: HIGH)

#### 1.1 Create Restore Engine
**File**: `internal/restore/engine.go`

```go
type Engine struct {
    cfg              *config.Config
    log              logger.Logger
    db               database.Database
    progress         progress.Indicator
    detailedReporter *progress.DetailedReporter
}

// Core methods
- New(cfg, log, db) *Engine
- RestoreSingle(ctx, archivePath, targetDB) error
- RestoreCluster(ctx, archivePath) error
- ValidateArchive(archivePath) error
```

**Features**:
- Archive format detection (dump, sql, tar.gz)
- Decompression handling (gzip, pigz)
- Progress tracking integration
- Error recovery
- Dry-run mode (`--dry-run` flag)

#### 1.2 Archive Format Handlers
**File**: `internal/restore/formats.go`

```go
type ArchiveFormat interface {
    Detect(path string) bool
    Extract(ctx, path, tempDir) error
    GetDatabases() ([]string, error)
}

// Implementations
- PostgreSQLDump    // .dump files
- PostgreSQLSQL     // .sql files
- MySQLSQL          // .sql.gz files
- ClusterArchive    // .tar.gz files
```

#### 1.3 Safety Features
**File**: `internal/restore/safety.go`

```go
// Pre-restore checks
- ValidateTargetDatabase()
- CheckDiskSpace()
- VerifyArchiveIntegrity()
- BackupExistingData() // Optional safety backup

// Confirmation system
- RequireConfirmation(archiveInfo, targetDB)
- GenerateRestorePreview()
```

### Phase 2: CLI Integration (Priority: HIGH)

#### 2.1 Update Restore Command
**File**: `cmd/restore.go` (new file, move from placeholder.go)

```bash
# Single database restore
dbbackup restore single backup.dump --target-db myapp_db

# Cluster restore
dbbackup restore cluster cluster_20251105.tar.gz

# With confirmation
dbbackup restore single backup.dump --target-db myapp_db --confirm

# Dry run (preview only)
dbbackup restore single backup.dump --target-db myapp_db --dry-run

# Force overwrite (skip confirmation)
dbbackup restore single backup.dump --target-db myapp_db --force
```

**Flags**:
- `--target-db`: Target database name (for single restore)
- `--confirm`: Enable actual restore (default: preview only)
- `--dry-run`: Show what would be done without executing
- `--force`: Skip confirmation prompts
- `--clean`: Drop existing database before restore
- `--create`: Create database if it doesn't exist
- `--no-owner`: Skip ownership restoration
- `--jobs`: Parallel restore jobs (PostgreSQL only)

#### 2.2 Command Structure
```
dbbackup restore
├── single <archive> --target-db <name>
│   ├── --confirm           # Actually execute
│   ├── --dry-run           # Preview only
│   ├── --force             # Skip prompts
│   ├── --clean             # Drop existing
│   ├── --create            # Create if missing
│   └── --jobs <n>          # Parallel jobs
├── cluster <archive>
│   ├── --confirm
│   ├── --dry-run
│   ├── --exclude <db>      # Skip specific DBs
│   └── --include <db>      # Only restore specific DBs
└── list <archive>          # List archive contents
    └── --format json       # JSON output
```

### Phase 3: Progress & Monitoring (Priority: MEDIUM)

#### 3.1 Restore Progress Tracking
**File**: `internal/restore/progress.go`

```go
type RestoreProgress struct {
    Phase          string        // "extraction", "restoration", "verification"
    Database       string
    TablesTotal    int
    TablesRestored int
    BytesRestored  int64
    StartTime      time.Time
    EstimatedTime  time.Duration
}
```

**Output Example**:
```
🔄 Restoring from: backup_20251105.tar.gz
   Phase: Extraction
   [1/3] Extracting archive...
   ✅ Archive extracted (56.6 MB)
   
   Phase: Restoration
   [2/3] Restoring databases...
       [1/13] myapp_db (tables: 45)
           ├─ users (1.2M rows) ✅
           ├─ orders (890K rows) ✅
           ├─ products (12K rows) ✅
   ✅ Database myapp_db restored (2.5 GB)
   
   Phase: Verification
   [3/3] Verifying restore...
   ✅ All tables verified
   
✅ Restore completed in 4m 23s
```

### Phase 4: TUI Integration (Priority: MEDIUM)

#### 4.1 TUI Restore Menu
**File**: `internal/tui/restore.go`

```
┌─ Restore Menu ────────────────────────────┐
│                                            │
│  1. Restore Single Database                │
│  2. Restore Cluster                        │
│  3. List Archive Contents                  │
│  4. Back to Main Menu                      │
│                                            │
│  Engine: PostgreSQL  ◀─────────────▶      │
│                                            │
└────────────────────────────────────────────┘
```

#### 4.2 Archive Browser
**File**: `internal/tui/archive_browser.go`

```
┌─ Select Archive ──────────────────────────┐
│                                            │
│  📦 cluster_20251105_132923.tar.gz        │
│     56.6 MB  |  2025-11-05 13:29          │
│                                            │
│  📦 db_myapp_20251104_151022.dump         │
│     807 KB   |  2025-11-04 15:10          │
│                                            │
│  ⬅  Back    ✓ Select                      │
└────────────────────────────────────────────┘
```

#### 4.3 Restore Execution Screen
Similar to backup execution but shows:
- Archive being restored
- Target database
- Current phase (extraction/restoration/verification)
- Tables being restored
- Progress indicators

### Phase 5: Advanced Features (Priority: LOW)

#### 5.1 Selective Restore
```bash
# Restore only specific tables
dbbackup restore single backup.dump \
  --target-db myapp_db \
  --table users \
  --table orders \
  --confirm

# Restore specific schemas
dbbackup restore single backup.dump \
  --target-db myapp_db \
  --schema public \
  --confirm
```

#### 5.2 Point-in-Time Restore
```bash
# Restore to specific timestamp (requires WAL archives)
dbbackup restore cluster archive.tar.gz \
  --target-time "2025-11-05 12:00:00" \
  --confirm
```

#### 5.3 Restore with Transformation
```bash
# Rename database during restore
dbbackup restore single prod_backup.dump \
  --target-db dev_database \
  --confirm

# Restore to different server
dbbackup restore single backup.dump \
  --target-db myapp_db \
  --host newserver.example.com \
  --confirm
```

#### 5.4 Parallel Cluster Restore
```bash
# Restore multiple databases in parallel
dbbackup restore cluster archive.tar.gz \
  --jobs 4 \
  --confirm
```

## Implementation Order

### Week 1: Core Functionality
1. ✅ Create `internal/restore/engine.go`
2. ✅ Implement `RestoreSingle()` for PostgreSQL
3. ✅ Implement `RestoreSingle()` for MySQL
4. ✅ Add archive format detection
5. ✅ Basic error handling

### Week 2: CLI & Safety
6. ✅ Move restore command to `cmd/restore.go`
7. ✅ Add `--confirm`, `--dry-run`, `--force` flags
8. ✅ Implement safety checks (ValidateArchive, CheckDiskSpace)
9. ✅ Add confirmation prompts
10. ✅ Progress tracking integration

### Week 3: Cluster Restore
11. ✅ Implement `RestoreCluster()` for tar.gz archives
12. ✅ Add extraction handling
13. ✅ Implement global objects restoration (roles, tablespaces)
14. ✅ Add per-database restoration with progress

### Week 4: TUI Integration
15. ✅ Create restore menu in TUI
16. ✅ Add archive browser
17. ✅ Implement restore execution screen
18. ✅ Add real-time progress updates

### Week 5: Testing & Polish
19. ✅ Integration tests
20. ✅ Error scenario testing
21. ✅ Documentation updates
22. ✅ CLI help text refinement

## File Structure

```
dbbackup/
├── cmd/
│   ├── restore.go                 # NEW: Restore commands
│   └── restore_impl.go            # NEW: Restore implementations
├── internal/
│   ├── restore/                   # NEW: Restore engine
│   │   ├── engine.go              # Core restore engine
│   │   ├── formats.go             # Archive format handlers
│   │   ├── safety.go              # Pre/post restore checks
│   │   ├── progress.go            # Restore progress tracking
│   │   └── verify.go              # Post-restore verification
│   ├── tui/
│   │   ├── restore.go             # NEW: Restore menu
│   │   ├── archive_browser.go    # NEW: Archive selection
│   │   └── restore_exec.go        # NEW: Restore execution screen
│   └── database/
│       ├── interface.go           # (existing - already has BuildRestoreCommand)
│       ├── postgresql.go          # (existing - already has BuildRestoreCommand)
│       └── mysql.go               # (existing - already has BuildRestoreCommand)
```

## Key Design Decisions

### 1. Safety-First Approach
- **Default: Preview Mode**: `restore` command shows what would be done
- **Explicit Confirmation**: Require `--confirm` flag for actual execution
- **Dry-Run Option**: `--dry-run` for testing without side effects
- **Pre-restore Validation**: Check archive integrity before starting

### 2. Progress Transparency
- Real-time progress indicators
- Phase-based tracking (extraction → restoration → verification)
- Per-table progress for detailed visibility
- Time estimates and completion ETA

### 3. Error Handling
- Graceful degradation (continue on non-critical errors)
- Detailed error messages with troubleshooting hints
- Transaction-based restoration where possible
- Rollback support (optional safety backup before restore)

### 4. Performance
- Parallel restoration for cluster backups
- Streaming decompression (no temporary files for compressed archives)
- CPU-aware job counts (reuse existing CPU detection)
- Memory-efficient processing (similar to backup streaming)

### 5. Compatibility
- Support all backup formats created by the tool
- Handle legacy backups (if format allows)
- Cross-database restore (with transformations)
- Version compatibility checks

## Testing Strategy

### Unit Tests
- Archive format detection
- Command building (PostgreSQL, MySQL)
- Safety validation logic
- Error handling paths

### Integration Tests
- Single database restore (PostgreSQL, MySQL)
- Cluster restore (PostgreSQL)
- Compressed archive handling
- Large database restoration (memory usage verification)

### Manual Testing Scenarios
1. Restore to same database (overwrite)
2. Restore to different database (rename)
3. Restore to different server
4. Partial restore (selective tables)
5. Cluster restore with exclusions
6. Error scenarios (disk full, connection lost, corrupted archive)

## Documentation Updates

### README.md
```markdown
## Restore Operations

### Single Database
dbbackup restore single backup.dump --target-db myapp_db --confirm

### Cluster Restore
dbbackup restore cluster cluster_20251105.tar.gz --confirm

### Preview Mode (Default)
dbbackup restore single backup.dump --target-db myapp_db
# Shows what would be done without executing

### Flags
- --confirm: Actually execute the restore
- --dry-run: Preview only (explicit)
- --force: Skip confirmation prompts
- --clean: Drop existing database before restore
- --create: Create database if missing
```

### New Documentation Files
- `RESTORE_GUIDE.md`: Comprehensive restore guide
- `RESTORE_BEST_PRACTICES.md`: Safety and performance tips

## Migration Path

Since restore is currently in "preview mode":
1. Keep existing `runRestore()` as reference
2. Implement new restore engine in parallel
3. Add `--legacy` flag to use old preview mode (temporary)
4. Test new implementation thoroughly
5. Remove legacy code after validation
6. Update all documentation

## Success Metrics

- ✅ Single database restore works for PostgreSQL and MySQL
- ✅ Cluster restore works for PostgreSQL
- ✅ All backup formats supported
- ✅ Progress tracking provides clear feedback
- ✅ Safety checks prevent accidental data loss
- ✅ Memory usage remains <1GB for large restores
- ✅ TUI integration matches backup experience
- ✅ Documentation is comprehensive
