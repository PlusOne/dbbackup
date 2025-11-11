#!/bin/bash
################################################################################
# Master Test Execution Script
# Automated testing for dbbackup command-line interface
################################################################################

set -e
set -o pipefail

# Configuration
DBBACKUP="./dbbackup"
TEST_DIR="/tmp/dbbackup_master_test_$$"
TEST_DB="postgres"
POSTGRES_USER="postgres"
LOG_FILE="/tmp/dbbackup_master_test_$(date +%Y%m%d_%H%M%S).log"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0
declare -a FAILED_TESTS

# Helper functions
log() {
    echo "$@" | tee -a "$LOG_FILE"
}

test_start() {
    TESTS_RUN=$((TESTS_RUN + 1))
    echo -ne "${YELLOW}[TEST $TESTS_RUN]${NC} $1 ... "
}

test_pass() {
    TESTS_PASSED=$((TESTS_PASSED + 1))
    echo -e "${GREEN}PASS${NC}"
    [ -n "$1" ] && echo "         ↳ $1"
}

test_fail() {
    TESTS_FAILED=$((TESTS_FAILED + 1))
    FAILED_TESTS+=("TEST $TESTS_RUN: $1")
    echo -e "${RED}FAIL${NC}"
    [ -n "$1" ] && echo "         ↳ $1"
}

run_as_postgres() {
    if [ "$(whoami)" = "postgres" ]; then
        "$@"
    else
        sudo -u postgres "$@"
    fi
}

cleanup() {
    rm -rf "$TEST_DIR" 2>/dev/null || true
}

init_test_env() {
    mkdir -p "$TEST_DIR"
    log "Test directory: $TEST_DIR"
    log "Log file: $LOG_FILE"
    log ""
}

################################################################################
# Test Functions
################################################################################

test_binary_exists() {
    test_start "Binary exists"
    if [ -f "$DBBACKUP" ] && [ -x "$DBBACKUP" ]; then
        test_pass "$(ls -lh $DBBACKUP | awk '{print $5}')"
    else
        test_fail "Binary not found or not executable"
        exit 1
    fi
}

test_help_commands() {
    test_start "Help command"
    if run_as_postgres $DBBACKUP --help >/dev/null 2>&1; then
        test_pass
    else
        test_fail
    fi
    
    test_start "Version command"
    if run_as_postgres $DBBACKUP --version >/dev/null 2>&1; then
        version=$(run_as_postgres $DBBACKUP --version 2>&1 | head -1)
        test_pass "$version"
    else
        test_fail
    fi
    
    test_start "Backup help"
    if run_as_postgres $DBBACKUP backup --help >/dev/null 2>&1; then
        test_pass
    else
        test_fail
    fi
    
    test_start "Restore help"
    if run_as_postgres $DBBACKUP restore --help >/dev/null 2>&1; then
        test_pass
    else
        test_fail
    fi
}

test_status_commands() {
    test_start "Status host"
    if run_as_postgres $DBBACKUP status host -d postgres --insecure >>"$LOG_FILE" 2>&1; then
        test_pass
    else
        test_fail
    fi
    
    test_start "Status CPU"
    if $DBBACKUP status cpu >>"$LOG_FILE" 2>&1; then
        test_pass
    else
        test_fail
    fi
}

test_single_backup() {
    local compress=$1
    local desc=$2
    
    test_start "Single backup (compression=$compress) $desc"
    local backup_dir="$TEST_DIR/single_c${compress}"
    mkdir -p "$backup_dir"
    
    if run_as_postgres timeout 120 $DBBACKUP backup single $TEST_DB -d postgres --insecure \
        --backup-dir "$backup_dir" --compression $compress >>"$LOG_FILE" 2>&1; then
        
        local backup_file=$(ls "$backup_dir"/db_${TEST_DB}_*.dump 2>/dev/null | head -1)
        if [ -n "$backup_file" ]; then
            local size=$(ls -lh "$backup_file" | awk '{print $5}')
            test_pass "$size"
        else
            test_fail "Backup file not found"
        fi
    else
        test_fail "Backup command failed"
    fi
}

test_cluster_backup() {
    test_start "Cluster backup (all databases)"
    local backup_dir="$TEST_DIR/cluster"
    mkdir -p "$backup_dir"
    
    if run_as_postgres timeout 300 $DBBACKUP backup cluster -d postgres --insecure \
        --backup-dir "$backup_dir" --compression 3 >>"$LOG_FILE" 2>&1; then
        
        local archive=$(ls "$backup_dir"/cluster_*.tar.gz 2>/dev/null | head -1)
        if [ -n "$archive" ]; then
            local size=$(ls -lh "$archive" | awk '{print $5}')
            local archive_size=$(stat -c%s "$archive" 2>/dev/null || stat -f%z "$archive" 2>/dev/null)
            
            if [ "$archive_size" -gt 1000 ]; then
                test_pass "$size"
            else
                test_fail "Archive is empty or too small"
            fi
        else
            test_fail "Cluster archive not found"
        fi
    else
        test_fail "Cluster backup failed"
    fi
}

test_restore_single() {
    test_start "Single database restore with --create"
    
    # First create a backup
    local backup_dir="$TEST_DIR/restore_test"
    mkdir -p "$backup_dir"
    
    if run_as_postgres $DBBACKUP backup single $TEST_DB -d postgres --insecure \
        --backup-dir "$backup_dir" --compression 1 >>"$LOG_FILE" 2>&1; then
        
        local backup_file=$(ls "$backup_dir"/db_${TEST_DB}_*.dump 2>/dev/null | head -1)
        if [ -n "$backup_file" ]; then
            local restore_db="master_test_restore_$$"
            
            if run_as_postgres timeout 120 $DBBACKUP restore single "$backup_file" \
                --target-db "$restore_db" -d postgres --insecure --create >>"$LOG_FILE" 2>&1; then
                
                # Check if database exists
                if run_as_postgres psql -lqt | cut -d \| -f 1 | grep -qw "$restore_db"; then
                    test_pass "Database restored"
                    # Cleanup
                    run_as_postgres psql -d postgres -c "DROP DATABASE IF EXISTS $restore_db" >>"$LOG_FILE" 2>&1
                else
                    test_fail "Restored database not found"
                fi
            else
                test_fail "Restore failed"
            fi
        else
            test_fail "Backup file not found"
        fi
    else
        test_fail "Initial backup failed"
    fi
}

test_compression_levels() {
    log ""
    log "=== Compression Level Tests ==="
    
    declare -A sizes
    for level in 1 6 9; do
        test_start "Compression level $level"
        local backup_dir="$TEST_DIR/compress_$level"
        mkdir -p "$backup_dir"
        
        if run_as_postgres timeout 120 $DBBACKUP backup single $TEST_DB -d postgres --insecure \
            --backup-dir "$backup_dir" --compression $level >>"$LOG_FILE" 2>&1; then
            
            local backup_file=$(ls "$backup_dir"/db_${TEST_DB}_*.dump 2>/dev/null | head -1)
            if [ -n "$backup_file" ]; then
                local size=$(stat -c%s "$backup_file" 2>/dev/null || stat -f%z "$backup_file" 2>/dev/null)
                local size_mb=$((size / 1024 / 1024))
                sizes[$level]=$size
                test_pass "${size_mb}MB"
            else
                test_fail "Backup not found"
            fi
        else
            test_fail "Backup failed"
        fi
    done
    
    # Verify compression works (level 1 > level 9)
    if [ ${sizes[1]:-0} -gt ${sizes[9]:-0} ]; then
        test_start "Compression efficiency check"
        test_pass "Level 1 (${sizes[1]} bytes) > Level 9 (${sizes[9]} bytes)"
    else
        test_start "Compression efficiency check"
        test_fail "Compression levels don't show expected size difference"
    fi
}

test_large_database() {
    # Check if testdb_50gb exists
    if run_as_postgres psql -lqt | cut -d \| -f 1 | grep -qw "testdb_50gb"; then
        test_start "Large database streaming compression"
        local backup_dir="$TEST_DIR/large_db"
        mkdir -p "$backup_dir"
        
        if run_as_postgres timeout 600 $DBBACKUP backup single testdb_50gb -d postgres --insecure \
            --backup-dir "$backup_dir" --compression 1 >>"$LOG_FILE" 2>&1; then
            
            local backup_file=$(ls "$backup_dir"/db_testdb_50gb_*.dump 2>/dev/null | head -1)
            if [ -n "$backup_file" ]; then
                local size=$(stat -c%s "$backup_file" 2>/dev/null || stat -f%z "$backup_file" 2>/dev/null)
                local size_mb=$((size / 1024 / 1024))
                
                # Verify it's compressed (should be < 2GB for 7.3GB database)
                if [ $size_mb -lt 2000 ]; then
                    test_pass "${size_mb}MB - streaming compression worked"
                else
                    test_fail "${size_mb}MB - too large, streaming compression may have failed"
                fi
            else
                test_fail "Backup file not found"
            fi
        else
            test_fail "Large database backup failed or timed out"
        fi
    else
        test_start "Large database test"
        echo -e "${YELLOW}SKIP${NC} (testdb_50gb not available)"
    fi
}

test_invalid_inputs() {
    test_start "Invalid database name"
    if run_as_postgres $DBBACKUP backup single nonexistent_db_12345 -d postgres --insecure \
        --backup-dir "$TEST_DIR" 2>&1 | grep -qi "error\|not exist\|failed"; then
        test_pass "Error properly reported"
    else
        test_fail "No error for invalid database"
    fi
    
    test_start "Missing backup file"
    if run_as_postgres $DBBACKUP restore single /nonexistent/file.dump -d postgres --insecure \
        2>&1 | grep -qi "error\|not found\|failed"; then
        test_pass "Error properly reported"
    else
        test_fail "No error for missing file"
    fi
}

################################################################################
# Main Execution
################################################################################

main() {
    echo "================================================"
    echo "dbbackup Master Test Suite - CLI Automation"
    echo "================================================"
    echo "Started: $(date)"
    echo ""
    
    init_test_env
    
    echo "=== Pre-Flight Checks ==="
    test_binary_exists
    
    echo ""
    echo "=== Basic Command Tests ==="
    test_help_commands
    test_status_commands
    
    echo ""
    echo "=== Backup Tests ==="
    test_single_backup 1 "(fast)"
    test_single_backup 6 "(default)"
    test_single_backup 9 "(best)"
    test_cluster_backup
    
    echo ""
    echo "=== Restore Tests ==="
    test_restore_single
    
    echo ""
    echo "=== Advanced Tests ==="
    test_compression_levels
    test_large_database
    
    echo ""
    echo "=== Error Handling Tests ==="
    test_invalid_inputs
    
    echo ""
    echo "================================================"
    echo "Test Summary"
    echo "================================================"
    echo "Total Tests: $TESTS_RUN"
    echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
    
    if [ $TESTS_FAILED -gt 0 ]; then
        echo ""
        echo -e "${RED}Failed Tests:${NC}"
        for failed in "${FAILED_TESTS[@]}"; do
            echo -e "  ${RED}✗${NC} $failed"
        done
    fi
    
    echo ""
    echo "Log file: $LOG_FILE"
    echo "Completed: $(date)"
    echo ""
    
    # Calculate success rate
    if [ $TESTS_RUN -gt 0 ]; then
        success_rate=$((TESTS_PASSED * 100 / TESTS_RUN))
        echo "Success Rate: ${success_rate}%"
        
        if [ $success_rate -ge 95 ]; then
            echo -e "${GREEN}✅ EXCELLENT - Production Ready${NC}"
            exit_code=0
        elif [ $success_rate -ge 80 ]; then
            echo -e "${YELLOW}⚠️  GOOD - Minor issues need attention${NC}"
            exit_code=1
        else
            echo -e "${RED}❌ POOR - Significant issues found${NC}"
            exit_code=2
        fi
    fi
    
    # Cleanup
    echo ""
    echo "Cleaning up test files..."
    cleanup
    
    exit $exit_code
}

# Trap cleanup on exit
trap cleanup EXIT INT TERM

# Run main
main "$@"
