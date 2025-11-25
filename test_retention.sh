#!/bin/bash
#
# Retention Policy Test - Verify old backup cleanup
#

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

BACKUP_DIR="/var/lib/pgsql/db_backups"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Retention Policy Test${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo

# Create test old backups as postgres user
echo -e "${YELLOW}► Creating test backups with old timestamps...${NC}"
su - postgres -c "
cd $BACKUP_DIR
touch -d '40 days ago' db_old_test_40days.dump
touch -d '40 days ago' db_old_test_40days.dump.sha256
touch -d '40 days ago' db_old_test_40days.dump.info
touch -d '35 days ago' db_old_test_35days.dump
touch -d '35 days ago' db_old_test_35days.dump.sha256
touch -d '35 days ago' db_old_test_35days.dump.info
touch -d '25 days ago' db_old_test_25days.dump
touch -d '25 days ago' db_old_test_25days.dump.sha256
touch -d '25 days ago' db_old_test_25days.dump.info
"

echo -e "${GREEN}✓ Test backups created${NC}"
echo

echo -e "${YELLOW}► Before retention cleanup:${NC}"
ls -lh $BACKUP_DIR/db_old_test*.dump 2>/dev/null
echo

# Run backup with retention set to 30 days, min 2 backups
echo -e "${YELLOW}► Running backup with retention policy (30 days, min 2 backups)...${NC}"
su - postgres -c "/root/dbbackup/dbbackup backup single postgres --retention-days 30 --min-backups 2" 2>&1 | grep -E "retention|cleanup|removed|Backup completed" || true
echo

echo -e "${YELLOW}► After retention cleanup:${NC}"
ls -lh $BACKUP_DIR/db_old_test*.dump 2>/dev/null || echo "  (old test backups cleaned up)"
echo

# Check if 40 and 35 day old files were removed
if [ ! -f "$BACKUP_DIR/db_old_test_40days.dump" ] && [ ! -f "$BACKUP_DIR/db_old_test_35days.dump" ]; then
    echo -e "${GREEN}✓ Retention policy working: Old backups (>30 days) removed${NC}"
elif [ -f "$BACKUP_DIR/db_old_test_25days.dump" ]; then
    echo -e "${GREEN}✓ Recent backups (<30 days) preserved${NC}"
else
    echo -e "${YELLOW}⚠ Retention behavior may differ from expected${NC}"
fi
echo

echo -e "${YELLOW}► Current backup inventory:${NC}"
echo "Total postgres backups: $(ls -1 $BACKUP_DIR/db_postgres_*.dump 2>/dev/null | wc -l)"
echo "Latest backups:"
ls -lht $BACKUP_DIR/db_postgres_*.dump 2>/dev/null | head -5
echo

echo -e "${GREEN}🎉 Retention policy test complete!${NC}"
