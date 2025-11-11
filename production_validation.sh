#!/bin/bash

################################################################################
# Production Validation Script for dbbackup
# 
# This script performs comprehensive testing of all CLI commands and validates
# the system is ready for production release.
#
# Requirements:
# - PostgreSQL running locally with test databases
# - Disk space for backups
# - Run as user with sudo access or as postgres user
################################################################################

set -e  # Exit on error
set -o pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
TESTS_TOTAL=0
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Configuration
DBBACKUP_BIN="./dbbackup"
TEST_BACKUP_DIR="/tmp/dbbackup_validation_$(date +%s)"
TEST_DB="postgres"
POSTGRES_USER="postgres"
LOG_FILE="/tmp/dbbackup_validation_$(date +%Y%m%d_%H%M%S).log"

# Test results
declare -a FAILED_TESTS=()

################################################################################
# Helper Functions
################################################################################

print_header() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

print_test() {
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    echo -e "${YELLOW}[TEST $TESTS_TOTAL]${NC} $1"
}

print_success() {
    TESTS_PASSED=$((TESTS_PASSED + 1))
    echo -e "  ${GREEN}✅ PASS${NC}: $1"
}

print_failure() {
    TESTS_FAILED=$((TESTS_FAILED + 1))
    FAILED_TESTS+=("$TESTS_TOTAL: $1")
    echo -e "  ${RED}❌ FAIL${NC}: $1"
}

print_skip() {
    TESTS_SKIPPED=$((TESTS_SKIPPED + 1))
    echo -e "  ${YELLOW}⊘ SKIP${NC}: $1"
}

run_as_postgres() {
    if [ "$(whoami)" = "postgres" ]; then
        "$@"
    else
        sudo -u postgres "$@"
    fi
}

cleanup_test_backups() {
    rm -rf "$TEST_BACKUP_DIR" 2>/dev/null || true
    mkdir -p "$TEST_BACKUP_DIR"
}

################################################################################
# Pre-Flight Checks
################################################################################

preflight_checks() {
    print_header "Pre-Flight Checks"
    
    # Check binary exists
    print_test "Check dbbackup binary exists"
    if [ -f "$DBBACKUP_BIN" ]; then
        print_success "Binary found: $DBBACKUP_BIN"
    else
        print_failure "Binary not found: $DBBACKUP_BIN"
        exit 1
    fi
    
    # Check binary is executable
    print_test "Check dbbackup is executable"
    if [ -x "$DBBACKUP_BIN" ]; then
        print_success "Binary is executable"
    else
        print_failure "Binary is not executable"
        exit 1
    fi
    
    # Check PostgreSQL tools
    print_test "Check PostgreSQL tools"
    if command -v pg_dump >/dev/null 2>&1 && command -v pg_restore >/dev/null 2>&1; then
        print_success "PostgreSQL tools available"
    else
        print_failure "PostgreSQL tools not found"
        exit 1
    fi
    
    # Check PostgreSQL is running
    print_test "Check PostgreSQL is running"
    if run_as_postgres psql -d postgres -c "SELECT 1" >/dev/null 2>&1; then
        print_success "PostgreSQL is running"
    else
        print_failure "PostgreSQL is not accessible"
        exit 1
    fi
    
    # Check disk space
    print_test "Check disk space"
    available=$(df -BG "$TEST_BACKUP_DIR" 2>/dev/null | awk 'NR==2 {print $4}' | tr -d 'G')
    if [ "$available" -gt 10 ]; then
        print_success "Sufficient disk space: ${available}GB available"
    else
        print_failure "Insufficient disk space: only ${available}GB available (need 10GB+)"
    fi
    
    # Check compression tools
    print_test "Check compression tools"
    if command -v pigz >/dev/null 2>&1; then
        print_success "pigz (parallel gzip) available"
    elif command -v gzip >/dev/null 2>&1; then
        print_success "gzip available (pigz not found, will be slower)"
    else
        print_failure "No compression tools found"
    fi
}

################################################################################
# CLI Command Tests
################################################################################

test_version_help() {
    print_header "Basic CLI Tests"
    
    print_test "Test --version flag"
    if run_as_postgres $DBBACKUP_BIN --version >/dev/null 2>&1; then
        print_success "Version command works"
    else
        print_failure "Version command failed"
    fi
    
    print_test "Test --help flag"
    if run_as_postgres $DBBACKUP_BIN --help >/dev/null 2>&1; then
        print_success "Help command works"
    else
        print_failure "Help command failed"
    fi
    
    print_test "Test backup --help"
    if run_as_postgres $DBBACKUP_BIN backup --help >/dev/null 2>&1; then
        print_success "Backup help works"
    else
        print_failure "Backup help failed"
    fi
    
    print_test "Test restore --help"
    if run_as_postgres $DBBACKUP_BIN restore --help >/dev/null 2>&1; then
        print_success "Restore help works"
    else
        print_failure "Restore help failed"
    fi
    
    print_test "Test status --help"
    if run_as_postgres $DBBACKUP_BIN status --help >/dev/null 2>&1; then
        print_success "Status help works"
    else
        print_failure "Status help failed"
    fi
}

test_backup_single() {
    print_header "Single Database Backup Tests"
    
    cleanup_test_backups
    
    # Test 1: Basic single database backup
    print_test "Single DB backup (default compression)"
    if run_as_postgres $DBBACKUP_BIN backup single "$TEST_DB" -d postgres --insecure \
        --backup-dir "$TEST_BACKUP_DIR" >>"$LOG_FILE" 2>&1; then
        if ls "$TEST_BACKUP_DIR"/db_${TEST_DB}_*.dump >/dev/null 2>&1; then
            size=$(ls -lh "$TEST_BACKUP_DIR"/db_${TEST_DB}_*.dump | awk '{print $5}')
            print_success "Backup created: $size"
        else
            print_failure "Backup file not found"
        fi
    else
        print_failure "Backup command failed"
    fi
    
    # Test 2: Low compression backup
    print_test "Single DB backup (low compression)"
    if run_as_postgres $DBBACKUP_BIN backup single "$TEST_DB" -d postgres --insecure \
        --backup-dir "$TEST_BACKUP_DIR" --compression 1 >>"$LOG_FILE" 2>&1; then
        print_success "Low compression backup succeeded"
    else
        print_failure "Low compression backup failed"
    fi
    
    # Test 3: High compression backup
    print_test "Single DB backup (high compression)"
    if run_as_postgres $DBBACKUP_BIN backup single "$TEST_DB" -d postgres --insecure \
        --backup-dir "$TEST_BACKUP_DIR" --compression 9 >>"$LOG_FILE" 2>&1; then
        print_success "High compression backup succeeded"
    else
        print_failure "High compression backup failed"
    fi
    
    # Test 4: Custom backup directory
    print_test "Single DB backup (custom directory)"
    custom_dir="$TEST_BACKUP_DIR/custom"
    mkdir -p "$custom_dir"
    if run_as_postgres $DBBACKUP_BIN backup single "$TEST_DB" -d postgres --insecure \
        --backup-dir "$custom_dir" >>"$LOG_FILE" 2>&1; then
        if ls "$custom_dir"/db_${TEST_DB}_*.dump >/dev/null 2>&1; then
            print_success "Backup created in custom directory"
        else
            print_failure "Backup not found in custom directory"
        fi
    else
        print_failure "Custom directory backup failed"
    fi
}

test_backup_cluster() {
    print_header "Cluster Backup Tests"
    
    cleanup_test_backups
    
    # Test 1: Basic cluster backup
    print_test "Cluster backup (all databases)"
    if timeout 180 run_as_postgres $DBBACKUP_BIN backup cluster -d postgres --insecure \
        --backup-dir "$TEST_BACKUP_DIR" --compression 3 >>"$LOG_FILE" 2>&1; then
        if ls "$TEST_BACKUP_DIR"/cluster_*.tar.gz >/dev/null 2>&1; then
            size=$(ls -lh "$TEST_BACKUP_DIR"/cluster_*.tar.gz 2>/dev/null | tail -1 | awk '{print $5}')
            if [ "$size" != "0" ]; then
                print_success "Cluster backup created: $size"
            else
                print_failure "Cluster backup is 0 bytes"
            fi
        else
            print_failure "Cluster backup file not found"
        fi
    else
        print_failure "Cluster backup failed or timed out"
    fi
    
    # Test 2: Verify no huge uncompressed temp files were left
    print_test "Verify no leftover temp files"
    if [ -d "$TEST_BACKUP_DIR/.cluster_"* ] 2>/dev/null; then
        print_failure "Temp cluster directory not cleaned up"
    else
        print_success "Temp directories cleaned up"
    fi
}

test_restore_single() {
    print_header "Single Database Restore Tests"
    
    cleanup_test_backups
    
    # Create a backup first
    print_test "Create backup for restore test"
    if run_as_postgres $DBBACKUP_BIN backup single "$TEST_DB" -d postgres --insecure \
        --backup-dir "$TEST_BACKUP_DIR" >>"$LOG_FILE" 2>&1; then
        backup_file=$(ls "$TEST_BACKUP_DIR"/db_${TEST_DB}_*.dump 2>/dev/null | head -1)
        if [ -n "$backup_file" ]; then
            print_success "Test backup created: $(basename $backup_file)"
            
            # Test restore with --create flag
            print_test "Restore with --create flag"
            restore_db="validation_restore_test_$$"
            if run_as_postgres $DBBACKUP_BIN restore single "$backup_file" \
                --target-db "$restore_db" -d postgres --insecure --create >>"$LOG_FILE" 2>&1; then
                # Check if database exists
                if run_as_postgres psql -lqt | cut -d \| -f 1 | grep -qw "$restore_db"; then
                    print_success "Database restored successfully with --create"
                    # Cleanup
                    run_as_postgres psql -d postgres -c "DROP DATABASE IF EXISTS $restore_db" >/dev/null 2>&1
                else
                    print_failure "Restored database not found"
                fi
            else
                print_failure "Restore with --create failed"
            fi
        else
            print_failure "Test backup file not found"
        fi
    else
        print_failure "Failed to create test backup"
    fi
}

test_status() {
    print_header "Status Command Tests"
    
    print_test "Status host command"
    if run_as_postgres $DBBACKUP_BIN status host -d postgres --insecure >>"$LOG_FILE" 2>&1; then
        print_success "Status host succeeded"
    else
        print_failure "Status host failed"
    fi
    
    print_test "Status cpu command"
    if $DBBACKUP_BIN status cpu >>"$LOG_FILE" 2>&1; then
        print_success "Status CPU succeeded"
    else
        print_failure "Status CPU failed"
    fi
}

test_compression_efficiency() {
    print_header "Compression Efficiency Tests"
    
    cleanup_test_backups
    
    # Create backups with different compression levels
    declare -A sizes
    
    for level in 1 6 9; do
        print_test "Backup with compression level $level"
        if run_as_postgres $DBBACKUP_BIN backup single "$TEST_DB" -d postgres --insecure \
            --backup-dir "$TEST_BACKUP_DIR" --compression $level >>"$LOG_FILE" 2>&1; then
            backup_file=$(ls -t "$TEST_BACKUP_DIR"/db_${TEST_DB}_*.dump 2>/dev/null | head -1)
            if [ -n "$backup_file" ]; then
                size=$(stat -f%z "$backup_file" 2>/dev/null || stat -c%s "$backup_file" 2>/dev/null)
                sizes[$level]=$size
                size_human=$(ls -lh "$backup_file" | awk '{print $5}')
                print_success "Level $level: $size_human"
            else
                print_failure "Backup file not found for level $level"
            fi
        else
            print_failure "Backup failed for compression level $level"
        fi
    done
    
    # Verify compression levels make sense (lower level = larger file)
    if [ ${sizes[1]:-0} -gt ${sizes[6]:-0} ] && [ ${sizes[6]:-0} -gt ${sizes[9]:-0} ]; then
        print_success "Compression levels work correctly (1 > 6 > 9)"
    else
        print_failure "Compression levels don't show expected size differences"
    fi
}

test_streaming_compression() {
    print_header "Streaming Compression Tests (Large DB)"
    
    # Check if testdb_50gb exists
    if run_as_postgres psql -lqt | cut -d \| -f 1 | grep -qw "testdb_50gb"; then
        cleanup_test_backups
        
        print_test "Backup large DB with streaming compression"
        # Use cluster backup which triggers streaming compression for large DBs
        if timeout 300 run_as_postgres $DBBACKUP_BIN backup single testdb_50gb -d postgres --insecure \
            --backup-dir "$TEST_BACKUP_DIR" --compression 1 >>"$LOG_FILE" 2>&1; then
            backup_file=$(ls "$TEST_BACKUP_DIR"/db_testdb_50gb_*.dump 2>/dev/null | head -1)
            if [ -n "$backup_file" ]; then
                size_human=$(ls -lh "$backup_file" | awk '{print $5}')
                print_success "Large DB backed up: $size_human"
            else
                print_failure "Large DB backup file not found"
            fi
        else
            print_failure "Large DB backup failed or timed out"
        fi
    else
        print_skip "testdb_50gb not found (large DB tests skipped)"
    fi
}

################################################################################
# Summary and Report
################################################################################

print_summary() {
    print_header "Validation Summary"
    
    echo ""
    echo "Total Tests: $TESTS_TOTAL"
    echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
    echo -e "${YELLOW}Skipped: $TESTS_SKIPPED${NC}"
    echo ""
    
    if [ $TESTS_FAILED -gt 0 ]; then
        echo -e "${RED}Failed Tests:${NC}"
        for test in "${FAILED_TESTS[@]}"; do
            echo -e "  ${RED}❌${NC} $test"
        done
        echo ""
    fi
    
    echo "Full log: $LOG_FILE"
    echo ""
    
    # Calculate success rate
    if [ $TESTS_TOTAL -gt 0 ]; then
        success_rate=$((TESTS_PASSED * 100 / TESTS_TOTAL))
        echo "Success Rate: ${success_rate}%"
        
        if [ $success_rate -ge 95 ]; then
            echo -e "${GREEN}✅ PRODUCTION READY${NC}"
            return 0
        elif [ $success_rate -ge 80 ]; then
            echo -e "${YELLOW}⚠️  NEEDS ATTENTION${NC}"
            return 1
        else
            echo -e "${RED}❌ NOT PRODUCTION READY${NC}"
            return 2
        fi
    fi
}

################################################################################
# Main Execution
################################################################################

main() {
    echo "================================================"
    echo "dbbackup Production Validation"
    echo "================================================"
    echo "Start Time: $(date)"
    echo "Log File: $LOG_FILE"
    echo "Test Backup Dir: $TEST_BACKUP_DIR"
    echo ""
    
    # Create log file
    touch "$LOG_FILE"
    
    # Run all test suites
    preflight_checks
    test_version_help
    test_backup_single
    test_backup_cluster
    test_restore_single
    test_status
    test_compression_efficiency
    test_streaming_compression
    
    # Print summary
    print_summary
    exit_code=$?
    
    # Cleanup
    echo ""
    echo "Cleaning up test files..."
    rm -rf "$TEST_BACKUP_DIR"
    
    echo "End Time: $(date)"
    echo ""
    
    exit $exit_code
}

# Run main
main
