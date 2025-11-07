# Restore TUI Integration Plan

## Overview
Integrate restore functionality into the interactive TUI menu, following the same architecture patterns as the existing backup features.

---

## Phase 1: Main Menu Integration

### 1.1 Add Restore Menu Items
**File**: `internal/tui/menu.go`

Add new menu choices after backup options:
```go
choices: []string{
    "Single Database Backup",           // 0
    "Sample Database Backup (with ratio)", // 1
    "Cluster Backup (all databases)",   // 2
    "─────────────────────────────",    // 3 (separator)
    "Restore Single Database",          // 4 - NEW
    "Restore Cluster",                  // 5 - NEW
    "List & Manage Backups",            // 6 - NEW
    "─────────────────────────────",    // 7 (separator)
    "View Active Operations",           // 8
    "Show Operation History",           // 9
    "Database Status & Health Check",   // 10
    "Configuration Settings",           // 11
    "Clear Operation History",          // 12
    "Quit",                             // 13
}
```

### 1.2 Add Menu Handlers
Add cases in `Update()` method:
```go
case 4: // Restore Single Database
    return m.handleRestoreSingle()
case 5: // Restore Cluster
    return m.handleRestoreCluster()
case 6: // List & Manage Backups
    return m.handleBackupList()
```

---

## Phase 2: Archive Browser Component

### 2.1 Create Archive Browser Model
**New File**: `internal/tui/archive_browser.go`

**Purpose**: Browse and select backup archives from the backup directory

**Structure**:
```go
type ArchiveBrowserModel struct {
    config      *config.Config
    logger      logger.Logger
    parent      tea.Model
    archives    []ArchiveInfo
    cursor      int
    loading     bool
    err         error
    title       string
    mode        string // "restore-single", "restore-cluster", "manage"
    searchTerm  string
    filterType  string // "all", "postgres", "mysql", "cluster"
}

type ArchiveInfo struct {
    Name         string
    Path         string
    Format       restore.ArchiveFormat
    Size         int64
    Modified     time.Time
    DatabaseName string
    Valid        bool
    ValidationMsg string
}
```

**Features**:
- List all backup archives in backup directory
- Display archive details (format, size, date, database name)
- Filter by type (PostgreSQL dump, MySQL SQL, cluster)
- Search by filename or database name
- Sort by date (newest first) or name
- Show validation status (valid/corrupted)
- Preview archive contents
- Multi-column display with color coding

**Navigation**:
- ↑/↓: Navigate archives
- Enter: Select archive for restore
- p: Preview archive contents
- f: Toggle filter (all/postgres/mysql/cluster)
- /: Search mode
- Esc: Back to main menu
- d: Delete archive (with confirmation)
- i: Show archive info (detailed)

---

## Phase 3: Restore Preview Component

### 3.1 Create Restore Preview Model
**New File**: `internal/tui/restore_preview.go`

**Purpose**: Show what will be restored and safety checks before execution

**Structure**:
```go
type RestorePreviewModel struct {
    config       *config.Config
    logger       logger.Logger
    parent       tea.Model
    archivePath  string
    archiveInfo  ArchiveInfo
    targetDB     string
    options      RestoreOptions
    safetyChecks []SafetyCheck
    warnings     []string
    executing    bool
}

type RestoreOptions struct {
    CleanFirst      bool
    CreateIfMissing bool
    TargetDatabase  string // if different from original
    ForceOverwrite  bool
}

type SafetyCheck struct {
    Name     string
    Status   string // "pending", "checking", "passed", "failed", "warning"
    Message  string
    Critical bool
}
```

**Safety Checks**:
1. Archive integrity validation
2. Disk space verification (3x archive size)
3. Required tools check (pg_restore/psql/mysql)
4. Target database existence check
5. Data conflict warnings
6. Backup recommendation (backup existing data first)

**Display Sections**:
```
╭─ Restore Preview ────────────────────────────╮
│                                              │
│ Archive Information:                         │
│   File: mydb_20241107_120000.dump.gz        │
│   Format: PostgreSQL Dump (gzip)            │
│   Size: 2.4 GB                              │
│   Created: 2024-11-07 12:00:00              │
│   Database: mydb                            │
│                                              │
│ Target Information:                          │
│   Database: mydb_restored                   │
│   Host: localhost:5432                      │
│   Clean First: ✓ Yes                        │
│   Create If Missing: ✓ Yes                  │
│                                              │
│ Safety Checks:                               │
│   ✓ Archive integrity ... PASSED            │
│   ✓ Disk space (7.2 GB free) ... PASSED     │
│   ✓ Required tools ... PASSED               │
│   ⚠ Target database exists ... WARNING      │
│     Existing data will be dropped!          │
│                                              │
│ Warnings:                                    │
│   • Restoring will replace all data in      │
│     database 'mydb_restored'                │
│   • Recommendation: Create backup first     │
│                                              │
╰──────────────────────────────────────────────╯

Options:
  [t] Toggle clean-first  [c] Toggle create
  [n] Change target name  [b] Backup first
  [Enter] Proceed  [Esc] Cancel
```

---

## Phase 4: Restore Execution Component

### 4.1 Create Restore Execution Model
**New File**: `internal/tui/restore_exec.go`

**Purpose**: Execute restore with real-time progress feedback

**Structure**:
```go
type RestoreExecutionModel struct {
    config       *config.Config
    logger       logger.Logger
    parent       tea.Model
    archivePath  string
    targetDB     string
    cleanFirst   bool
    createIfMissing bool
    restoreType  string // "single" or "cluster"
    
    // Progress tracking
    status       string
    phase        string // "validating", "decompressing", "restoring", "verifying"
    progress     int
    details      []string
    startTime    time.Time
    
    // Results
    done         bool
    err          error
    result       string
    elapsed      time.Duration
}
```

**Progress Phases**:
1. **Validating**: Archive validation and safety checks
2. **Decompressing**: If compressed archive (.gz)
3. **Restoring**: Actual database restore operation
4. **Verifying**: Post-restore verification (optional)

**Display**:
```
╭─ Restoring Database ─────────────────────────╮
│                                              │
│ Phase: Restoring                             │
│ Status: Importing schema objects...          │
│                                              │
│ ████████████████░░░░░░░░░░░░  58%           │
│                                              │
│ Details:                                     │
│   • Archive validated successfully           │
│   • Decompressing with pigz (4 cores)       │
│   • Restored 2,451 tables                   │
│   • Restored 15,234 sequences               │
│   • Importing data...                       │
│                                              │
│ Elapsed: 2m 34s                             │
│                                              │
╰──────────────────────────────────────────────╯

Press Ctrl+C to cancel (with cleanup)
```

**Pattern**: Follow `backup_exec.go` architecture
- Use tea.Cmd for background execution
- Use tea.Tick for progress updates
- Handle cancellation with cleanup
- Show detailed progress from restore engine

---

## Phase 5: Cluster Restore Flow

### 5.1 Cluster Restore Preview
**Enhancement**: Extend `restore_preview.go` for cluster restores

**Additional Features**:
- List all databases in cluster archive
- Show extraction preview
- Database selection (restore all or subset)
- Sequential restore order
- Estimated time per database

**Display**:
```
╭─ Cluster Restore Preview ────────────────────╮
│                                              │
│ Archive: cluster_backup_20241107.tar.gz     │
│ Type: Full Cluster Backup                   │
│ Size: 15.8 GB                               │
│                                              │
│ Databases to Restore: (Select with Space)   │
│   [✓] db_production (2.4 GB)                │
│   [✓] db_analytics (8.1 GB)                 │
│   [✓] db_staging (1.2 GB)                   │
│   [✓] db_dev (0.8 GB)                       │
│   [ ] template_db (0.1 GB)                  │
│                                              │
│ Total Selected: 4 databases (12.5 GB)       │
│ Estimated Time: 45-60 minutes               │
│                                              │
│ Safety: All checks passed ✓                 │
│                                              │
╰──────────────────────────────────────────────╯

[Space] Select  [a] Select all  [Enter] Proceed
```

---

## Phase 6: Backup Management Component

### 6.1 Backup Management View
**New File**: `internal/tui/backup_manager.go`

**Purpose**: Comprehensive backup archive management

**Features**:
- View all backups with detailed information
- Sort by date, size, name, database
- Filter by type, database, date range
- Show disk usage statistics
- Archive operations:
  - Verify integrity
  - Delete with confirmation
  - Rename/move archives
  - Export metadata
  - Show detailed info
- Bulk operations (select multiple)

**Display**:
```
╭─ Backup Archive Manager ─────────────────────────────────────╮
│                                                               │
│ Total Archives: 23  |  Total Size: 87.4 GB  |  Free: 142 GB │
│                                                               │
│ Filter: [All Types ▼]  Sort: [Date (Newest) ▼]  Search: __  │
│                                                               │
│ DATE           DATABASE      SIZE    TYPE        STATUS      │
│ ────────────────────────────────────────────────────────────│
│ 2024-11-07 12:00  mydb       2.4 GB  PG Dump     ✓ Valid   │
│ 2024-11-07 08:00  cluster    15.8 GB Cluster     ✓ Valid   │
│ 2024-11-06 23:00  analytics   8.1 GB PG SQL      ✓ Valid   │
│ 2024-11-06 18:00  testdb      0.5 GB MySQL SQL   ✓ Valid   │
│ 2024-11-06 12:00  mydb       2.3 GB  PG Dump     ⚠ Old     │
│                                                               │
│ Selected: 0  |  [v] Verify  [d] Delete  [i] Info  [r] Restore│
│                                                               │
╰───────────────────────────────────────────────────────────────╯

Navigation: ↑/↓  Select: Space  Action: Enter  Back: Esc
```

---

## Phase 7: Integration Points

### 7.1 Restore Engine Integration
**File**: `internal/restore/engine.go`

Add TUI-friendly methods:
```go
// NewSilent creates restore engine without progress callbacks
func NewSilent(cfg *config.Config, log logger.Logger, db database.Database) *Engine

// RestoreSingleWithProgress with progress callback
func (e *Engine) RestoreSingleWithProgress(
    ctx context.Context,
    archivePath, targetDB string,
    cleanFirst, createIfMissing bool,
    progressCallback func(phase, status string, percent int),
) error
```

### 7.2 Progress Integration
**File**: `internal/progress/progress.go`

Add restore-specific phases:
```go
const (
    PhaseRestoreValidation  = "Validation"
    PhaseRestoreDecompress  = "Decompression"
    PhaseRestoreImport      = "Importing"
    PhaseRestoreVerify      = "Verification"
)
```

---

## Phase 8: User Experience Enhancements

### 8.1 Keyboard Shortcuts
Global shortcuts in all restore views:
- `?`: Show help overlay
- `Esc`: Back/Cancel
- `Ctrl+C`: Emergency abort
- `Tab`: Switch between panes (if multiple)

### 8.2 Visual Feedback
- Color coding:
  - 🟢 Green: Successful operations, valid archives
  - 🟡 Yellow: Warnings, old backups
  - 🔴 Red: Errors, failed checks, invalid archives
  - 🔵 Blue: Info, in-progress operations
  
- Icons:
  - ✓: Success/Valid
  - ✗: Failed/Invalid
  - ⚠: Warning
  - 🔍: Validating
  - 💾: Restoring
  - 📦: Archive
  - 🗄️: Database

### 8.3 Error Handling
- Graceful error recovery
- Clear error messages
- Suggested actions
- Automatic retry on transient failures
- Cleanup on cancellation

---

## Phase 9: Safety Features

### 9.1 Confirmation Dialogs
Use existing `confirmation.go` pattern for:
- Restore with data replacement
- Delete archives
- Cluster restore (all databases)
- Force operations (skip safety checks)

### 9.2 Dry-Run Mode
- Show preview by default
- Require explicit confirmation
- No accidental restores
- Show what would happen

### 9.3 Backup-Before-Restore
Optional feature:
- Offer to backup existing database before restore
- Quick backup with timestamp
- Automatic cleanup of temporary backups

---

## Phase 10: Testing Strategy

### 10.1 Unit Tests
- Archive detection and validation
- Format parsing
- Safety check logic
- Preview generation

### 10.2 Integration Tests
- Complete restore flow
- Cluster restore
- Error scenarios
- Cancellation handling

### 10.3 Manual Testing Scenarios
1. Restore single PostgreSQL dump
2. Restore compressed MySQL SQL
3. Restore full cluster
4. Cancel mid-restore
5. Restore with different target name
6. Restore with clean-first option
7. Handle corrupted archive
8. Handle insufficient disk space
9. Handle missing tools

---

## Implementation Order

### Priority 1 (Core Functionality)
1. ✅ Phase 1: Main menu integration
2. ✅ Phase 2: Archive browser
3. ✅ Phase 3: Restore preview
4. ✅ Phase 4: Restore execution

### Priority 2 (Enhanced Features)
5. Phase 5: Cluster restore flow
6. Phase 6: Backup management
7. Phase 7: Engine integration refinements

### Priority 3 (Polish)
8. Phase 8: UX enhancements
9. Phase 9: Safety features
10. Phase 10: Comprehensive testing

---

## File Structure Summary

```
internal/tui/
├── menu.go                    # MODIFY: Add restore menu items
├── archive_browser.go         # NEW: Browse and select archives
├── restore_preview.go         # NEW: Preview restore operation
├── restore_exec.go            # NEW: Execute restore with progress
├── backup_manager.go          # NEW: Comprehensive backup management
├── confirmation.go            # REUSE: For dangerous operations
├── backup_exec.go             # REFERENCE: Pattern for execution
└── dbselector.go              # REFERENCE: Pattern for selection

internal/restore/
├── engine.go                  # MODIFY: Add TUI-friendly methods
└── progress.go                # NEW: Restore progress tracking
```

---

## Estimated Implementation Time

- **Phase 1-4 (Core)**: 2-3 days
- **Phase 5-7 (Enhanced)**: 2-3 days
- **Phase 8-10 (Polish & Testing)**: 1-2 days
- **Total**: 5-8 days for complete implementation

---

## Success Criteria

✓ User can browse and select backup archives visually
✓ Restore preview shows all relevant information
✓ Safety checks prevent accidental data loss
✓ Progress feedback during restore operation
✓ Error handling with clear messages
✓ Cluster restore supports database selection
✓ Backup management provides archive maintenance
✓ Consistent UX with existing backup features
✓ No console output conflicts (silent operations)
✓ Proper cleanup on cancellation

---

## Notes

- Follow existing TUI patterns from `backup_exec.go` and `dbselector.go`
- Use `backup.NewSilent()` pattern for restore engine
- No `fmt.Println()` or direct console output
- All progress through TUI tea.Msg system
- Reuse existing styles and components where possible
- Maintain consistent keyboard navigation
- Add comprehensive error handling
- Support both PostgreSQL and MySQL restore
