#!/bin/bash
#
# Comprehensive Security Testing Suite for dbbackup
# Tests all security features via both CLI and TUI modes
#
# Usage: ./comprehensive_security_test.sh [options]
#   --cli-only        Test CLI mode only
#   --tui-only        Test TUI mode only
#   --quick           Run quick tests only
#   --verbose         Enable verbose output
#

set -e  # Exit on error

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DBBACKUP="${SCRIPT_DIR}/dbbackup"
TEST_DIR="${SCRIPT_DIR}/test_workspace"
LOG_DIR="${TEST_DIR}/logs"
BACKUP_DIR="${TEST_DIR}/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
MAIN_LOG="${LOG_DIR}/comprehensive_test_${TIMESTAMP}.log"
SUMMARY_LOG="${LOG_DIR}/test_summary_${TIMESTAMP}.log"

# Test configuration
TEST_HOST="${TEST_HOST:-localhost}"
TEST_PORT="${TEST_PORT:-5432}"
TEST_USER="${TEST_USER:-postgres}"
TEST_DB="${TEST_DB:-postgres}"

# Colors
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

# Parse arguments
CLI_ONLY=false
TUI_ONLY=false
QUICK_MODE=false
VERBOSE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --cli-only)
            CLI_ONLY=true
            shift
            ;;
        --tui-only)
            TUI_ONLY=true
            shift
            ;;
        --quick)
            QUICK_MODE=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Setup
setup_test_environment() {
    echo -e "${BLUE}=== Setting up test environment ===${NC}"
    
    # Create directories
    mkdir -p "${TEST_DIR}"
    mkdir -p "${LOG_DIR}"
    mkdir -p "${BACKUP_DIR}"
    
    # Build if needed
    if [ ! -f "${DBBACKUP}" ]; then
        echo "Building dbbackup..."
        cd "${SCRIPT_DIR}"
        go build -o dbbackup
    fi
    
    # Create test log
    cat > "${MAIN_LOG}" <<EOF
=================================================
Comprehensive Security Test Suite
Started: $(date)
Host: ${TEST_HOST}:${TEST_PORT}
Database: ${TEST_DB}
=================================================

EOF
    
    echo "Test environment ready"
    echo "Main log: ${MAIN_LOG}"
    echo "Summary: ${SUMMARY_LOG}"
    echo ""
}

# Logging functions
log_test_start() {
    local test_name="$1"
    local mode="$2"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -e "${BLUE}[TEST $TOTAL_TESTS] ${mode}: ${test_name}${NC}"
    echo "[TEST $TOTAL_TESTS] ${mode}: ${test_name}" >> "${MAIN_LOG}"
    echo "Started: $(date)" >> "${MAIN_LOG}"
}

log_test_pass() {
    local test_name="$1"
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}✓ PASS${NC}: ${test_name}"
    echo "✓ PASS: ${test_name}" >> "${MAIN_LOG}"
    echo "" >> "${MAIN_LOG}"
}

log_test_fail() {
    local test_name="$1"
    local reason="$2"
    FAILED_TESTS=$((FAILED_TESTS + 1))
    echo -e "${RED}✗ FAIL${NC}: ${test_name}"
    echo "  Reason: ${reason}"
    echo "✗ FAIL: ${test_name}" >> "${MAIN_LOG}"
    echo "  Reason: ${reason}" >> "${MAIN_LOG}"
    echo "" >> "${MAIN_LOG}"
}

log_test_skip() {
    local test_name="$1"
    local reason="$2"
    SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
    echo -e "${YELLOW}⊘ SKIP${NC}: ${test_name}"
    echo "  Reason: ${reason}"
    echo "⊘ SKIP: ${test_name}" >> "${MAIN_LOG}"
    echo "  Reason: ${reason}" >> "${MAIN_LOG}"
    echo "" >> "${MAIN_LOG}"
}

log_section() {
    local section="$1"
    echo ""
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}  $section${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo "" | tee -a "${MAIN_LOG}"
    echo "=== $section ===" >> "${MAIN_LOG}"
    echo "" >> "${MAIN_LOG}"
}

# Test execution wrapper
run_cli_test() {
    local test_name="$1"
    local command="$2"
    local expected_pattern="$3"
    
    log_test_start "$test_name" "CLI"
    
    local output
    local exit_code
    
    if [ "$VERBOSE" = true ]; then
        output=$(eval "$command" 2>&1 | tee -a "${MAIN_LOG}")
        exit_code=${PIPESTATUS[0]}
    else
        output=$(eval "$command" 2>&1 | tee -a "${MAIN_LOG}")
        exit_code=$?
    fi
    
    if [ -n "$expected_pattern" ]; then
        if echo "$output" | grep -q "$expected_pattern"; then
            log_test_pass "$test_name"
            return 0
        else
            log_test_fail "$test_name" "Expected pattern not found: $expected_pattern"
            return 1
        fi
    else
        if [ $exit_code -eq 0 ]; then
            log_test_pass "$test_name"
            return 0
        else
            log_test_fail "$test_name" "Command failed with exit code $exit_code"
            return 1
        fi
    fi
}

run_tui_test() {
    local test_name="$1"
    local auto_select="$2"
    local auto_database="$3"
    local additional_flags="$4"
    local expected_pattern="$5"
    
    log_test_start "$test_name" "TUI"
    
    local command="${DBBACKUP} interactive --auto-select ${auto_select}"
    [ -n "$auto_database" ] && command="$command --auto-database '${auto_database}'"
    command="$command --dry-run --verbose-tui"
    [ -n "$additional_flags" ] && command="$command ${additional_flags}"
    command="$command --backup-dir '${BACKUP_DIR}' --host ${TEST_HOST} --port ${TEST_PORT}"
    
    local output
    if [ "$VERBOSE" = true ]; then
        output=$(eval "$command" 2>&1 | tee -a "${MAIN_LOG}")
    else
        output=$(eval "$command" 2>&1 | tee -a "${MAIN_LOG}")
    fi
    local exit_code=$?
    
    if [ -n "$expected_pattern" ]; then
        if echo "$output" | grep -q "$expected_pattern"; then
            log_test_pass "$test_name"
            return 0
        else
            log_test_fail "$test_name" "Expected pattern not found: $expected_pattern"
            return 1
        fi
    else
        if [ $exit_code -eq 0 ]; then
            log_test_pass "$test_name"
            return 0
        else
            log_test_fail "$test_name" "Command failed with exit code $exit_code"
            return 1
        fi
    fi
}

# ============================================================================
# SECURITY FEATURE TESTS
# ============================================================================

test_retention_policy() {
    log_section "RETENTION POLICY TESTS"
    
    # Setup: Create old backup files
    mkdir -p "${BACKUP_DIR}"
    for i in {1..10}; do
        local old_date=$(date -d "$i days ago" +%Y%m%d)
        touch -t "${old_date}0000" "${BACKUP_DIR}/db_test_${old_date}_120000.dump"
    done
    
    if [ "$CLI_ONLY" = false ]; then
        # CLI Test
        run_cli_test \
            "Retention: CLI with 7 day policy" \
            "${DBBACKUP} backup single ${TEST_DB} --backup-dir '${BACKUP_DIR}' --retention-days 7 --min-backups 3 --host ${TEST_HOST} --dry-run 2>&1" \
            ""
    fi
    
    if [ "$TUI_ONLY" = false ]; then
        # TUI Test
        run_tui_test \
            "Retention: TUI with 7 day policy" \
            "0" \
            "${TEST_DB}" \
            "--retention-days 7 --min-backups 3" \
            ""
    fi
    
    # Cleanup
    rm -f "${BACKUP_DIR}"/db_test_*.dump
}

test_rate_limiting() {
    log_section "RATE LIMITING TESTS"
    
    if [ "$CLI_ONLY" = false ]; then
        # CLI Test with invalid host (should trigger rate limiting)
        run_cli_test \
            "Rate Limit: CLI with max retries" \
            "${DBBACKUP} backup single ${TEST_DB} --host invalid.nonexistent --max-retries 2 --backup-dir '${BACKUP_DIR}' 2>&1" \
            "rate limit\\|max retries\\|connection failed"
    fi
    
    if [ "$TUI_ONLY" = false ]; then
        # TUI Test with rate limiting
        run_tui_test \
            "Rate Limit: TUI with max retries" \
            "0" \
            "${TEST_DB}" \
            "--host invalid.nonexistent --max-retries 2" \
            "rate limit\\|max retries\\|connection failed"
    fi
}

test_privilege_checks() {
    log_section "PRIVILEGE CHECK TESTS"
    
    local is_root=false
    if [ "$(id -u)" = "0" ]; then
        is_root=true
    fi
    
    if [ "$is_root" = true ]; then
        if [ "$CLI_ONLY" = false ]; then
            # CLI Test as root (should warn)
            run_cli_test \
                "Privilege: CLI running as root (should warn)" \
                "${DBBACKUP} backup single ${TEST_DB} --backup-dir '${BACKUP_DIR}' --host ${TEST_HOST} --dry-run 2>&1" \
                "elevated privileges\\|running as root\\|Administrator"
        fi
        
        if [ "$TUI_ONLY" = false ]; then
            # TUI Test as root
            run_tui_test \
                "Privilege: TUI running as root (should warn)" \
                "0" \
                "${TEST_DB}" \
                "" \
                "elevated privileges\\|running as root"
        fi
        
        if [ "$CLI_ONLY" = false ]; then
            # CLI Test with --allow-root flag
            run_cli_test \
                "Privilege: CLI with --allow-root override" \
                "${DBBACKUP} backup single ${TEST_DB} --backup-dir '${BACKUP_DIR}' --host ${TEST_HOST} --allow-root --dry-run 2>&1" \
                ""
        fi
    else
        log_test_skip "Privilege checks" "Not running as root (cannot test root warnings)"
    fi
}

test_resource_limits() {
    log_section "RESOURCE LIMIT TESTS"
    
    if [ "$CLI_ONLY" = false ]; then
        # CLI Test with resource checks enabled
        run_cli_test \
            "Resources: CLI with checks enabled" \
            "${DBBACKUP} backup single ${TEST_DB} --backup-dir '${BACKUP_DIR}' --host ${TEST_HOST} --check-resources --dry-run 2>&1" \
            ""
        
        # CLI Test with resource checks disabled
        run_cli_test \
            "Resources: CLI with checks disabled" \
            "${DBBACKUP} backup single ${TEST_DB} --backup-dir '${BACKUP_DIR}' --host ${TEST_HOST} --no-check-resources --dry-run 2>&1" \
            ""
    fi
    
    if [ "$TUI_ONLY" = false ]; then
        # TUI Test with resource checks
        run_tui_test \
            "Resources: TUI with checks enabled" \
            "0" \
            "${TEST_DB}" \
            "--check-resources" \
            ""
    fi
}

test_path_sanitization() {
    log_section "PATH SANITIZATION TESTS"
    
    if [ "$CLI_ONLY" = false ]; then
        # CLI Test with path traversal attempt
        run_cli_test \
            "Path Security: CLI rejects path traversal" \
            "${DBBACKUP} backup single ${TEST_DB} --backup-dir '../../etc/passwd' --host ${TEST_HOST} 2>&1" \
            "invalid.*path\\|path traversal\\|security"
        
        # CLI Test with valid path
        run_cli_test \
            "Path Security: CLI accepts valid path" \
            "${DBBACKUP} backup single ${TEST_DB} --backup-dir '${BACKUP_DIR}' --host ${TEST_HOST} --dry-run 2>&1" \
            ""
    fi
}

test_checksum_verification() {
    log_section "CHECKSUM VERIFICATION TESTS"
    
    if [ "$QUICK_MODE" = true ]; then
        log_test_skip "Checksum tests" "Quick mode enabled"
        return
    fi
    
    # Create a test backup file
    local test_backup="${BACKUP_DIR}/test_checksum.dump"
    echo "test backup data" > "$test_backup"
    
    # Generate checksum manually
    local checksum=$(sha256sum "$test_backup" | awk '{print $1}')
    echo "${checksum}  ${test_backup}" > "${test_backup}.sha256"
    
    if [ "$CLI_ONLY" = false ]; then
        # CLI Test: Checksum verification should pass
        run_cli_test \
            "Checksum: CLI verifies valid checksum" \
            "${DBBACKUP} restore single '${test_backup}' --host ${TEST_HOST} --dry-run 2>&1" \
            "checksum verified\\|✓"
    fi
    
    # Corrupt the backup
    echo "corrupted" >> "$test_backup"
    
    if [ "$CLI_ONLY" = false ]; then
        # CLI Test: Checksum verification should fail
        run_cli_test \
            "Checksum: CLI detects corruption" \
            "${DBBACKUP} restore single '${test_backup}' --host ${TEST_HOST} --dry-run 2>&1" \
            "checksum.*fail\\|verification failed\\|mismatch"
    fi
    
    # Cleanup
    rm -f "$test_backup" "${test_backup}.sha256"
}

test_audit_logging() {
    log_section "AUDIT LOGGING TESTS"
    
    if [ "$CLI_ONLY" = false ]; then
        # CLI Test: Check audit logs are generated
        run_cli_test \
            "Audit: CLI generates audit events" \
            "${DBBACKUP} backup single ${TEST_DB} --backup-dir '${BACKUP_DIR}' --host ${TEST_HOST} --dry-run --debug 2>&1" \
            "AUDIT\\|audit.*true"
    fi
    
    if [ "$TUI_ONLY" = false ]; then
        # TUI Test: Check audit logs in TUI mode
        run_tui_test \
            "Audit: TUI generates audit events" \
            "0" \
            "${TEST_DB}" \
            "--debug" \
            "AUDIT\\|audit"
    fi
}

test_config_persistence() {
    log_section "CONFIG PERSISTENCE TESTS"
    
    local config_file="${TEST_DIR}/.dbbackup.conf"
    rm -f "$config_file"
    
    cd "${TEST_DIR}"
    
    if [ "$CLI_ONLY" = false ]; then
        # CLI Test: Create config with security settings
        run_cli_test \
            "Config: CLI saves security settings" \
            "${DBBACKUP} backup single ${TEST_DB} --backup-dir '${BACKUP_DIR}' --host ${TEST_HOST} --retention-days 14 --max-retries 5 --dry-run 2>&1" \
            ""
        
        # Verify config file created
        if [ -f "$config_file" ]; then
            if grep -q "\[security\]" "$config_file" && \
               grep -q "retention_days = 14" "$config_file" && \
               grep -q "max_retries = 5" "$config_file"; then
                log_test_pass "Config: Security section saved correctly"
            else
                log_test_fail "Config: Security section incomplete" "Missing expected settings"
            fi
        else
            log_test_fail "Config: Config file not created" "Expected $config_file"
        fi
    fi
    
    cd "${SCRIPT_DIR}"
}

test_tui_automation() {
    log_section "TUI AUTOMATION TESTS"
    
    if [ "$CLI_ONLY" = true ]; then
        log_test_skip "TUI automation tests" "CLI-only mode"
        return
    fi
    
    # Test all menu options with auto-select
    local menu_options=(
        "0:Single Database Backup"
        "1:Sample Database Backup"
        "2:Cluster Backup"
        "4:Restore Single Database"
        "5:Restore Cluster Backup"
        "10:Database Status"
    )
    
    for option in "${menu_options[@]}"; do
        local index="${option%%:*}"
        local name="${option#*:}"
        
        run_tui_test \
            "TUI Menu: Auto-select option ${index} (${name})" \
            "$index" \
            "${TEST_DB}" \
            "" \
            ""
    done
}

# ============================================================================
# INTEGRATION TESTS
# ============================================================================

test_full_backup_workflow() {
    log_section "FULL BACKUP WORKFLOW TESTS"
    
    if [ "$QUICK_MODE" = true ]; then
        log_test_skip "Full workflow tests" "Quick mode enabled"
        return
    fi
    
    if [ "$CLI_ONLY" = false ]; then
        # CLI: Full backup with all security features
        run_cli_test \
            "Workflow: CLI full backup with security" \
            "${DBBACKUP} backup single ${TEST_DB} \
                --backup-dir '${BACKUP_DIR}' \
                --host ${TEST_HOST} \
                --retention-days 30 \
                --min-backups 5 \
                --max-retries 3 \
                --check-resources \
                --dry-run 2>&1" \
            ""
    fi
    
    if [ "$TUI_ONLY" = false ]; then
        # TUI: Full backup with all security features
        run_tui_test \
            "Workflow: TUI full backup with security" \
            "0" \
            "${TEST_DB}" \
            "--retention-days 30 --min-backups 5 --max-retries 3 --check-resources" \
            ""
    fi
}

# ============================================================================
# MAIN EXECUTION
# ============================================================================

main() {
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  Comprehensive Security Test Suite for dbbackup               ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    setup_test_environment
    
    # Run all test suites
    test_retention_policy
    test_rate_limiting
    test_privilege_checks
    test_resource_limits
    test_path_sanitization
    test_checksum_verification
    test_audit_logging
    test_config_persistence
    test_tui_automation
    test_full_backup_workflow
    
    # Generate summary
    generate_summary
    
    # Cleanup
    cleanup_test_environment
}

generate_summary() {
    log_section "TEST SUMMARY"
    
    local pass_rate=0
    if [ $TOTAL_TESTS -gt 0 ]; then
        pass_rate=$((PASSED_TESTS * 100 / TOTAL_TESTS))
    fi
    
    cat > "${SUMMARY_LOG}" <<EOF
=================================================
Test Summary
=================================================
Total Tests:   $TOTAL_TESTS
Passed:        $PASSED_TESTS
Failed:        $FAILED_TESTS
Skipped:       $SKIPPED_TESTS
Pass Rate:     ${pass_rate}%

Completed:     $(date)
=================================================
EOF
    
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}Total Tests:   ${TOTAL_TESTS}${NC}"
    echo -e "${GREEN}✓ Passed:      ${PASSED_TESTS}${NC}"
    echo -e "${RED}✗ Failed:      ${FAILED_TESTS}${NC}"
    echo -e "${YELLOW}⊘ Skipped:     ${SKIPPED_TESTS}${NC}"
    echo -e "${BLUE}Pass Rate:     ${pass_rate}%${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo "Detailed logs: ${MAIN_LOG}"
    echo "Summary:       ${SUMMARY_LOG}"
    echo ""
    
    if [ $FAILED_TESTS -gt 0 ]; then
        echo -e "${RED}⚠ Some tests failed. Please review the logs.${NC}"
        exit 1
    else
        echo -e "${GREEN}✓ All tests passed successfully!${NC}"
        exit 0
    fi
}

cleanup_test_environment() {
    echo ""
    echo "Cleaning up..."
    
    # Remove test workspace (optional - comment out to keep for debugging)
    # rm -rf "${TEST_DIR}"
    
    echo "Cleanup complete"
}

# Run main
main
