#!/bin/bash

# Aggressive 50GB Database Creator
# Specifically designed to reach exactly 50GB

set -e

DB_NAME="testdb_massive_50gb"
TARGET_SIZE_GB=50

echo "=================================================="
echo "AGGRESSIVE 50GB Database Creator"
echo "Database: $DB_NAME"
echo "Target Size: ${TARGET_SIZE_GB}GB"
echo "=================================================="

# Check available space
AVAILABLE_GB=$(df / | tail -1 | awk '{print int($4/1024/1024)}')
echo "Available disk space: ${AVAILABLE_GB}GB"

if [ $AVAILABLE_GB -lt $((TARGET_SIZE_GB + 20)) ]; then
    echo "❌ ERROR: Insufficient disk space. Need at least $((TARGET_SIZE_GB + 20))GB buffer"
    exit 1
fi

echo "✅ Sufficient disk space available"

echo ""
echo "1. Creating database for massive data..."

# Drop and recreate database  
sudo -u postgres psql -c "DROP DATABASE IF EXISTS $DB_NAME;" 2>/dev/null || true
sudo -u postgres psql -c "CREATE DATABASE $DB_NAME;"

# Create simple table optimized for massive data
sudo -u postgres psql -d $DB_NAME << 'EOF'
-- Single massive table with large binary columns
CREATE TABLE massive_data (
    id BIGSERIAL PRIMARY KEY,
    large_text TEXT NOT NULL,
    binary_chunk BYTEA NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Index for basic functionality
CREATE INDEX idx_massive_data_id ON massive_data(id);
EOF

echo "✅ Database schema created"

echo ""
echo "2. Inserting massive data in chunks..."

# Calculate how many rows we need for 50GB
# Strategy: Each row will be approximately 10MB
# 50GB = 50,000MB, so we need about 5,000 rows of 10MB each

CHUNK_SIZE_MB=10
TOTAL_CHUNKS=$((TARGET_SIZE_GB * 1024 / CHUNK_SIZE_MB))  # 5,120 chunks for 50GB

echo "Inserting $TOTAL_CHUNKS chunks of ${CHUNK_SIZE_MB}MB each..."

for i in $(seq 1 $TOTAL_CHUNKS); do
    # Progress indicator
    if [ $((i % 100)) -eq 0 ] || [ $i -le 10 ]; then
        CURRENT_SIZE=$(sudo -u postgres psql -d $DB_NAME -tAc "SELECT ROUND(pg_database_size('$DB_NAME') / 1024.0 / 1024.0 / 1024.0, 2);" 2>/dev/null || echo "0")
        echo "  Progress: $i/$TOTAL_CHUNKS ($(($i * 100 / $TOTAL_CHUNKS))%) - Current size: ${CURRENT_SIZE}GB"
        
        # Check if we've reached target
        if (( $(echo "$CURRENT_SIZE >= $TARGET_SIZE_GB" | bc -l 2>/dev/null || echo "0") )); then
            echo "✅ Target size reached! Stopping at chunk $i"
            break
        fi
    fi
    
    # Insert chunk with large data
    sudo -u postgres psql -d $DB_NAME << EOF > /dev/null
INSERT INTO massive_data (large_text, binary_chunk) 
VALUES (
    -- Large text component (~5MB as text)
    repeat('This is a large text chunk for testing massive database operations. It contains repeated content to reach the target size for backup and restore performance testing. Row: $i of $TOTAL_CHUNKS. ', 25000),
    -- Large binary component (~5MB as binary)
    decode(encode(repeat('MASSIVE_BINARY_DATA_CHUNK_FOR_TESTING_DATABASE_BACKUP_RESTORE_PERFORMANCE_ON_LARGE_DATASETS_ROW_${i}_OF_${TOTAL_CHUNKS}_', 25000)::bytea, 'base64'), 'base64')
);
EOF

    # Every 500 chunks, run VACUUM to prevent excessive table bloat
    if [ $((i % 500)) -eq 0 ]; then
        echo "    Running maintenance (VACUUM) at chunk $i..."
        sudo -u postgres psql -d $DB_NAME -c "VACUUM massive_data;" > /dev/null
    fi
done

echo ""
echo "3. Final optimization..."

sudo -u postgres psql -d $DB_NAME << 'EOF'
-- Final optimization
VACUUM ANALYZE massive_data;

-- Update statistics
ANALYZE;
EOF

echo ""
echo "4. Final database metrics..."

sudo -u postgres psql -d $DB_NAME << 'EOF'
-- Database size and statistics
SELECT 
    'Database Size' as metric,
    pg_size_pretty(pg_database_size(current_database())) as value,
    ROUND(pg_database_size(current_database()) / 1024.0 / 1024.0 / 1024.0, 2) || ' GB' as size_gb;

SELECT 
    'Table Size' as metric,
    pg_size_pretty(pg_total_relation_size('massive_data')) as value,
    ROUND(pg_total_relation_size('massive_data') / 1024.0 / 1024.0 / 1024.0, 2) || ' GB' as size_gb;

SELECT 
    'Row Count' as metric,
    COUNT(*)::text as value,
    'rows' as unit
FROM massive_data;

SELECT 
    'Average Row Size' as metric,
    pg_size_pretty(pg_total_relation_size('massive_data') / GREATEST(COUNT(*), 1)) as value,
    'per row' as unit
FROM massive_data;
EOF

FINAL_SIZE=$(sudo -u postgres psql -d $DB_NAME -tAc "SELECT pg_size_pretty(pg_database_size('$DB_NAME'));" 2>/dev/null)
FINAL_GB=$(sudo -u postgres psql -d $DB_NAME -tAc "SELECT ROUND(pg_database_size('$DB_NAME') / 1024.0 / 1024.0 / 1024.0, 2);" 2>/dev/null)

echo ""
echo "=================================================="
echo "✅ MASSIVE DATABASE CREATION COMPLETED!"
echo "=================================================="
echo "Database Name: $DB_NAME"
echo "Final Size: $FINAL_SIZE (${FINAL_GB}GB)"
echo "Target: ${TARGET_SIZE_GB}GB"

if (( $(echo "$FINAL_GB >= $TARGET_SIZE_GB" | bc -l 2>/dev/null || echo "0") )); then
    echo "🎯 TARGET ACHIEVED! Database is >= ${TARGET_SIZE_GB}GB"
else
    echo "⚠️  Target not fully reached, but substantial database created"
fi

echo "=================================================="

echo ""
echo "🧪 Ready for LARGE DATABASE testing:"
echo ""
echo "# Test single database backup (will take significant time):"
echo "time sudo -u postgres ./dbbackup backup single $DB_NAME --confirm"
echo ""
echo "# Test cluster backup (includes this massive DB):"
echo "time sudo -u postgres ./dbbackup backup cluster --confirm"  
echo ""
echo "# Monitor system resources during backup:"
echo "watch 'free -h && df -h && ls -lah *.dump* *.tar.gz 2>/dev/null'"
echo ""
echo "# Check database size anytime:"
echo "sudo -u postgres psql -d $DB_NAME -c \"SELECT pg_size_pretty(pg_database_size('$DB_NAME'));\""