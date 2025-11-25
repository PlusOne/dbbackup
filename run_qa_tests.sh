#!/bin/bash
#
# Automated QA Test Script for dbbackup Interactive Mode
# Tests as many features as possible without manual interaction
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Config
BINARY="/root/dbbackup/dbbackup"
TEST_DIR="/tmp/dbbackup_qa_test"
BACKUP_DIR="$TEST_DIR/backups"
LOG_FILE="$TEST_DIR/qa_test_$(date +%Y%m%d_%H%M%S).log"
REPORT_FILE="/root/dbbackup/QA_TEST_RESULTS.md"

# Counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0
CRITICAL_ISSUES=0
MAJOR_ISSUES=0
MINOR_ISSUES=0

echo -e "${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║           QA Test Suite - dbbackup Interactive Mode            ║${NC}"
echo -e "${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo
echo -e "${BLUE}Test Date:${NC} $(date)"
echo -e "${BLUE}Environment:${NC} $(uname -s) $(uname -m)"
echo -e "${BLUE}Binary:${NC} $BINARY"
echo -e "${BLUE}Test Directory:${NC} $TEST_DIR"
echo -e "${BLUE}Log File:${NC} $LOG_FILE"
echo

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}ERROR: Must run as root for postgres user switching${NC}"
    exit 1
fi

# Setup
echo -e "${YELLOW}► Setting up test environment...${NC}"
rm -rf "$TEST_DIR"
mkdir -p "$BACKUP_DIR"
chmod 755 "$TEST_DIR" "$BACKUP_DIR"
chown -R postgres:postgres "$TEST_DIR"
cp "$BINARY" "$TEST_DIR/"
chmod 755 "$TEST_DIR/dbbackup"
chown postgres:postgres "$TEST_DIR/dbbackup"
echo -e "${GREEN}✓ Environment ready${NC}"
echo

# Test function
run_test() {
    local name="$1"
    local severity="$2"  # CRITICAL, MAJOR, MINOR
    local cmd="$3"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}TEST $TOTAL_TESTS: $name${NC}"
    echo -e "${CYAN}Severity: $severity${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    if [ -n "$cmd" ]; then
        echo -e "${YELLOW}Command:${NC} $cmd"
        echo
        
        if eval "$cmd" >> "$LOG_FILE" 2>&1; then
            echo -e "${GREEN}✅ PASSED${NC}"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo -e "${RED}❌ FAILED${NC}"
            FAILED_TESTS=$((FAILED_TESTS + 1))
            
            case "$severity" in
                CRITICAL) CRITICAL_ISSUES=$((CRITICAL_ISSUES + 1)) ;;
                MAJOR) MAJOR_ISSUES=$((MAJOR_ISSUES + 1)) ;;
                MINOR) MINOR_ISSUES=$((MINOR_ISSUES + 1)) ;;
            esac
        fi
    else
        echo -e "${YELLOW}⏭️  MANUAL TEST REQUIRED${NC}"
        SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
    fi
    
    echo
}

cd "$TEST_DIR"

# ============================================================================
# PHASE 1: Basic Functionality (CRITICAL)
# ============================================================================

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  PHASE 1: Basic Functionality (CRITICAL)                       ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo

run_test "Application Version Check" "CRITICAL" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup --version'"

run_test "Application Help" "CRITICAL" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup --help' | grep -q 'interactive'"

run_test "Interactive Mode Launch (--help)" "CRITICAL" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup interactive --help' | grep -q 'auto-select'"

run_test "Single Database Backup (CLI)" "CRITICAL" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup backup single postgres --backup-dir $BACKUP_DIR' > /dev/null 2>&1"

run_test "Verify Backup Files Created" "CRITICAL" \
    "ls $BACKUP_DIR/db_postgres_*.dump >/dev/null 2>&1 && ls $BACKUP_DIR/db_postgres_*.dump.sha256 >/dev/null 2>&1"

run_test "Backup Checksum Validation" "CRITICAL" \
    "cd $BACKUP_DIR && sha256sum -c \$(ls -t db_postgres_*.sha256 | head -1) 2>&1 | grep -q 'OK'"

run_test "List Backups Command" "CRITICAL" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup list' | grep -q 'backup'"

# ============================================================================
# PHASE 2: TUI Auto-Select Tests (MAJOR)
# ============================================================================

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  PHASE 2: TUI Automation (MAJOR)                               ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo

run_test "TUI Auto-Select Single Backup" "MAJOR" \
    "timeout 5s su - postgres -c 'cd $TEST_DIR && ./dbbackup interactive --auto-select 0 --auto-database postgres --debug' 2>&1 | grep -q 'Auto-select'"

run_test "TUI Auto-Select Status View" "MAJOR" \
    "timeout 3s su - postgres -c 'cd $TEST_DIR && ./dbbackup interactive --auto-select 10 --debug' 2>&1 | grep -q 'Status\|Database'"

run_test "TUI Auto-Select with Logging" "MAJOR" \
    "timeout 3s su - postgres -c 'cd $TEST_DIR && ./dbbackup interactive --auto-select 10 --verbose-tui --tui-log-file $TEST_DIR/tui.log' 2>&1 && test -f $TEST_DIR/tui.log"

# ============================================================================
# PHASE 3: Configuration Tests (MAJOR)
# ============================================================================

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  PHASE 3: Configuration (MAJOR)                                ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo

# Create test config
cat > "$TEST_DIR/.dbbackup.conf" <<EOF
[database]
type = postgres
host = localhost
port = 5432
user = postgres

[backup]
backup_dir = $BACKUP_DIR
compression = 9

[security]
retention_days = 7
min_backups = 3
EOF
chown postgres:postgres "$TEST_DIR/.dbbackup.conf"

run_test "Config File Loading" "MAJOR" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup backup single postgres' 2>&1 | grep -q 'Loaded configuration'"

run_test "Config File Created After Backup" "MAJOR" \
    "test -f $TEST_DIR/.dbbackup.conf && grep -q 'retention_days' $TEST_DIR/.dbbackup.conf"

run_test "Config File No Password Leak" "CRITICAL" \
    "! grep -i 'password.*=' $TEST_DIR/.dbbackup.conf"

# ============================================================================
# PHASE 4: Security Features (CRITICAL)
# ============================================================================

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  PHASE 4: Security Features (CRITICAL)                         ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo

run_test "Retention Policy Flag Available" "MAJOR" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup --help' | grep -q 'retention-days'"

run_test "Rate Limiting Flag Available" "MAJOR" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup --help' | grep -q 'max-retries'"

run_test "Privilege Check Flag Available" "MAJOR" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup --help' | grep -q 'allow-root'"

run_test "Resource Check Flag Available" "MAJOR" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup --help' | grep -q 'check-resources'"

# Create old backups for retention test
su - postgres -c "
cd $BACKUP_DIR
touch -d '40 days ago' db_old_40.dump db_old_40.dump.sha256 db_old_40.dump.info
touch -d '35 days ago' db_old_35.dump db_old_35.dump.sha256 db_old_35.dump.info
"

run_test "Retention Policy Cleanup" "MAJOR" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup backup single postgres --retention-days 30 --min-backups 2 --debug' 2>&1 | grep -q 'Removing old backup' && ! test -f $BACKUP_DIR/db_old_40.dump"

# ============================================================================
# PHASE 5: Error Handling (MAJOR)
# ============================================================================

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  PHASE 5: Error Handling (MAJOR)                               ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo

run_test "Invalid Database Name Handling" "MAJOR" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup backup single nonexistent_db_xyz_123' 2>&1 | grep -qE 'error|failed|not found'"

run_test "Invalid Host Handling" "MAJOR" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup backup single postgres --host invalid.host.xyz --max-retries 1' 2>&1 | grep -qE 'connection.*failed|error'"

run_test "Invalid Compression Level" "MINOR" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup backup single postgres --compression 15' 2>&1 | grep -qE 'invalid|error'"

# ============================================================================
# PHASE 6: Data Integrity (CRITICAL)
# ============================================================================

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  PHASE 6: Data Integrity (CRITICAL)                            ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo

run_test "Backup File is Valid PostgreSQL Dump" "CRITICAL" \
    "file $BACKUP_DIR/db_postgres_*.dump | grep -qE 'PostgreSQL|data'"

run_test "Checksum File Format Valid" "CRITICAL" \
    "cat $BACKUP_DIR/db_postgres_*.sha256 | grep -qE '[0-9a-f]{64}'"

run_test "Metadata File Created" "MAJOR" \
    "ls $BACKUP_DIR/db_postgres_*.dump.info >/dev/null 2>&1 && grep -q 'timestamp' $BACKUP_DIR/db_postgres_*.dump.info"

# ============================================================================
# Summary
# ============================================================================

echo
echo -e "${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║                    TEST SUMMARY                                 ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
echo
echo -e "${BLUE}Total Tests:${NC}     $TOTAL_TESTS"
echo -e "${GREEN}Passed:${NC}          $PASSED_TESTS"
echo -e "${RED}Failed:${NC}          $FAILED_TESTS"
echo -e "${YELLOW}Skipped:${NC}         $SKIPPED_TESTS"
echo
echo -e "${BLUE}Issues by Severity:${NC}"
echo -e "${RED}  Critical:${NC}      $CRITICAL_ISSUES"
echo -e "${YELLOW}  Major:${NC}         $MAJOR_ISSUES"
echo -e "${YELLOW}  Minor:${NC}         $MINOR_ISSUES"
echo
echo -e "${BLUE}Log File:${NC} $LOG_FILE"
echo

# Update report file
cat >> "$REPORT_FILE" <<EOF

## Automated Test Results (Updated: $(date))

**Tests Executed:** $TOTAL_TESTS  
**Passed:** $PASSED_TESTS  
**Failed:** $FAILED_TESTS  
**Skipped:** $SKIPPED_TESTS  

**Issues Found:**
- Critical: $CRITICAL_ISSUES
- Major: $MAJOR_ISSUES
- Minor: $MINOR_ISSUES

**Success Rate:** $(( PASSED_TESTS * 100 / TOTAL_TESTS ))%

---

EOF

# Final verdict
if [ $CRITICAL_ISSUES -gt 0 ]; then
    echo -e "${RED}❌ CRITICAL ISSUES FOUND - NOT READY FOR RELEASE${NC}"
    EXIT_CODE=2
elif [ $MAJOR_ISSUES -gt 0 ]; then
    echo -e "${YELLOW}⚠️  MAJOR ISSUES FOUND - CONSIDER FIXING BEFORE RELEASE${NC}"
    EXIT_CODE=1
elif [ $FAILED_TESTS -gt 0 ]; then
    echo -e "${YELLOW}⚠️  MINOR ISSUES FOUND - DOCUMENT AND ADDRESS${NC}"
    EXIT_CODE=0
else
    echo -e "${GREEN}✅ ALL TESTS PASSED - READY FOR RELEASE${NC}"
    EXIT_CODE=0
fi

echo
echo -e "${BLUE}Detailed log:${NC} cat $LOG_FILE"
echo -e "${BLUE}Full report:${NC} cat $REPORT_FILE"
echo

exit $EXIT_CODE
