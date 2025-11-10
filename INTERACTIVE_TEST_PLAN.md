# Interactive Mode (TUI) Test Plan

**Date**: November 10, 2025  
**Goal**: Test all TUI functionality systematically

## Test Execution Plan

### Phase 1: Basic Navigation & Menu
1. Launch TUI
2. Navigate menu with arrows
3. Test all menu options
4. Test quit/exit functionality

### Phase 2: Database Operations
1. Backup single database
2. Backup cluster
3. Restore single database
4. Restore cluster
5. View status

### Phase 3: Operation History
1. View history viewport
2. Navigate long history
3. Verify timestamps and durations
4. Test with various operation types

### Phase 4: Error Handling
1. Test invalid inputs
2. Test cancelled operations
3. Test disk space errors
4. Test authentication errors

## Test Checklist

- [ ] TUI launches without errors
- [ ] Main menu displays correctly
- [ ] Arrow keys navigate properly
- [ ] Enter key selects options
- [ ] 'q' key quits gracefully
- [ ] Ctrl+C exits cleanly
- [ ] Database list displays
- [ ] Backup progress shows correctly
- [ ] Restore progress shows correctly
- [ ] Operation history works
- [ ] History navigation smooth
- [ ] Error messages clear
- [ ] No crashes or panics
- [ ] Color output works
- [ ] Status information correct

## Interactive Testing Script

```bash
# Test 1: Launch TUI
sudo -u postgres ./dbbackup

# Test 2: Backup single database
# Select: Backup Single Database
# Choose: ownership_test
# Confirm

# Test 3: Backup cluster
# Select: Backup Cluster
# Confirm

# Test 4: Restore single
# Select: Restore Single Database
# Choose backup file
# Confirm

# Test 5: View status
# Select: View Database Status
# Verify all databases shown

# Test 6: View history
# Select: View Operation History
# Navigate with arrows
# Verify timestamps correct

# Test 7: Quit
# Press 'q'
# Verify clean exit
```

## Expected Behaviors

### Main Menu
```
┌─ Database Backup & Recovery Tool ─────────────────────────────┐
│                                                                │
│  1. Backup Single Database                                    │
│  2. Backup Cluster                                            │
│  3. Restore Single Database                                   │
│  4. Restore Cluster                                           │
│  5. View Database Status                                      │
│  6. View Operation History                                    │
│  7. Quit                                                      │
│                                                                │
│  Use ↑↓ to navigate, Enter to select, q to quit              │
└────────────────────────────────────────────────────────────────┘
```

### Progress Indicator
```
🔄 Backing up database 'ownership_test'
   [████████████████████░░░░░░░░] 75% | Elapsed: 2s | ETA: ~1s
```

### Operation History
```
┌─ Operation History ───────────────────────────────────────────┐
│                                                                │
│  ✅ Cluster Backup - 12:34:56 - Duration: 5.3s               │
│  ✅ Single Backup (ownership_test) - 12:30:45 - Duration: 0.1s│
│  ✅ Cluster Restore - 12:25:30 - Duration: 12.3s             │
│  ❌ Single Restore (test_db) - 12:20:15 - FAILED             │
│                                                                │
│  Use ↑↓ to scroll, ESC to return                             │
└────────────────────────────────────────────────────────────────┘
```

## Issues to Watch For

1. **Menu rendering glitches**
2. **Progress bar flickering**
3. **History viewport scrolling issues**
4. **Color rendering problems**
5. **Keyboard input lag**
6. **Memory leaks (long operations)**
7. **Terminal size handling**
8. **Ctrl+C during operations**

## Test Results

| Component | Status | Notes |
|-----------|--------|-------|
| Main Menu | ⏳ | To be tested |
| Navigation | ⏳ | To be tested |
| Backup Single | ⏳ | To be tested |
| Backup Cluster | ⏳ | To be tested |
| Restore Single | ⏳ | To be tested |
| Restore Cluster | ⏳ | To be tested |
| View Status | ⏳ | To be tested |
| Operation History | ⏳ | To be tested |
| Error Handling | ⏳ | To be tested |
| Exit/Quit | ⏳ | To be tested |

---

**Next**: Start manual interactive testing session
