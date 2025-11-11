#!/bin/bash

# Large Test Database Creator - 50GB with Blobs
# Creates a substantial database for testing backup/restore performance on large datasets

set -e

DB_NAME="testdb_large_50gb"
TARGET_SIZE_GB=50
CHUNK_SIZE_MB=10  # Size of each blob chunk in MB
TOTAL_CHUNKS=$((TARGET_SIZE_GB * 1024 / CHUNK_SIZE_MB))  # Total number of chunks needed

echo "=================================================="
echo "Creating Large Test Database: $DB_NAME"
echo "Target Size: ${TARGET_SIZE_GB}GB"
echo "Chunk Size: ${CHUNK_SIZE_MB}MB"
echo "Total Chunks: $TOTAL_CHUNKS"
echo "=================================================="

# Check available space
AVAILABLE_GB=$(df / | tail -1 | awk '{print int($4/1024/1024)}')
echo "Available disk space: ${AVAILABLE_GB}GB"

if [ $AVAILABLE_GB -lt $((TARGET_SIZE_GB + 10)) ]; then
    echo "❌ ERROR: Insufficient disk space. Need at least $((TARGET_SIZE_GB + 10))GB"
    exit 1
fi

echo "✅ Sufficient disk space available"

# Database connection settings
PGUSER="postgres"
PGHOST="localhost"
PGPORT="5432"

echo ""
echo "1. Creating database and schema..."

# Drop and recreate database
sudo -u postgres psql -c "DROP DATABASE IF EXISTS $DB_NAME;" 2>/dev/null || true
sudo -u postgres psql -c "CREATE DATABASE $DB_NAME;"

# Create tables with different data types
sudo -u postgres psql -d $DB_NAME << 'EOF'
-- Table for large binary objects (blobs)
CREATE TABLE large_blobs (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255),
    description TEXT,
    blob_data BYTEA,
    created_at TIMESTAMP DEFAULT NOW(),
    size_mb INTEGER
);

-- Table for structured data with indexes
CREATE TABLE test_data (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    username VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL,
    profile_data JSONB,
    large_text TEXT,
    random_number NUMERIC(15,2),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Table for time series data (lots of rows)
CREATE TABLE metrics (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL,
    metric_name VARCHAR(100) NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    tags JSONB,
    metadata TEXT
);

-- Indexes for performance
CREATE INDEX idx_test_data_user_id ON test_data(user_id);
CREATE INDEX idx_test_data_email ON test_data(email);
CREATE INDEX idx_test_data_created ON test_data(created_at);
CREATE INDEX idx_metrics_timestamp ON metrics(timestamp);
CREATE INDEX idx_metrics_name ON metrics(metric_name);
CREATE INDEX idx_metrics_tags ON metrics USING GIN(tags);

-- Large text table for document storage
CREATE TABLE documents (
    id SERIAL PRIMARY KEY,
    title VARCHAR(500),
    content TEXT,
    document_data BYTEA,
    tags TEXT[],
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_documents_tags ON documents USING GIN(tags);
EOF

echo "✅ Database schema created"

echo ""
echo "2. Generating large blob data..."

# Function to generate random data
generate_blob_data() {
    local chunk_num=$1
    local size_mb=$2
    
    # Generate random binary data using dd and base64
    dd if=/dev/urandom bs=1M count=$size_mb 2>/dev/null | base64 -w 0
}

echo "Inserting $TOTAL_CHUNKS blob chunks of ${CHUNK_SIZE_MB}MB each..."

# Insert blob data in chunks
for i in $(seq 1 $TOTAL_CHUNKS); do
    echo -n "  Progress: $i/$TOTAL_CHUNKS ($(($i * 100 / $TOTAL_CHUNKS))%) - "
    
    # Generate blob data
    BLOB_DATA=$(generate_blob_data $i $CHUNK_SIZE_MB)
    
    # Insert into database
    sudo -u postgres psql -d $DB_NAME -c "
        INSERT INTO large_blobs (name, description, blob_data, size_mb) 
        VALUES (
            'blob_chunk_$i',
            'Large binary data chunk $i of $TOTAL_CHUNKS for testing backup/restore performance',
            decode('$BLOB_DATA', 'base64'),
            $CHUNK_SIZE_MB
        );" > /dev/null
    
    echo "✅ Chunk $i inserted"
    
    # Every 10 chunks, show current database size
    if [ $((i % 10)) -eq 0 ]; then
        CURRENT_SIZE=$(sudo -u postgres psql -d $DB_NAME -tAc "
            SELECT pg_size_pretty(pg_database_size('$DB_NAME'));" 2>/dev/null || echo "Unknown")
        echo "    Current database size: $CURRENT_SIZE"
    fi
done

echo ""
echo "3. Generating structured test data..."

# Insert large amounts of structured data
sudo -u postgres psql -d $DB_NAME << 'EOF'
-- Insert 1 million rows of test data (will add significant size)
INSERT INTO test_data (user_id, username, email, profile_data, large_text, random_number)
SELECT 
    generate_series % 100000 as user_id,
    'user_' || generate_series as username,
    'user_' || generate_series || '@example.com' as email,
    ('{"preferences": {"theme": "dark", "language": "en", "notifications": true}, "metadata": {"last_login": "2024-01-01", "session_count": ' || (generate_series % 1000) || ', "data": "' || repeat('x', 100) || '"}}')::jsonb as profile_data,
    repeat('This is large text content for testing. ', 50) || ' Row: ' || generate_series as large_text,
    random() * 1000000 as random_number
FROM generate_series(1, 1000000);

-- Insert time series data (2 million rows)
INSERT INTO metrics (timestamp, metric_name, value, tags, metadata)
SELECT 
    NOW() - (generate_series || ' minutes')::interval as timestamp,
    CASE (generate_series % 5)
        WHEN 0 THEN 'cpu_usage'
        WHEN 1 THEN 'memory_usage' 
        WHEN 2 THEN 'disk_io'
        WHEN 3 THEN 'network_tx'
        ELSE 'network_rx'
    END as metric_name,
    random() * 100 as value,
    ('{"host": "server_' || (generate_series % 100) || '", "env": "' || 
     CASE (generate_series % 3) WHEN 0 THEN 'prod' WHEN 1 THEN 'staging' ELSE 'dev' END || 
     '", "region": "us-' || CASE (generate_series % 2) WHEN 0 THEN 'east' ELSE 'west' END || '"}')::jsonb as tags,
    'Generated metric data for testing - ' || repeat('metadata_', 10) as metadata
FROM generate_series(1, 2000000);

-- Insert document data with embedded binary content
INSERT INTO documents (title, content, document_data, tags)
SELECT 
    'Document ' || generate_series as title,
    repeat('This is document content with lots of text to increase database size. ', 100) || 
    ' Document ID: ' || generate_series || '. ' ||
    repeat('Additional content to make documents larger. ', 20) as content,
    decode(encode(('Binary document data for doc ' || generate_series || ': ' || repeat('BINARY_DATA_', 1000))::bytea, 'base64'), 'base64') as document_data,
    ARRAY['tag_' || (generate_series % 10), 'category_' || (generate_series % 5), 'type_document'] as tags
FROM generate_series(1, 100000);
EOF

echo "✅ Structured data inserted"

echo ""
echo "4. Final database statistics..."

# Get final database size and statistics
sudo -u postgres psql -d $DB_NAME << 'EOF'
SELECT 
    'Database Size' as metric,
    pg_size_pretty(pg_database_size(current_database())) as value
UNION ALL
SELECT 
    'Table: large_blobs',
    pg_size_pretty(pg_total_relation_size('large_blobs'))
UNION ALL
SELECT 
    'Table: test_data', 
    pg_size_pretty(pg_total_relation_size('test_data'))
UNION ALL
SELECT 
    'Table: metrics',
    pg_size_pretty(pg_total_relation_size('metrics'))
UNION ALL
SELECT 
    'Table: documents',
    pg_size_pretty(pg_total_relation_size('documents'));

-- Row counts
SELECT 'large_blobs rows' as table_name, COUNT(*) as row_count FROM large_blobs
UNION ALL
SELECT 'test_data rows', COUNT(*) FROM test_data  
UNION ALL
SELECT 'metrics rows', COUNT(*) FROM metrics
UNION ALL
SELECT 'documents rows', COUNT(*) FROM documents;
EOF

echo ""
echo "=================================================="
echo "✅ Large test database creation completed!"
echo "Database: $DB_NAME"
echo "=================================================="

# Show final size
FINAL_SIZE=$(sudo -u postgres psql -d $DB_NAME -tAc "SELECT pg_size_pretty(pg_database_size('$DB_NAME'));" 2>/dev/null)
echo "Final database size: $FINAL_SIZE"

echo ""
echo "You can now test backup/restore operations:"
echo "  # Backup the large database"
echo "  sudo -u postgres ./dbbackup backup single $DB_NAME"
echo ""
echo "  # Backup entire cluster (including this large DB)"  
echo "  sudo -u postgres ./dbbackup backup cluster"
echo ""
echo "  # Check database size anytime:"
echo "  sudo -u postgres psql -d $DB_NAME -c \"SELECT pg_size_pretty(pg_database_size('$DB_NAME'));\""