#!/bin/bash
#
# Test Runner Wrapper - Executes tests as postgres user
# Usage: ./run_tests_as_postgres.sh [quick|comprehensive] [options]
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "ERROR: This script must be run as root to switch to postgres user"
    echo "Usage: sudo ./run_tests_as_postgres.sh [quick|comprehensive] [options]"
    exit 1
fi

# Check if postgres user exists
if ! id postgres &>/dev/null; then
    echo "ERROR: postgres user does not exist"
    echo "Please install PostgreSQL or create the postgres user"
    exit 1
fi

# Determine which test to run
TEST_TYPE="${1:-quick}"
shift || true

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Running tests as postgres user"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

case "$TEST_TYPE" in
    quick)
        echo "Executing: quick_test.sh"
        echo ""
        # Give postgres user access to the directory
        chmod -R 755 "$SCRIPT_DIR"
        # Run as postgres user
        su - postgres -c "cd '$SCRIPT_DIR' && bash quick_test.sh"
        ;;
    
    comprehensive|comp)
        echo "Executing: comprehensive_security_test.sh $*"
        echo ""
        # Give postgres user access to the directory
        chmod -R 755 "$SCRIPT_DIR"
        # Run as postgres user with any additional arguments
        su - postgres -c "cd '$SCRIPT_DIR' && bash comprehensive_security_test.sh $*"
        ;;
    
    *)
        echo "ERROR: Unknown test type: $TEST_TYPE"
        echo ""
        echo "Usage: sudo ./run_tests_as_postgres.sh [quick|comprehensive] [options]"
        echo ""
        echo "Examples:"
        echo "  sudo ./run_tests_as_postgres.sh quick"
        echo "  sudo ./run_tests_as_postgres.sh comprehensive --quick"
        echo "  sudo ./run_tests_as_postgres.sh comprehensive --cli-only"
        echo "  sudo ./run_tests_as_postgres.sh comprehensive --verbose"
        exit 1
        ;;
esac

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Test execution complete"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
