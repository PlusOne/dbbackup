#!/bin/bash

# Optimized Large Database Creator - 50GB target
# More efficient approach using PostgreSQL's built-in functions

set -e

DB_NAME="testdb_50gb"
TARGET_SIZE_GB=50

echo "=================================================="
echo "OPTIMIZED Large Test Database Creator"
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
echo "1. Creating optimized database schema..."

# Drop and recreate database  
sudo -u postgres psql -c "DROP DATABASE IF EXISTS $DB_NAME;" 2>/dev/null || true
sudo -u postgres psql -c "CREATE DATABASE $DB_NAME;"

# Create optimized schema for rapid data generation
sudo -u postgres psql -d $DB_NAME << 'EOF'
-- Large blob table with efficient storage
CREATE TABLE mega_blobs (
    id BIGSERIAL PRIMARY KEY,
    chunk_id INTEGER NOT NULL,
    blob_data BYTEA NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Massive text table for document storage  
CREATE TABLE big_documents (
    id BIGSERIAL PRIMARY KEY,
    doc_name VARCHAR(100),
    content TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

-- High-volume metrics table
CREATE TABLE huge_metrics (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL,
    sensor_id INTEGER NOT NULL,
    metric_type VARCHAR(50) NOT NULL,
    value_data TEXT NOT NULL,  -- Large text field
    binary_payload BYTEA,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for realism
CREATE INDEX idx_mega_blobs_chunk ON mega_blobs(chunk_id);
CREATE INDEX idx_big_docs_name ON big_documents(doc_name);
CREATE INDEX idx_huge_metrics_timestamp ON huge_metrics(timestamp);
CREATE INDEX idx_huge_metrics_sensor ON huge_metrics(sensor_id);
EOF

echo "✅ Optimized schema created"

echo ""
echo "2. Generating large-scale data using PostgreSQL's generate_series..."

# Strategy: Use PostgreSQL's efficient bulk operations
echo "Inserting massive text documents (targeting ~20GB)..."

sudo -u postgres psql -d $DB_NAME << 'EOF'
-- Insert 2 million large text documents (~20GB estimated)
INSERT INTO big_documents (doc_name, content, metadata)
SELECT 
    'doc_' || generate_series,
    -- Each document: ~10KB of text content
    repeat('Lorem ipsum dolor sit amet, consectetur adipiscing elit. ' ||
           'Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. ' ||
           'Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris. ' ||
           'Duis aute irure dolor in reprehenderit in voluptate velit esse cillum. ' ||
           'Excepteur sint occaecat cupidatat non proident, sunt in culpa qui. ' ||
           'Nulla pariatur. Sed ut perspiciatis unde omnis iste natus error sit. ' ||
           'At vero eos et accusamus et iusto odio dignissimos ducimus qui blanditiis. ' ||
           'Document content section ' || generate_series || '. ', 50),
    ('{"doc_type": "test", "size_category": "large", "batch": ' || (generate_series / 10000) || 
     ', "tags": ["bulk_data", "test_doc", "large_dataset"]}')::jsonb
FROM generate_series(1, 2000000);
EOF

echo "✅ Large documents inserted"

# Check current size
CURRENT_SIZE=$(sudo -u postgres psql -d $DB_NAME -tAc "SELECT pg_database_size('$DB_NAME') / 1024 / 1024 / 1024.0;" 2>/dev/null)
echo "Current database size: ${CURRENT_SIZE}GB"

echo "Inserting high-volume metrics data (targeting additional ~15GB)..."

sudo -u postgres psql -d $DB_NAME << 'EOF'
-- Insert 5 million metrics records with large payloads (~15GB estimated)
INSERT INTO huge_metrics (timestamp, sensor_id, metric_type, value_data, binary_payload)
SELECT 
    NOW() - (generate_series * INTERVAL '1 second'),
    generate_series % 10000,  -- 10,000 different sensors
    CASE (generate_series % 5)
        WHEN 0 THEN 'temperature'
        WHEN 1 THEN 'humidity'
        WHEN 2 THEN 'pressure'  
        WHEN 3 THEN 'vibration'
        ELSE 'electromagnetic'
    END,
    -- Large JSON-like text payload (~3KB each)
    '{"readings": [' || 
    '{"timestamp": "' || (NOW() - (generate_series * INTERVAL '1 second'))::text || 
    '", "value": ' || (random() * 1000)::int || 
    ', "quality": "good", "metadata": "' || repeat('data_', 20) || '"},' ||
    '{"timestamp": "' || (NOW() - ((generate_series + 1) * INTERVAL '1 second'))::text || 
    '", "value": ' || (random() * 1000)::int || 
    ', "quality": "good", "metadata": "' || repeat('data_', 20) || '"},' ||
    '{"timestamp": "' || (NOW() - ((generate_series + 2) * INTERVAL '1 second'))::text || 
    '", "value": ' || (random() * 1000)::int || 
    ', "quality": "good", "metadata": "' || repeat('data_', 20) || '"}' ||
    '], "sensor_info": "' || repeat('sensor_metadata_', 30) || 
    '", "calibration": "' || repeat('calibration_data_', 25) || '"}',
    -- Binary payload (~1KB each)  
    decode(encode(repeat('BINARY_SENSOR_DATA_CHUNK_', 25)::bytea, 'base64'), 'base64')
FROM generate_series(1, 5000000);
EOF

echo "✅ Metrics data inserted"

# Check size again
CURRENT_SIZE=$(sudo -u postgres psql -d $DB_NAME -tAc "SELECT pg_database_size('$DB_NAME') / 1024 / 1024 / 1024.0;" 2>/dev/null)
echo "Current database size: ${CURRENT_SIZE}GB"

echo "Inserting binary blob data to reach 50GB target..."

# Calculate remaining size needed
REMAINING_GB=$(echo "$TARGET_SIZE_GB - $CURRENT_SIZE" | bc -l 2>/dev/null || echo "15")
REMAINING_MB=$(echo "$REMAINING_GB * 1024" | bc -l 2>/dev/null || echo "15360")

echo "Need approximately ${REMAINING_GB}GB more data..."

# Insert binary blobs to fill remaining space
sudo -u postgres psql -d $DB_NAME << EOF
-- Insert large binary chunks to reach target size
-- Each blob will be approximately 5MB
INSERT INTO mega_blobs (chunk_id, blob_data)
SELECT 
    generate_series,
    -- Generate ~5MB of binary data per row
    decode(encode(repeat('LARGE_BINARY_CHUNK_FOR_TESTING_PURPOSES_', 100000)::bytea, 'base64'), 'base64')
FROM generate_series(1, ${REMAINING_MB%.*} / 5);
EOF

echo "✅ Binary blob data inserted"

echo ""
echo "3. Final optimization and statistics..."

# Analyze tables for accurate statistics
sudo -u postgres psql -d $DB_NAME << 'EOF'
-- Update table statistics
ANALYZE big_documents;
ANALYZE huge_metrics;  
ANALYZE mega_blobs;

-- Vacuum to optimize storage
VACUUM ANALYZE;
EOF

echo ""
echo "4. Final database metrics..."

sudo -u postgres psql -d $DB_NAME << 'EOF'
-- Database size breakdown
SELECT 
    'TOTAL DATABASE SIZE' as component,
    pg_size_pretty(pg_database_size(current_database())) as size,
    ROUND(pg_database_size(current_database()) / 1024.0 / 1024.0 / 1024.0, 2) || ' GB' as size_gb
UNION ALL
SELECT 
    'big_documents table',
    pg_size_pretty(pg_total_relation_size('big_documents')),
    ROUND(pg_total_relation_size('big_documents') / 1024.0 / 1024.0 / 1024.0, 2) || ' GB'
UNION ALL
SELECT 
    'huge_metrics table',
    pg_size_pretty(pg_total_relation_size('huge_metrics')),
    ROUND(pg_total_relation_size('huge_metrics') / 1024.0 / 1024.0 / 1024.0, 2) || ' GB'
UNION ALL
SELECT 
    'mega_blobs table', 
    pg_size_pretty(pg_total_relation_size('mega_blobs')),
    ROUND(pg_total_relation_size('mega_blobs') / 1024.0 / 1024.0 / 1024.0, 2) || ' GB';

-- Row counts
SELECT 
    'TABLE ROWS' as metric,
    '' as value,
    '' as extra
UNION ALL
SELECT 
    'big_documents',
    COUNT(*)::text,
    'rows'
FROM big_documents
UNION ALL
SELECT 
    'huge_metrics', 
    COUNT(*)::text,
    'rows'
FROM huge_metrics
UNION ALL
SELECT 
    'mega_blobs',
    COUNT(*)::text, 
    'rows'
FROM mega_blobs;
EOF

FINAL_SIZE=$(sudo -u postgres psql -d $DB_NAME -tAc "SELECT pg_size_pretty(pg_database_size('$DB_NAME'));" 2>/dev/null)
FINAL_GB=$(sudo -u postgres psql -d $DB_NAME -tAc "SELECT ROUND(pg_database_size('$DB_NAME') / 1024.0 / 1024.0 / 1024.0, 2);" 2>/dev/null)

echo ""
echo "=================================================="
echo "✅ LARGE DATABASE CREATION COMPLETED!"
echo "=================================================="
echo "Database Name: $DB_NAME"
echo "Final Size: $FINAL_SIZE (${FINAL_GB}GB)"
echo "Target: ${TARGET_SIZE_GB}GB"
echo "=================================================="

echo ""
echo "🧪 Ready for testing large database operations:"
echo ""
echo "# Test single database backup:"
echo "time sudo -u postgres ./dbbackup backup single $DB_NAME --confirm"
echo ""
echo "# Test cluster backup (includes this large DB):"
echo "time sudo -u postgres ./dbbackup backup cluster --confirm"  
echo ""
echo "# Monitor backup progress:"
echo "watch 'ls -lah /backup/ 2>/dev/null || ls -lah ./*.dump* ./*.tar.gz 2>/dev/null'"
echo ""
echo "# Check database size anytime:"
echo "sudo -u postgres psql -d $DB_NAME -c \"SELECT pg_size_pretty(pg_database_size('$DB_NAME'));\""