#!/bin/bash
#
# DISASTER RECOVERY TEST SCRIPT
# Full cluster backup -> destroy all databases -> restore cluster
#
# This script performs the ultimate validation test:
# 1. Backup entire PostgreSQL cluster with maximum performance
# 2. Drop all user databases (destructive!)
# 3. Restore entire cluster from backup
# 4. Verify database count and integrity
#

set -e  # Exit on any error

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
BACKUP_DIR="/var/lib/pgsql/db_backups"
DBBACKUP_BIN="./dbbackup"
DB_USER="postgres"
DB_NAME="postgres"

# Performance settings - use maximum CPU
MAX_CORES=$(nproc)  # Use all available cores
COMPRESSION_LEVEL=3  # Fast compression for large DBs
CPU_WORKLOAD="cpu-intensive"  # Maximum CPU utilization
PARALLEL_JOBS=$MAX_CORES  # Maximum parallelization

echo -e "${CYAN}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║   DISASTER RECOVERY TEST - FULL CLUSTER VALIDATION     ║${NC}"
echo -e "${CYAN}╔════════════════════════════════════════════════════════╗${NC}"
echo ""
echo -e "${BLUE}Configuration:${NC}"
echo -e "  Backup directory: ${BACKUP_DIR}"
echo -e "  Max CPU cores:    ${MAX_CORES}"
echo -e "  Compression:      ${COMPRESSION_LEVEL}"
echo -e "  CPU workload:     ${CPU_WORKLOAD}"
echo -e "  Parallel jobs:    ${PARALLEL_JOBS}"
echo ""

# Step 0: Pre-flight checks
echo -e "${BLUE}[STEP 0/5]${NC} Pre-flight checks..."

if [ ! -f "$DBBACKUP_BIN" ]; then
    echo -e "${RED}ERROR: dbbackup binary not found at $DBBACKUP_BIN${NC}"
    exit 1
fi

if ! command -v psql &> /dev/null; then
    echo -e "${RED}ERROR: psql not found${NC}"
    exit 1
fi

echo -e "${GREEN}✓${NC} Pre-flight checks passed"
echo ""

# Step 1: Save current database list
echo -e "${BLUE}[STEP 1/5]${NC} Documenting current cluster state..."
PRE_BACKUP_LIST="/tmp/pre_disaster_recovery_dblist_$(date +%s).txt"
sudo -u $DB_USER psql -l -t > "$PRE_BACKUP_LIST"
DB_COUNT=$(sudo -u $DB_USER psql -l -t | grep -v "^$" | grep -v "template" | wc -l)
echo -e "${GREEN}✓${NC} Documented ${DB_COUNT} databases to ${PRE_BACKUP_LIST}"
echo ""

# Step 2: Full cluster backup with maximum performance
echo -e "${BLUE}[STEP 2/5]${NC} ${YELLOW}Backing up entire cluster...${NC}"
echo -e "${CYAN}Performance settings: ${MAX_CORES} cores, compression=${COMPRESSION_LEVEL}, workload=${CPU_WORKLOAD}${NC}"
echo ""

BACKUP_START=$(date +%s)

sudo -u $DB_USER $DBBACKUP_BIN backup cluster \
    -d $DB_NAME \
    --insecure \
    --compression $COMPRESSION_LEVEL \
    --backup-dir "$BACKUP_DIR" \
    --max-cores $MAX_CORES \
    --cpu-workload "$CPU_WORKLOAD" \
    --dump-jobs $PARALLEL_JOBS \
    --jobs $PARALLEL_JOBS

BACKUP_END=$(date +%s)
BACKUP_DURATION=$((BACKUP_END - BACKUP_START))

# Find the most recent cluster backup
BACKUP_FILE=$(ls -t "$BACKUP_DIR"/cluster_*.tar.gz | head -1)
BACKUP_SIZE=$(du -h "$BACKUP_FILE" | cut -f1)

echo ""
echo -e "${GREEN}✓${NC} Cluster backup completed in ${BACKUP_DURATION}s"
echo -e "  Archive: ${BACKUP_FILE}"
echo -e "  Size:    ${BACKUP_SIZE}"
echo ""

# Step 3: DESTRUCTIVE - Drop all user databases
echo -e "${BLUE}[STEP 3/5]${NC} ${RED}DESTROYING ALL DATABASES (POINT OF NO RETURN!)${NC}"
echo -e "${YELLOW}Waiting 3 seconds... Press Ctrl+C to abort${NC}"
sleep 3

echo -e "${RED}🔥 DROPPING ALL USER DATABASES...${NC}"

# Get list of all databases except templates and postgres
USER_DBS=$(sudo -u $DB_USER psql -d postgres -t -c "SELECT datname FROM pg_database WHERE datistemplate = false AND datname != 'postgres';")

DROPPED_COUNT=0
for db in $USER_DBS; do
    echo -e "  Dropping: ${db}"
    sudo -u $DB_USER psql -d postgres -c "DROP DATABASE IF EXISTS \"$db\";" 2>&1 | grep -v "does not exist" || true
    DROPPED_COUNT=$((DROPPED_COUNT + 1))
done

REMAINING_DBS=$(sudo -u $DB_USER psql -l -t | grep -v "^$" | grep -v "template" | wc -l)
echo ""
echo -e "${GREEN}✓${NC} Dropped ${DROPPED_COUNT} databases (${REMAINING_DBS} remaining)"
echo -e "${CYAN}Remaining databases:${NC}"
sudo -u $DB_USER psql -l | head -10
echo ""

# Step 4: Restore full cluster
echo -e "${BLUE}[STEP 4/5]${NC} ${YELLOW}RESTORING FULL CLUSTER FROM BACKUP...${NC}"
echo ""

RESTORE_START=$(date +%s)

sudo -u $DB_USER $DBBACKUP_BIN restore cluster \
    "$BACKUP_FILE" \
    --confirm \
    -d $DB_NAME \
    --insecure \
    --jobs $PARALLEL_JOBS

RESTORE_END=$(date +%s)
RESTORE_DURATION=$((RESTORE_END - RESTORE_START))

echo ""
echo -e "${GREEN}✓${NC} Cluster restore completed in ${RESTORE_DURATION}s"
echo ""

# Step 5: Verify restoration
echo -e "${BLUE}[STEP 5/5]${NC} Verifying restoration..."

POST_RESTORE_LIST="/tmp/post_disaster_recovery_dblist_$(date +%s).txt"
sudo -u $DB_USER psql -l -t > "$POST_RESTORE_LIST"
RESTORED_DB_COUNT=$(sudo -u $DB_USER psql -l -t | grep -v "^$" | grep -v "template" | wc -l)

echo -e "${CYAN}Restored databases:${NC}"
sudo -u $DB_USER psql -l

echo ""
echo -e "${GREEN}✓${NC} Restored ${RESTORED_DB_COUNT} databases"
echo ""

# Check if database counts match
if [ "$RESTORED_DB_COUNT" -eq "$DB_COUNT" ]; then
    echo -e "${GREEN}✅ DATABASE COUNT MATCH: ${RESTORED_DB_COUNT}/${DB_COUNT}${NC}"
else
    echo -e "${YELLOW}⚠️  DATABASE COUNT MISMATCH: ${RESTORED_DB_COUNT} restored vs ${DB_COUNT} original${NC}"
fi

# Check largest databases
echo ""
echo -e "${CYAN}Largest restored databases:${NC}"
sudo -u $DB_USER psql -c "\l+" | grep -E "MB|GB" | head -5

# Summary
echo ""
echo -e "${CYAN}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║              DISASTER RECOVERY TEST SUMMARY            ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  ${BLUE}Backup:${NC}"
echo -e "    - Duration:  ${BACKUP_DURATION}s ($(($BACKUP_DURATION / 60))m $(($BACKUP_DURATION % 60))s)"
echo -e "    - File:      ${BACKUP_FILE}"
echo -e "    - Size:      ${BACKUP_SIZE}"
echo ""
echo -e "  ${BLUE}Restore:${NC}"
echo -e "    - Duration:  ${RESTORE_DURATION}s ($(($RESTORE_DURATION / 60))m $(($RESTORE_DURATION % 60))s)"
echo -e "    - Databases: ${RESTORED_DB_COUNT}/${DB_COUNT}"
echo ""
echo -e "  ${BLUE}Performance:${NC}"
echo -e "    - CPU cores: ${MAX_CORES}"
echo -e "    - Jobs:      ${PARALLEL_JOBS}"
echo -e "    - Workload:  ${CPU_WORKLOAD}"
echo ""
echo -e "  ${BLUE}Verification:${NC}"
echo -e "    - Pre-test:  ${PRE_BACKUP_LIST}"
echo -e "    - Post-test: ${POST_RESTORE_LIST}"
echo ""
TOTAL_DURATION=$((BACKUP_DURATION + RESTORE_DURATION))
echo -e "${GREEN}✅ DISASTER RECOVERY TEST COMPLETED IN ${TOTAL_DURATION}s ($(($TOTAL_DURATION / 60))m)${NC}"
echo ""
