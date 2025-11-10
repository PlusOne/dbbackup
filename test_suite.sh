#!/bin/bash
#
# DBBackup Complete Test Suite
# Automated testing of all command-line options
# Results written to test_results.txt
#

RESULTS_FILE="test_results_$(date +%Y%m%d_%H%M%S).txt"
DBBACKUP="./dbbackup"
TEST_DB="test_automation_db"
BACKUP_DIR="/var/lib/pgsql/db_backups"
TEST_BACKUP_DIR="/tmp/test_backups_$$"

# Colors for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

#######################################
# Helper Functions
#######################################

log() {
    echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $1" | tee -a "$RESULTS_FILE"
}

log_success() {
    echo -e "${GREEN}✅ PASS:${NC} $1" | tee -a "$RESULTS_FILE"
    ((PASSED_TESTS++))
    ((TOTAL_TESTS++))
}

log_fail() {
    echo -e "${RED}❌ FAIL:${NC} $1" | tee -a "$RESULTS_FILE"
    ((FAILED_TESTS++))
    ((TOTAL_TESTS++))
}

log_skip() {
    echo -e "${YELLOW}⊘ SKIP:${NC} $1" | tee -a "$RESULTS_FILE"
    ((SKIPPED_TESTS++))
    ((TOTAL_TESTS++))
}

log_section() {
    echo "" | tee -a "$RESULTS_FILE"
    echo "================================================================" | tee -a "$RESULTS_FILE"
    echo "  $1" | tee -a "$RESULTS_FILE"
    echo "================================================================" | tee -a "$RESULTS_FILE"
}

run_test() {
    local test_name="$1"
    local test_cmd="$2"
    local expected_result="${3:-0}" # 0=success, 1=failure expected
    
    log "Running: $test_name"
    echo "Command: $test_cmd" >> "$RESULTS_FILE"
    
    # Run command and capture output
    local output
    local exit_code
    output=$(eval "$test_cmd" 2>&1)
    exit_code=$?
    
    # Save output to results file
    echo "Exit Code: $exit_code" >> "$RESULTS_FILE"
    echo "Output:" >> "$RESULTS_FILE"
    echo "$output" | head -50 >> "$RESULTS_FILE"
    echo "---" >> "$RESULTS_FILE"
    
    # Check result
    if [ "$expected_result" -eq 0 ]; then
        # Expecting success
        if [ $exit_code -eq 0 ]; then
            log_success "$test_name"
            return 0
        else
            log_fail "$test_name (exit code: $exit_code)"
            return 1
        fi
    else
        # Expecting failure
        if [ $exit_code -ne 0 ]; then
            log_success "$test_name (correctly failed)"
            return 0
        else
            log_fail "$test_name (should have failed)"
            return 1
        fi
    fi
}

setup_test_env() {
    log "Setting up test environment..."
    
    # Create test database
    sudo -u postgres psql -c "DROP DATABASE IF EXISTS $TEST_DB;" > /dev/null 2>&1
    sudo -u postgres psql -c "CREATE DATABASE $TEST_DB;" > /dev/null 2>&1
    sudo -u postgres psql -d "$TEST_DB" -c "CREATE TABLE test_table (id SERIAL, data TEXT);" > /dev/null 2>&1
    sudo -u postgres psql -d "$TEST_DB" -c "INSERT INTO test_table (data) VALUES ('test1'), ('test2'), ('test3');" > /dev/null 2>&1
    
    # Create test backup directory
    mkdir -p "$TEST_BACKUP_DIR"
    
    log "Test environment ready"
}

cleanup_test_env() {
    log "Cleaning up test environment..."
    sudo -u postgres psql -c "DROP DATABASE IF EXISTS ${TEST_DB};" > /dev/null 2>&1
    sudo -u postgres psql -c "DROP DATABASE IF EXISTS ${TEST_DB}_restored;" > /dev/null 2>&1
    sudo -u postgres psql -c "DROP DATABASE IF EXISTS ${TEST_DB}_created;" > /dev/null 2>&1
    rm -rf "$TEST_BACKUP_DIR"
    log "Cleanup complete"
}

#######################################
# Test Suite
#######################################

main() {
    log_section "DBBackup Complete Test Suite"
    echo "Date: $(date)" | tee -a "$RESULTS_FILE"
    echo "Host: $(hostname)" | tee -a "$RESULTS_FILE"
    echo "User: $(whoami)" | tee -a "$RESULTS_FILE"
    echo "DBBackup: $DBBACKUP" | tee -a "$RESULTS_FILE"
    echo "Results File: $RESULTS_FILE" | tee -a "$RESULTS_FILE"
    echo "" | tee -a "$RESULTS_FILE"
    
    # Setup
    setup_test_env
    
    #######################################
    # 1. BASIC HELP & VERSION
    #######################################
    log_section "1. Basic Commands"
    
    run_test "Help command" \
        "sudo -u postgres $DBBACKUP --help"
    
    run_test "Version flag" \
        "sudo -u postgres $DBBACKUP --version"
    
    run_test "Status command" \
        "sudo -u postgres $DBBACKUP status"
    
    #######################################
    # 2. BACKUP SINGLE DATABASE
    #######################################
    log_section "2. Backup Single Database"
    
    run_test "Backup single database (basic)" \
        "sudo -u postgres $DBBACKUP backup single $TEST_DB"
    
    run_test "Backup single with compression level 9" \
        "sudo -u postgres $DBBACKUP backup single $TEST_DB --compression=9"
    
    run_test "Backup single with compression level 1" \
        "sudo -u postgres $DBBACKUP backup single $TEST_DB --compression=1"
    
    run_test "Backup single with custom backup dir" \
        "sudo -u postgres $DBBACKUP backup single $TEST_DB --backup-dir=$TEST_BACKUP_DIR"
    
    run_test "Backup single with jobs=1" \
        "sudo -u postgres $DBBACKUP backup single $TEST_DB --jobs=1"
    
    run_test "Backup single with jobs=16" \
        "sudo -u postgres $DBBACKUP backup single $TEST_DB --jobs=16"
    
    run_test "Backup single non-existent database (should fail)" \
        "sudo -u postgres $DBBACKUP backup single nonexistent_database_xyz" 1
    
    run_test "Backup single with debug logging" \
        "sudo -u postgres $DBBACKUP backup single $TEST_DB --debug"
    
    run_test "Backup single with no-color" \
        "sudo -u postgres $DBBACKUP backup single $TEST_DB --no-color"
    
    #######################################
    # 3. BACKUP CLUSTER
    #######################################
    log_section "3. Backup Cluster"
    
    run_test "Backup cluster (basic)" \
        "sudo -u postgres $DBBACKUP backup cluster"
    
    run_test "Backup cluster with compression 9" \
        "sudo -u postgres $DBBACKUP backup cluster --compression=9"
    
    run_test "Backup cluster with jobs=4" \
        "sudo -u postgres $DBBACKUP backup cluster --jobs=4"
    
    run_test "Backup cluster with dump-jobs=4" \
        "sudo -u postgres $DBBACKUP backup cluster --dump-jobs=4"
    
    run_test "Backup cluster with custom backup dir" \
        "sudo -u postgres $DBBACKUP backup cluster --backup-dir=$TEST_BACKUP_DIR"
    
    run_test "Backup cluster with debug" \
        "sudo -u postgres $DBBACKUP backup cluster --debug"
    
    #######################################
    # 4. RESTORE LIST
    #######################################
    log_section "4. Restore List"
    
    run_test "List available backups" \
        "sudo -u postgres $DBBACKUP restore list"
    
    run_test "List backups from custom dir" \
        "sudo -u postgres $DBBACKUP restore list --backup-dir=$TEST_BACKUP_DIR"
    
    #######################################
    # 5. RESTORE SINGLE DATABASE
    #######################################
    log_section "5. Restore Single Database"
    
    # Get latest backup file
    LATEST_BACKUP=$(find "$BACKUP_DIR" -name "db_${TEST_DB}_*.dump" -type f -printf '%T@ %p\n' | sort -n | tail -1 | cut -d' ' -f2-)
    
    if [ -n "$LATEST_BACKUP" ]; then
        log "Using backup file: $LATEST_BACKUP"
        
        # Create target database for restore
        sudo -u postgres psql -c "DROP DATABASE IF EXISTS ${TEST_DB}_restored;" > /dev/null 2>&1
        sudo -u postgres psql -c "CREATE DATABASE ${TEST_DB}_restored;" > /dev/null 2>&1
        
        run_test "Restore single database (basic)" \
            "sudo -u postgres $DBBACKUP restore single $LATEST_BACKUP --target=${TEST_DB}_restored --confirm"
        
        run_test "Restore single with --clean flag" \
            "sudo -u postgres $DBBACKUP restore single $LATEST_BACKUP --target=${TEST_DB}_restored --clean --confirm"
        
        run_test "Restore single with --create flag" \
            "sudo -u postgres $DBBACKUP restore single $LATEST_BACKUP --target=${TEST_DB}_created --create --confirm"
        
        run_test "Restore single with --dry-run" \
            "sudo -u postgres $DBBACKUP restore single $LATEST_BACKUP --target=${TEST_DB}_restored --dry-run"
        
        run_test "Restore single with --verbose" \
            "sudo -u postgres $DBBACKUP restore single $LATEST_BACKUP --target=${TEST_DB}_restored --verbose --confirm"
        
        run_test "Restore single with --force" \
            "sudo -u postgres $DBBACKUP restore single $LATEST_BACKUP --target=${TEST_DB}_restored --force --confirm"
        
        run_test "Restore single without --confirm (should show dry-run)" \
            "sudo -u postgres $DBBACKUP restore single $LATEST_BACKUP --target=${TEST_DB}_restored"
    else
        log_skip "Restore single tests (no backup file found)"
    fi
    
    run_test "Restore non-existent file (should fail)" \
        "sudo -u postgres $DBBACKUP restore single /tmp/nonexistent_file.dump --confirm" 1
    
    #######################################
    # 6. RESTORE CLUSTER
    #######################################
    log_section "6. Restore Cluster"
    
    # Get latest cluster backup
    LATEST_CLUSTER=$(find "$BACKUP_DIR" -name "cluster_*.tar.gz" -type f -printf '%T@ %p\n' | sort -n | tail -1 | cut -d' ' -f2-)
    
    if [ -n "$LATEST_CLUSTER" ]; then
        log "Using cluster backup: $LATEST_CLUSTER"
        
        run_test "Restore cluster with --dry-run" \
            "sudo -u postgres $DBBACKUP restore cluster $LATEST_CLUSTER --dry-run"
        
        run_test "Restore cluster with --verbose" \
            "sudo -u postgres $DBBACKUP restore cluster $LATEST_CLUSTER --verbose --confirm"
        
        run_test "Restore cluster with --force" \
            "sudo -u postgres $DBBACKUP restore cluster $LATEST_CLUSTER --force --confirm"
        
        run_test "Restore cluster with --jobs=2" \
            "sudo -u postgres $DBBACKUP restore cluster $LATEST_CLUSTER --jobs=2 --confirm"
        
        run_test "Restore cluster without --confirm (should show dry-run)" \
            "sudo -u postgres $DBBACKUP restore cluster $LATEST_CLUSTER"
    else
        log_skip "Restore cluster tests (no cluster backup found)"
    fi
    
    #######################################
    # 7. GLOBAL FLAGS
    #######################################
    log_section "7. Global Flags"
    
    run_test "Custom host flag" \
        "sudo -u postgres $DBBACKUP status --host=localhost"
    
    run_test "Custom port flag" \
        "sudo -u postgres $DBBACKUP status --port=5432"
    
    run_test "Custom user flag" \
        "sudo -u postgres $DBBACKUP status --user=postgres"
    
    run_test "Database type postgres" \
        "sudo -u postgres $DBBACKUP status --db-type=postgres"
    
    run_test "SSL mode disable (insecure)" \
        "sudo -u postgres $DBBACKUP status --insecure"
    
    run_test "SSL mode require" \
        "sudo -u postgres $DBBACKUP status --ssl-mode=require" 1
    
    run_test "SSL mode prefer" \
        "sudo -u postgres $DBBACKUP status --ssl-mode=prefer"
    
    run_test "Max cores flag" \
        "sudo -u postgres $DBBACKUP status --max-cores=4"
    
    run_test "Disable auto-detect cores" \
        "sudo -u postgres $DBBACKUP status --auto-detect-cores=false"
    
    run_test "CPU workload balanced" \
        "sudo -u postgres $DBBACKUP status --cpu-workload=balanced"
    
    run_test "CPU workload cpu-intensive" \
        "sudo -u postgres $DBBACKUP status --cpu-workload=cpu-intensive"
    
    run_test "CPU workload io-intensive" \
        "sudo -u postgres $DBBACKUP status --cpu-workload=io-intensive"
    
    #######################################
    # 8. AUTHENTICATION TESTS
    #######################################
    log_section "8. Authentication Tests"
    
    run_test "Connection with peer auth (default)" \
        "sudo -u postgres $DBBACKUP status"
    
    run_test "Connection with --user flag" \
        "sudo -u postgres $DBBACKUP status --user=postgres"
    
    # This should fail or warn
    run_test "Wrong user flag (should fail/warn)" \
        "./dbbackup status --user=postgres" 1
    
    #######################################
    # 9. ERROR SCENARIOS
    #######################################
    log_section "9. Error Scenarios"
    
    run_test "Invalid compression level (should fail)" \
        "sudo -u postgres $DBBACKUP backup single $TEST_DB --compression=99" 1
    
    run_test "Invalid database type (should fail)" \
        "sudo -u postgres $DBBACKUP status --db-type=invalid" 1
    
    run_test "Invalid CPU workload (should fail)" \
        "sudo -u postgres $DBBACKUP status --cpu-workload=invalid" 1
    
    run_test "Invalid port (should fail)" \
        "sudo -u postgres $DBBACKUP status --port=99999" 1
    
    run_test "Backup to read-only directory (should fail)" \
        "sudo -u postgres $DBBACKUP backup single $TEST_DB --backup-dir=/proc" 1
    
    #######################################
    # 10. INTERACTIVE MODE (Quick Test)
    #######################################
    log_section "10. Interactive Mode"
    
    # Can't fully test interactive mode in script, but check it launches
    run_test "Interactive mode help" \
        "sudo -u postgres $DBBACKUP interactive --help"
    
    #######################################
    # SUMMARY
    #######################################
    log_section "Test Suite Summary"
    
    echo "" | tee -a "$RESULTS_FILE"
    echo "Total Tests:  $TOTAL_TESTS" | tee -a "$RESULTS_FILE"
    echo "Passed:       $PASSED_TESTS" | tee -a "$RESULTS_FILE"
    echo "Failed:       $FAILED_TESTS" | tee -a "$RESULTS_FILE"
    echo "Skipped:      $SKIPPED_TESTS" | tee -a "$RESULTS_FILE"
    echo "" | tee -a "$RESULTS_FILE"
    
    if [ $FAILED_TESTS -eq 0 ]; then
        log_success "All tests passed! 🎉"
        EXIT_CODE=0
    else
        log_fail "$FAILED_TESTS test(s) failed"
        EXIT_CODE=1
    fi
    
    echo "" | tee -a "$RESULTS_FILE"
    echo "Results saved to: $RESULTS_FILE" | tee -a "$RESULTS_FILE"
    echo "" | tee -a "$RESULTS_FILE"
    
    # Cleanup
    cleanup_test_env
    
    exit $EXIT_CODE
}

# Run main function
main "$@"
