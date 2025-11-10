# DBBackup Comprehensive Test Plan

**Date**: November 10, 2025  
**Goal**: Test all command-line and interactive functions to ensure stability

## Issues Found

### 🐛 BUG #1: `--create` flag not implemented in single database restore
**Severity**: HIGH  
**Location**: `internal/restore/engine.go` - `RestoreSingle()` function  
**Description**: The `createIfMissing` parameter is accepted but never used. Function doesn't create target database.  
**Expected**: When `--create` flag is used, should create database if it doesn't exist  
**Actual**: Fails with "database does not exist" error  
**Fix Required**: Add database creation logic before restore

---

## Test Matrix

### 1. BACKUP OPERATIONS

#### 1.1 Single Database Backup
| Test Case | Command | Expected Result | Status |
|-----------|---------|-----------------|--------|
| Basic backup | `backup single ownership_test` | ✅ Success | ✅ PASS |
| With compression level | `backup single ownership_test --compression=9` | Success with higher compression | ⏳ TODO |
| With custom backup dir | `backup single ownership_test --backup-dir=/tmp/backups` | Success, file in /tmp/backups | ⏳ TODO |
| Non-existent database | `backup single nonexistent_db` | Error: database does not exist | ⏳ TODO |
| Without permissions | `backup single postgres` (as non-postgres user) | Error: authentication failed | ⏳ TODO |

#### 1.2 Cluster Backup
| Test Case | Command | Expected Result | Status |
|-----------|---------|-----------------|--------|
| Basic cluster backup | `backup cluster` | ✅ Success, all DBs backed up | ✅ PASS |
| With parallel jobs | `backup cluster --jobs=4` | Success with 4 parallel jobs | ⏳ TODO |
| With custom compression | `backup cluster --compression=1` | Fast backup, larger file | ⏳ TODO |
| Disk space insufficient | `backup cluster` (low disk space) | Error: insufficient disk space | ⏳ TODO |

---

### 2. RESTORE OPERATIONS

#### 2.1 Single Database Restore
| Test Case | Command | Expected Result | Status |
|-----------|---------|-----------------|--------|
| Basic restore | `restore single backup.dump --target=test_db --confirm` | Database exists: Success | ⏳ TODO |
| With --create flag | `restore single backup.dump --target=new_db --create --confirm` | ❌ BUG: Database not created | ❌ **BUG #1** |
| With --clean flag | `restore single backup.dump --target=existing_db --clean --confirm` | Drop + Restore success | ⏳ TODO |
| Dry-run mode | `restore single backup.dump --target=test_db --dry-run` | Show plan, no changes | ⏳ TODO |
| Without --confirm | `restore single backup.dump --target=test_db` | Show dry-run automatically | ⏳ TODO |
| Non-existent file | `restore single /tmp/nonexistent.dump --confirm` | Error: file not found | ⏳ TODO |
| Corrupted archive | `restore single corrupted.dump --confirm` | Error: invalid archive | ⏳ TODO |
| Wrong format | `restore single cluster.tar.gz --confirm` | Error: not a single DB backup | ⏳ TODO |

#### 2.2 Cluster Restore
| Test Case | Command | Expected Result | Status |
|-----------|---------|-----------------|--------|
| Basic cluster restore | `restore cluster backup.tar.gz --confirm` | ✅ All DBs restored with ownership | ✅ PASS |
| Dry-run mode | `restore cluster backup.tar.gz --dry-run` | Show restore plan | ⏳ TODO |
| Without --confirm | `restore cluster backup.tar.gz` | Show dry-run automatically | ⏳ TODO |
| Non-superuser | `restore cluster backup.tar.gz --confirm` (non-superuser) | Warning: limited ownership | ⏳ TODO |
| Disk space insufficient | `restore cluster backup.tar.gz --confirm` | Error: insufficient disk space | ⏳ TODO |
| With --force flag | `restore cluster backup.tar.gz --force --confirm` | Skip safety checks | ⏳ TODO |

#### 2.3 Restore List
| Test Case | Command | Expected Result | Status |
|-----------|---------|-----------------|--------|
| List all backups | `restore list` | ✅ Show all backup files | ✅ PASS |
| List with filter | `restore list --database=test_db` | Show only test_db backups | ⏳ TODO |
| Empty backup dir | `restore list` (no backups) | Show "No backups found" | ⏳ TODO |

---

### 3. STATUS OPERATIONS

| Test Case | Command | Expected Result | Status |
|-----------|---------|-----------------|--------|
| Basic status | `status` | Show database list | ⏳ TODO |
| Detailed status | `status --detailed` | Show with sizes/counts | ⏳ TODO |
| JSON format | `status --format=json` | Valid JSON output | ⏳ TODO |
| Filter by database | `status --database=test_db` | Show only test_db | ⏳ TODO |

---

### 4. INTERACTIVE MODE (TUI)

#### 4.1 Main Menu Navigation
| Test Case | Action | Expected Result | Status |
|-----------|--------|-----------------|--------|
| Launch TUI | `./dbbackup` or `./dbbackup interactive` | Show main menu | ⏳ TODO |
| Navigate with arrows | Up/Down arrows | Highlight moves | ⏳ TODO |
| Select with Enter | Enter key on option | Execute option | ⏳ TODO |
| Quit with q | Press 'q' | Exit gracefully | ⏳ TODO |
| Ctrl+C handling | Press Ctrl+C | Exit gracefully | ⏳ TODO |

#### 4.2 Backup Single Database (TUI)
| Test Case | Action | Expected Result | Status |
|-----------|--------|-----------------|--------|
| Select database | Choose from list | Database selected | ⏳ TODO |
| Confirm backup | Confirm dialog | Backup starts | ⏳ TODO |
| Cancel backup | Cancel dialog | Return to menu | ⏳ TODO |
| Progress display | During backup | Show progress bar | ⏳ TODO |
| Completion message | After success | Show success message | ⏳ TODO |
| Error handling | Backup fails | Show error, don't crash | ⏳ TODO |

#### 4.3 Backup Cluster (TUI)
| Test Case | Action | Expected Result | Status |
|-----------|--------|-----------------|--------|
| Confirm cluster backup | Confirm dialog | Backup all DBs | ⏳ TODO |
| Progress tracking | During backup | Show N/M databases | ⏳ TODO |
| ETA display | Long operation | Show time remaining | ⏳ TODO |
| Error on one DB | One DB fails | Continue with others | ⏳ TODO |

#### 4.4 Restore Single Database (TUI)
| Test Case | Action | Expected Result | Status |
|-----------|--------|-----------------|--------|
| Select archive | Choose from list | Archive selected | ⏳ TODO |
| Select target DB | Choose/enter name | Target selected | ⏳ TODO |
| Confirm restore | Confirm dialog | Restore starts | ⏳ TODO |
| Cancel restore | Cancel dialog | Return to menu | ⏳ TODO |
| Progress display | During restore | Show progress | ⏳ TODO |

#### 4.5 Restore Cluster (TUI)
| Test Case | Action | Expected Result | Status |
|-----------|--------|-----------------|--------|
| Select cluster archive | Choose from list | Archive selected | ⏳ TODO |
| Confirm restore | Warning + confirm | Restore starts | ⏳ TODO |
| Progress tracking | During restore | Show N/M databases | ⏳ TODO |
| Ownership preservation | After restore | Verify owners correct | ⏳ TODO |

#### 4.6 View Status (TUI)
| Test Case | Action | Expected Result | Status |
|-----------|--------|-----------------|--------|
| Show all databases | Select status option | List all DBs | ⏳ TODO |
| Scroll long lists | Arrow keys | Scroll works | ⏳ TODO |
| Return to menu | Back/Escape | Return to main menu | ⏳ TODO |

#### 4.7 Operation History (TUI)
| Test Case | Action | Expected Result | Status |
|-----------|--------|-----------------|--------|
| View history | Select history option | Show past operations | ⏳ TODO |
| Navigate history | Up/Down arrows | Scroll through history | ⏳ TODO |
| Long operations | View 2-hour backup | Show correct duration | ⏳ TODO |
| Empty history | First run | Show "No history" | ⏳ TODO |

---

### 5. EDGE CASES & ERROR SCENARIOS

#### 5.1 Authentication & Permissions
| Test Case | Scenario | Expected Result | Status |
|-----------|----------|-----------------|--------|
| Peer auth mismatch | Run as root with --user postgres | Show warning, suggest sudo | ⏳ TODO |
| No password | Connect with peer auth | Success via Unix socket | ⏳ TODO |
| Wrong password | Connect with bad password | Error: authentication failed | ⏳ TODO |
| No .pgpass file | Remote connection | Prompt for password | ⏳ TODO |

#### 5.2 Disk Space
| Test Case | Scenario | Expected Result | Status |
|-----------|----------|-----------------|--------|
| Insufficient space | Backup when disk full | Error before starting | ⏳ TODO |
| Restore needs space | Restore 10GB with 5GB free | Error with clear message | ⏳ TODO |

#### 5.3 Database State
| Test Case | Scenario | Expected Result | Status |
|-----------|----------|-----------------|--------|
| Active connections | Restore DB with connections | Terminate connections first | ⏳ TODO |
| Database locked | Backup locked DB | Wait or error gracefully | ⏳ TODO |
| Corrupted database | Backup corrupted DB | Warning but continue | ⏳ TODO |

#### 5.4 Network & Remote
| Test Case | Scenario | Expected Result | Status |
|-----------|----------|-----------------|--------|
| Remote host | `--host=remote.server` | Connect via TCP | ⏳ TODO |
| Network timeout | Connection hangs | Timeout with error | ⏳ TODO |
| SSL required | `--ssl-mode=require` | Connect with SSL | ⏳ TODO |

---

### 6. CROSS-PLATFORM COMPATIBILITY

| Platform | Test | Status |
|----------|------|--------|
| Linux amd64 | All operations | ⏳ TODO |
| Linux arm64 | All operations | ⏳ TODO |
| macOS Intel | All operations | ⏳ TODO |
| macOS ARM | All operations | ⏳ TODO |
| Windows amd64 | All operations | ⏳ TODO |

---

## Testing Procedure

### Phase 1: Command-Line Testing (Current)
1. ✅ Test basic backup single
2. ✅ Test basic backup cluster  
3. ✅ Test restore list
4. ❌ **Found BUG #1**: restore single --create not working
5. ⏳ Test remaining command-line operations

### Phase 2: Fix Critical Bugs
1. Fix BUG #1: Implement database creation in RestoreSingle
2. Test fix thoroughly
3. Commit and push

### Phase 3: Interactive Mode Testing
1. Test all TUI menu options
2. Test navigation and user input
3. Test progress indicators
4. Test error handling

### Phase 4: Edge Case Testing
1. Test authentication scenarios
2. Test disk space limits
3. Test error conditions
4. Test concurrent operations

### Phase 5: Documentation & Report
1. Update README with known issues
2. Create bug fix changelog
3. Update user documentation
4. Final stability report

---

## Success Criteria

- ✅ All critical bugs fixed
- ✅ All command-line operations work as documented
- ✅ All interactive operations complete successfully
- ✅ Error messages are clear and helpful
- ✅ No crashes or panics under normal use
- ✅ Cross-platform builds succeed
- ✅ Documentation matches actual behavior

---

## Notes

- Test with PostgreSQL peer authentication (most common setup)
- Test with MariaDB/MySQL (ensure no regressions)
- Test with both superuser and regular user accounts
- Test with various database sizes (small, medium, large)
- Test with special characters in database names

---

**Last Updated**: November 10, 2025  
**Test Progress**: 3/100+ tests completed  
**Critical Bugs Found**: 1  
**Status**: IN PROGRESS
