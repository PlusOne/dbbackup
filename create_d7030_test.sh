#!/usr/bin/env bash
# create_d7030_test.sh
# Create a realistic d7030 database with tables, data, and many BLOBs to test large object restore

set -euo pipefail

DB_NAME="d7030"
NUM_DOCUMENTS=15000  # Number of documents with BLOBs (~750MB at 50KB each)
NUM_IMAGES=10000     # Number of image records (~900MB for images + ~100MB thumbnails)
# Total BLOBs: 25,000 large objects
# Approximate size: 15000*50KB + 10000*90KB + 10000*10KB = ~2.4GB in BLOBs alone
# With tables, indexes, and overhead: ~3-4GB per iteration
# We'll create multiple batches to reach ~25GB

echo "Creating database: $DB_NAME"

# Drop if exists
sudo -u postgres psql -c "DROP DATABASE IF EXISTS $DB_NAME;" 2>/dev/null || true

# Create database
sudo -u postgres psql -c "CREATE DATABASE $DB_NAME;"

echo "Creating schema and tables..."

# Enable pgcrypto extension for gen_random_bytes
sudo -u postgres psql -d "$DB_NAME" -c "CREATE EXTENSION IF NOT EXISTS pgcrypto;"

# Create schema with realistic business tables
sudo -u postgres psql -d "$DB_NAME" <<'EOF'
-- Create tables for a document management system
CREATE TABLE departments (
    dept_id SERIAL PRIMARY KEY,
    dept_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE employees (
    emp_id SERIAL PRIMARY KEY,
    dept_id INTEGER REFERENCES departments(dept_id),
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
    email VARCHAR(100) UNIQUE,
    hire_date DATE DEFAULT CURRENT_DATE
);

CREATE TABLE document_types (
    type_id SERIAL PRIMARY KEY,
    type_name VARCHAR(50) NOT NULL,
    description TEXT
);

-- Table with large objects (BLOBs)
CREATE TABLE documents (
    doc_id SERIAL PRIMARY KEY,
    emp_id INTEGER REFERENCES employees(emp_id),
    type_id INTEGER REFERENCES document_types(type_id),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    file_data OID,  -- Large object reference
    file_size INTEGER,
    mime_type VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE images (
    image_id SERIAL PRIMARY KEY,
    doc_id INTEGER REFERENCES documents(doc_id),
    image_name VARCHAR(255),
    image_data OID,  -- Large object reference
    thumbnail_data OID,  -- Another large object
    width INTEGER,
    height INTEGER,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE audit_log (
    log_id SERIAL PRIMARY KEY,
    table_name VARCHAR(50),
    record_id INTEGER,
    action VARCHAR(20),
    changed_by INTEGER,
    changed_at TIMESTAMP DEFAULT NOW(),
    details JSONB
);

-- Create indexes
CREATE INDEX idx_documents_emp ON documents(emp_id);
CREATE INDEX idx_documents_type ON documents(type_id);
CREATE INDEX idx_images_doc ON images(doc_id);
CREATE INDEX idx_audit_table ON audit_log(table_name, record_id);

-- Insert reference data
INSERT INTO departments (dept_name) VALUES 
    ('Engineering'), ('Sales'), ('Marketing'), ('HR'), ('Finance');

INSERT INTO document_types (type_name, description) VALUES
    ('Contract', 'Legal contracts and agreements'),
    ('Invoice', 'Financial invoices and receipts'),
    ('Report', 'Business reports and analysis'),
    ('Manual', 'Technical manuals and guides'),
    ('Presentation', 'Presentation slides and materials');

-- Insert employees
INSERT INTO employees (dept_id, first_name, last_name, email)
SELECT 
    (random() * 4 + 1)::INTEGER,
    'Employee_' || generate_series,
    'LastName_' || generate_series,
    'employee' || generate_series || '@d7030.com'
FROM generate_series(1, 50);

EOF

echo "Inserting documents with large objects (BLOBs)..."
echo "This will take several minutes to create ~25GB of data..."

# Create temporary files with random data for importing in postgres home
# Make documents larger for 25GB target: ~1MB each
TEMP_FILE="/var/lib/pgsql/test_blob_data.bin"
sudo dd if=/dev/urandom of="$TEMP_FILE" bs=1M count=1 2>/dev/null
sudo chown postgres:postgres "$TEMP_FILE"

# Create documents with actual large objects using lo_import
sudo -u postgres psql -d "$DB_NAME" <<EOF
DO \$\$
DECLARE
    v_emp_id INTEGER;
    v_type_id INTEGER;
    v_loid OID;
BEGIN
    FOR i IN 1..$NUM_DOCUMENTS LOOP
        -- Random employee and document type
        v_emp_id := (random() * 49 + 1)::INTEGER;
        v_type_id := (random() * 4 + 1)::INTEGER;
        
        -- Import file as large object (creates a unique BLOB for each)
        v_loid := lo_import('$TEMP_FILE');
        
        -- Insert document record
        INSERT INTO documents (emp_id, type_id, title, description, file_data, file_size, mime_type)
        VALUES (
            v_emp_id,
            v_type_id,
            'Document_' || i || '_' || (CASE v_type_id 
                WHEN 1 THEN 'Contract'
                WHEN 2 THEN 'Invoice'
                WHEN 3 THEN 'Report'
                WHEN 4 THEN 'Manual'
                ELSE 'Presentation'
            END),
            'This is a test document with large object data. Document number ' || i,
            v_loid,
            1048576,
            (CASE v_type_id 
                WHEN 1 THEN 'application/pdf'
                WHEN 2 THEN 'application/pdf'
                WHEN 3 THEN 'application/vnd.ms-excel'
                WHEN 4 THEN 'application/pdf'
                ELSE 'application/vnd.ms-powerpoint'
            END)
        );
        
        -- Progress indicator
        IF i % 500 = 0 THEN
            RAISE NOTICE 'Created % documents with BLOBs...', i;
        END IF;
    END LOOP;
END \$\$;
EOF

rm -f "$TEMP_FILE"

echo "Inserting images with large objects..."

# Create temp files for image and thumbnail in postgres home
# Make images larger: ~1.5MB for full image, ~200KB for thumbnail
TEMP_IMAGE="/var/lib/pgsql/test_image_data.bin"
TEMP_THUMB="/var/lib/pgsql/test_thumb_data.bin"
sudo dd if=/dev/urandom of="$TEMP_IMAGE" bs=1M count=1 bs=512K count=3 2>/dev/null
sudo dd if=/dev/urandom of="$TEMP_THUMB" bs=1K count=200 2>/dev/null
sudo chown postgres:postgres "$TEMP_IMAGE" "$TEMP_THUMB"

# Create images with multiple large objects per record
sudo -u postgres psql -d "$DB_NAME" <<EOF
DO \$\$
DECLARE
    v_doc_id INTEGER;
    v_image_oid OID;
    v_thumb_oid OID;
BEGIN
    FOR i IN 1..$NUM_IMAGES LOOP
        -- Random document (only from successfully created documents)
        SELECT doc_id INTO v_doc_id FROM documents ORDER BY random() LIMIT 1;
        
        IF v_doc_id IS NULL THEN
            EXIT; -- No documents exist, skip images
        END IF;
        
        -- Import full-size image as large object
        v_image_oid := lo_import('$TEMP_IMAGE');
        
        -- Import thumbnail as large object
        v_thumb_oid := lo_import('$TEMP_THUMB');
        
        -- Insert image record
        INSERT INTO images (doc_id, image_name, image_data, thumbnail_data, width, height)
        VALUES (
            v_doc_id,
            'Image_' || i || '.jpg',
            v_image_oid,
            v_thumb_oid,
            (random() * 2000 + 800)::INTEGER,
            (random() * 1500 + 600)::INTEGER
        );
        
        IF i % 500 = 0 THEN
            RAISE NOTICE 'Created % images with BLOBs...', i;
        END IF;
    END LOOP;
END \$\$;
EOF

rm -f "$TEMP_IMAGE" "$TEMP_THUMB"

echo "Inserting audit log data..."

# Create audit log entries
sudo -u postgres psql -d "$DB_NAME" <<EOF
INSERT INTO audit_log (table_name, record_id, action, changed_by, details)
SELECT 
    'documents',
    doc_id,
    (ARRAY['INSERT', 'UPDATE', 'VIEW'])[(random() * 2 + 1)::INTEGER],
    (random() * 49 + 1)::INTEGER,
    jsonb_build_object(
        'timestamp', NOW() - (random() * INTERVAL '90 days'),
        'ip_address', '192.168.' || (random() * 255)::INTEGER || '.' || (random() * 255)::INTEGER,
        'user_agent', 'Mozilla/5.0'
    )
FROM documents
CROSS JOIN generate_series(1, 3);
EOF

echo ""
echo "Database statistics:"
sudo -u postgres psql -d "$DB_NAME" <<'EOF'
SELECT 
    'Departments' as table_name, 
    COUNT(*) as row_count 
FROM departments
UNION ALL
SELECT 'Employees', COUNT(*) FROM employees
UNION ALL
SELECT 'Document Types', COUNT(*) FROM document_types
UNION ALL
SELECT 'Documents (with BLOBs)', COUNT(*) FROM documents
UNION ALL
SELECT 'Images (with BLOBs)', COUNT(*) FROM images
UNION ALL
SELECT 'Audit Log', COUNT(*) FROM audit_log;

-- Count large objects
SELECT COUNT(*) as total_large_objects FROM pg_largeobject_metadata;

-- Total size of large objects
SELECT pg_size_pretty(SUM(pg_column_size(data))) as total_blob_size 
FROM pg_largeobject;
EOF

echo ""
echo "✅ Database $DB_NAME created successfully with realistic data and BLOBs!"
echo ""
echo "Large objects created:"
echo "  - $NUM_DOCUMENTS documents (each with ~1MB BLOB)"
echo "  - $NUM_IMAGES images (each with 2 BLOBs: ~1.5MB image + ~200KB thumbnail)"
echo "  - Total: ~$((NUM_DOCUMENTS + NUM_IMAGES * 2)) large objects"
echo ""
echo "Estimated size: ~$((NUM_DOCUMENTS * 1 + NUM_IMAGES * 1 + NUM_IMAGES * 0))MB in BLOBs"
echo ""
echo "You can now backup this database and test restore with large object locks."
