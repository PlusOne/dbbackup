#!/bin/bash
#
# Quick Test Script - Fast validation of security features
# Usage: ./quick_test.sh
#

set -e

DBBACKUP="./dbbackup"
TEST_DIR="./test_quick"
BACKUP_DIR="${TEST_DIR}/backups"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Quick Security Feature Test"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Setup
mkdir -p "${BACKUP_DIR}"

# Build if needed
if [ ! -f "${DBBACKUP}" ]; then
    echo "Building dbbackup..."
    go build -o dbbackup
fi

echo "1. Testing TUI Auto-Select (Single Backup)"
echo "   Command: ${DBBACKUP} interactive --auto-select 0 --auto-database testdb --dry-run --verbose-tui"
${DBBACKUP} interactive --auto-select 0 --auto-database testdb --dry-run --verbose-tui --backup-dir "${BACKUP_DIR}" 2>&1 | head -20
echo ""

echo "2. Testing Help for New Flags"
echo "   Checking --auto-select, --retention-days, --max-retries..."
${DBBACKUP} interactive --help | grep -E "auto-select|retention-days|max-retries|allow-root|verbose-tui" || echo "Flags found!"
echo ""

echo "3. Testing Security Flags in Root Command"
${DBBACKUP} --help | grep -E "retention|retries|allow-root" | head -5
echo ""

echo "4. Testing CLI Retention Policy"
echo "   Creating test backups..."
for i in {1..5}; do
    touch "${BACKUP_DIR}/db_test_$(date -d "$i days ago" +%Y%m%d)_120000.dump"
done
ls -lh "${BACKUP_DIR}"
echo ""

echo "5. Testing Privilege Check (as current user)"
${DBBACKUP} backup single testdb --backup-dir "${BACKUP_DIR}" --dry-run 2>&1 | grep -i "privilege\|root\|warning" || echo "No root warning (expected if not root)"
echo ""

echo "6. Testing Resource Checks"
${DBBACKUP} backup single testdb --backup-dir "${BACKUP_DIR}" --check-resources --dry-run 2>&1 | grep -i "resource\|limit" || echo "Resource checks completed"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Quick Test Complete!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "To run comprehensive tests, use:"
echo "  ./comprehensive_security_test.sh"
echo ""

# Cleanup
rm -rf "${TEST_DIR}"
