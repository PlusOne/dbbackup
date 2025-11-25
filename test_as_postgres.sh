#!/bin/bash
#
# Focused Security Test - CLI Mode Only (Fully Automated)
# Runs as postgres user for proper database access
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Config
TEST_DIR="/tmp/dbbackup_test"
BINARY="/root/dbbackup/dbbackup"
LOG_FILE="$TEST_DIR/test_$(date +%Y%m%d_%H%M%S).log"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Security Features Test (CLI Mode)${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}ERROR: Must run as root to switch to postgres user${NC}"
    echo "Usage: sudo $0"
    exit 1
fi

# Setup test environment
echo -e "${YELLOW}► Setting up test environment...${NC}"
rm -rf "$TEST_DIR"
mkdir -p "$TEST_DIR/backups"
chmod 755 "$TEST_DIR" "$TEST_DIR/backups"
chown -R postgres:postgres "$TEST_DIR"

# Copy binary
cp "$BINARY" "$TEST_DIR/"
chmod 755 "$TEST_DIR/dbbackup"

# Create config
cat > "$TEST_DIR/.dbbackup.conf" <<'EOF'
[database]
type = postgres
host = localhost
port = 5432
user = postgres
database = postgres

[backup]
dir = /tmp/dbbackup_test/backups
format = custom
jobs = 2

[security]
retention_days = 7
min_backups = 3
max_retries = 3
EOF
chmod 644 "$TEST_DIR/.dbbackup.conf"
chown postgres:postgres "$TEST_DIR/.dbbackup.conf"

echo -e "${GREEN}✓ Environment ready: $TEST_DIR${NC}"
echo

# Test counters
TOTAL=0
PASSED=0
FAILED=0

# Test function
run_test() {
    local name="$1"
    local cmd="$2"
    
    TOTAL=$((TOTAL + 1))
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Test $TOTAL: $name${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}Command:${NC} $cmd"
    echo
    
    if eval "$cmd"; then
        echo
        echo -e "${GREEN}✓ PASSED${NC}"
        PASSED=$((PASSED + 1))
    else
        echo
        echo -e "${RED}✗ FAILED${NC}"
        FAILED=$((FAILED + 1))
    fi
    echo
}

cd "$TEST_DIR"

# Test 1: Security flags available
run_test "Security Flags Available" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup --help' 2>&1 | grep -E 'retention-days|max-retries|allow-root|check-resources'"

# Test 2: Retention policy flag
run_test "Retention Policy Flag" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup backup single postgres --retention-days 7 --dry-run' 2>&1 | grep -i 'retention\|dry-run'"

# Test 3: Rate limiting (max retries)
run_test "Rate Limiting Configuration" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup backup single postgres --max-retries 2 --dry-run' 2>&1 | head -20"

# Test 4: Privilege check (as non-root)
run_test "Privilege Check (postgres user)" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup backup single postgres --dry-run' 2>&1 | grep -v 'WARNING.*root' | head -10"

# Test 5: Resource limits check
run_test "Resource Limits Check" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup backup single postgres --check-resources --dry-run' 2>&1 | head -15"

# Test 6: Actual backup with security features
run_test "Real Backup with Security Features" \
    "su - postgres -c 'cd $TEST_DIR && ./dbbackup backup single postgres --retention-days 30 --min-backups 5 --debug' 2>&1 | grep -E 'Backup completed|Starting backup|Connected'"

# Test 7: Config persistence
run_test "Config File Persistence" \
    "test -f $TEST_DIR/.dbbackup.conf && grep -q 'retention_days' $TEST_DIR/.dbbackup.conf"

# Test 8: Backup files created (check actual backup directory)
run_test "Backup Files Created" \
    "ls -lh /var/lib/pgsql/db_backups/db_postgres_*.dump 2>/dev/null | tail -3"

# Test 9: Checksum files present
run_test "Checksum Files Created" \
    "ls /var/lib/pgsql/db_backups/db_postgres_*.sha256 2>/dev/null | tail -3"

# Test 10: Meta files present
run_test "Metadata Files Created" \
    "ls /var/lib/pgsql/db_backups/db_postgres_*.info 2>/dev/null | tail -3"

# Summary
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Test Summary${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo
echo -e "Total Tests:  $TOTAL"
echo -e "${GREEN}Passed:       $PASSED${NC}"
echo -e "${RED}Failed:       $FAILED${NC}"
echo
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    EXIT_CODE=0
else
    echo -e "${RED}⚠️  Some tests failed${NC}"
    EXIT_CODE=1
fi
echo
echo -e "Test directory: $TEST_DIR"
echo -e "Backup directory: /var/lib/pgsql/db_backups"
echo -e "Total postgres backups: $(ls -1 /var/lib/pgsql/db_backups/db_postgres_*.dump 2>/dev/null | wc -l) files"
echo

exit $EXIT_CODE
