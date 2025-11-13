#!/bin/bash
# Verify that backup contains large objects (BLOBs)

if [ $# -eq 0 ]; then
    echo "Usage: $0 <backup_file.dump>"
    echo "Example: $0 /var/lib/pgsql/db_backups/d7030.dump"
    exit 1
fi

BACKUP_FILE="$1"

if [ ! -f "$BACKUP_FILE" ]; then
    echo "Error: File not found: $BACKUP_FILE"
    exit 1
fi

echo "========================================="
echo "Backup BLOB/Large Object Verification"
echo "========================================="
echo "File: $BACKUP_FILE"
echo ""

# Check if file is a valid PostgreSQL dump
echo "1. Checking dump file format..."
pg_restore -l "$BACKUP_FILE" > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "   ✅ Valid PostgreSQL custom format dump"
else
    echo "   ❌ Not a valid pg_dump custom format file"
    exit 1
fi

# List table of contents and look for BLOB entries
echo ""
echo "2. Checking for BLOB/Large Object entries..."
BLOB_COUNT=$(pg_restore -l "$BACKUP_FILE" | grep -i "BLOB\|LARGE OBJECT" | wc -l)

if [ $BLOB_COUNT -gt 0 ]; then
    echo "   ✅ Found $BLOB_COUNT large object entries in backup"
    echo ""
    echo "   Sample entries:"
    pg_restore -l "$BACKUP_FILE" | grep -i "BLOB\|LARGE OBJECT" | head -10
else
    echo "   ⚠️  No large object entries found"
    echo "   This could mean:"
    echo "   - Database has no large objects (normal)"
    echo "   - Backup was created without --blobs flag (problem)"
fi

echo ""
echo "3. Full table of contents summary..."
pg_restore -l "$BACKUP_FILE" | tail -20

echo ""
echo "========================================="
echo "Verification complete"
echo "========================================="
