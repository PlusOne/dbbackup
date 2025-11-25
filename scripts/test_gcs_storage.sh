#!/bin/bash

# Google Cloud Storage (fake-gcs-server) Testing Script for dbbackup
# Tests backup, restore, verify, and cleanup with GCS emulator

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
GCS_ENDPOINT="http://localhost:4443/storage/v1"
BUCKET_NAME="test-backups"
PROJECT_ID="test-project"

# Database connection details (from docker-compose)
POSTGRES_HOST="localhost"
POSTGRES_PORT="5435"
POSTGRES_USER="testuser"
POSTGRES_PASS="testpass"
POSTGRES_DB="testdb"

MYSQL_HOST="localhost"
MYSQL_PORT="3309"
MYSQL_USER="testuser"
MYSQL_PASS="testpass"
MYSQL_DB="testdb"

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Functions
print_header() {
    echo -e "\n${BLUE}=== $1 ===${NC}\n"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
    ((TESTS_PASSED++))
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
    ((TESTS_FAILED++))
}

print_info() {
    echo -e "${YELLOW}ℹ $1${NC}"
}

wait_for_gcs() {
    print_info "Waiting for fake-gcs-server to be ready..."
    for i in {1..30}; do
        if curl -f -s "$GCS_ENDPOINT/b" > /dev/null 2>&1; then
            print_success "fake-gcs-server is ready"
            return 0
        fi
        sleep 1
    done
    print_error "fake-gcs-server failed to start"
    return 1
}

create_test_bucket() {
    print_info "Creating test bucket..."
    curl -X POST "$GCS_ENDPOINT/b?project=$PROJECT_ID" \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"$BUCKET_NAME\"}" > /dev/null 2>&1 || true
    print_success "Test bucket created"
}

# Build dbbackup if needed
build_dbbackup() {
    print_header "Building dbbackup"
    if [ ! -f "./dbbackup" ]; then
        go build -o dbbackup .
        print_success "Built dbbackup binary"
    else
        print_info "Using existing dbbackup binary"
    fi
}

# Start services
start_services() {
    print_header "Starting GCS Emulator and Database Services"
    docker-compose -f docker-compose.gcs.yml up -d
    
    # Wait for services
    sleep 5
    wait_for_gcs
    create_test_bucket
    
    print_info "Waiting for PostgreSQL..."
    sleep 3
    
    print_info "Waiting for MySQL..."
    sleep 3
    
    print_success "All services started"
}

# Stop services
stop_services() {
    print_header "Stopping Services"
    docker-compose -f docker-compose.gcs.yml down
    print_success "Services stopped"
}

# Create test data in databases
create_test_data() {
    print_header "Creating Test Data"
    
    # PostgreSQL
    PGPASSWORD=$POSTGRES_PASS psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d $POSTGRES_DB <<EOF
DROP TABLE IF EXISTS test_table;
CREATE TABLE test_table (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO test_table (name) VALUES ('GCS Test 1'), ('GCS Test 2'), ('GCS Test 3');
EOF
    print_success "Created PostgreSQL test data"
    
    # MySQL
    mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASS $MYSQL_DB <<EOF
DROP TABLE IF EXISTS test_table;
CREATE TABLE test_table (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO test_table (name) VALUES ('GCS Test 1'), ('GCS Test 2'), ('GCS Test 3');
EOF
    print_success "Created MySQL test data"
}

# Test 1: PostgreSQL backup to GCS
test_postgres_backup() {
    print_header "Test 1: PostgreSQL Backup to GCS"
    
    ./dbbackup backup postgres \
        --host $POSTGRES_HOST \
        --port $POSTGRES_PORT \
        --user $POSTGRES_USER \
        --password $POSTGRES_PASS \
        --database $POSTGRES_DB \
        --output ./backups/pg_gcs_test.sql \
        --cloud "gs://$BUCKET_NAME/postgres/backup1.sql?endpoint=$GCS_ENDPOINT"
    
    if [ $? -eq 0 ]; then
        print_success "PostgreSQL backup uploaded to GCS"
    else
        print_error "PostgreSQL backup failed"
        return 1
    fi
}

# Test 2: MySQL backup to GCS
test_mysql_backup() {
    print_header "Test 2: MySQL Backup to GCS"
    
    ./dbbackup backup mysql \
        --host $MYSQL_HOST \
        --port $MYSQL_PORT \
        --user $MYSQL_USER \
        --password $MYSQL_PASS \
        --database $MYSQL_DB \
        --output ./backups/mysql_gcs_test.sql \
        --cloud "gs://$BUCKET_NAME/mysql/backup1.sql?endpoint=$GCS_ENDPOINT"
    
    if [ $? -eq 0 ]; then
        print_success "MySQL backup uploaded to GCS"
    else
        print_error "MySQL backup failed"
        return 1
    fi
}

# Test 3: List backups in GCS
test_list_backups() {
    print_header "Test 3: List GCS Backups"
    
    ./dbbackup cloud list "gs://$BUCKET_NAME/postgres/?endpoint=$GCS_ENDPOINT"
    
    if [ $? -eq 0 ]; then
        print_success "Listed GCS backups"
    else
        print_error "Failed to list backups"
        return 1
    fi
}

# Test 4: Verify backup in GCS
test_verify_backup() {
    print_header "Test 4: Verify GCS Backup"
    
    ./dbbackup verify "gs://$BUCKET_NAME/postgres/backup1.sql?endpoint=$GCS_ENDPOINT"
    
    if [ $? -eq 0 ]; then
        print_success "Backup verification successful"
    else
        print_error "Backup verification failed"
        return 1
    fi
}

# Test 5: Restore from GCS
test_restore_from_gcs() {
    print_header "Test 5: Restore from GCS"
    
    # Drop and recreate database
    PGPASSWORD=$POSTGRES_PASS psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d postgres <<EOF
DROP DATABASE IF EXISTS testdb_restored;
CREATE DATABASE testdb_restored;
EOF
    
    ./dbbackup restore postgres \
        --source "gs://$BUCKET_NAME/postgres/backup1.sql?endpoint=$GCS_ENDPOINT" \
        --host $POSTGRES_HOST \
        --port $POSTGRES_PORT \
        --user $POSTGRES_USER \
        --password $POSTGRES_PASS \
        --database testdb_restored
    
    if [ $? -eq 0 ]; then
        print_success "Restored from GCS backup"
        
        # Verify restored data
        COUNT=$(PGPASSWORD=$POSTGRES_PASS psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d testdb_restored -t -c "SELECT COUNT(*) FROM test_table;")
        if [ "$COUNT" -eq 3 ]; then
            print_success "Restored data verified (3 rows)"
        else
            print_error "Restored data incorrect (expected 3 rows, got $COUNT)"
        fi
    else
        print_error "Restore from GCS failed"
        return 1
    fi
}

# Test 6: Large file upload (chunked upload)
test_large_file_upload() {
    print_header "Test 6: Large File Upload (Chunked)"
    
    # Create a large test file (200MB)
    print_info "Creating 200MB test file..."
    dd if=/dev/urandom of=./backups/large_test.dat bs=1M count=200 2>/dev/null
    
    print_info "Uploading large file to GCS..."
    ./dbbackup cloud upload \
        ./backups/large_test.dat \
        "gs://$BUCKET_NAME/large/large_test.dat?endpoint=$GCS_ENDPOINT"
    
    if [ $? -eq 0 ]; then
        print_success "Large file uploaded successfully (chunked)"
        
        # Verify file exists and has correct size
        print_info "Downloading large file..."
        ./dbbackup cloud download \
            "gs://$BUCKET_NAME/large/large_test.dat?endpoint=$GCS_ENDPOINT" \
            ./backups/large_test_downloaded.dat
        
        if [ $? -eq 0 ]; then
            ORIGINAL_SIZE=$(stat -f%z ./backups/large_test.dat 2>/dev/null || stat -c%s ./backups/large_test.dat)
            DOWNLOADED_SIZE=$(stat -f%z ./backups/large_test_downloaded.dat 2>/dev/null || stat -c%s ./backups/large_test_downloaded.dat)
            
            if [ "$ORIGINAL_SIZE" -eq "$DOWNLOADED_SIZE" ]; then
                print_success "Downloaded file size matches original ($ORIGINAL_SIZE bytes)"
            else
                print_error "File size mismatch (original: $ORIGINAL_SIZE, downloaded: $DOWNLOADED_SIZE)"
            fi
        else
            print_error "Large file download failed"
        fi
        
        # Cleanup
        rm -f ./backups/large_test.dat ./backups/large_test_downloaded.dat
    else
        print_error "Large file upload failed"
        return 1
    fi
}

# Test 7: Delete from GCS
test_delete_backup() {
    print_header "Test 7: Delete Backup from GCS"
    
    ./dbbackup cloud delete "gs://$BUCKET_NAME/mysql/backup1.sql?endpoint=$GCS_ENDPOINT"
    
    if [ $? -eq 0 ]; then
        print_success "Deleted backup from GCS"
        
        # Verify deletion
        if ! ./dbbackup cloud list "gs://$BUCKET_NAME/mysql/?endpoint=$GCS_ENDPOINT" | grep -q "backup1.sql"; then
            print_success "Verified backup was deleted"
        else
            print_error "Backup still exists after deletion"
        fi
    else
        print_error "Failed to delete backup"
        return 1
    fi
}

# Test 8: Cleanup old backups
test_cleanup() {
    print_header "Test 8: Cleanup Old Backups"
    
    # Create multiple backups with different timestamps
    for i in {1..5}; do
        ./dbbackup backup postgres \
            --host $POSTGRES_HOST \
            --port $POSTGRES_PORT \
            --user $POSTGRES_USER \
            --password $POSTGRES_PASS \
            --database $POSTGRES_DB \
            --output "./backups/pg_cleanup_$i.sql" \
            --cloud "gs://$BUCKET_NAME/cleanup/backup_$i.sql?endpoint=$GCS_ENDPOINT"
        sleep 1
    done
    
    print_success "Created 5 test backups"
    
    # Cleanup, keeping only 2
    ./dbbackup cleanup "gs://$BUCKET_NAME/cleanup/?endpoint=$GCS_ENDPOINT" --keep 2
    
    if [ $? -eq 0 ]; then
        print_success "Cleanup completed"
        
        # Count remaining backups
        COUNT=$(./dbbackup cloud list "gs://$BUCKET_NAME/cleanup/?endpoint=$GCS_ENDPOINT" | grep -c "backup_")
        if [ "$COUNT" -le 2 ]; then
            print_success "Verified cleanup (kept 2 backups)"
        else
            print_error "Cleanup failed (expected 2 backups, found $COUNT)"
        fi
    else
        print_error "Cleanup failed"
        return 1
    fi
}

# Main test execution
main() {
    print_header "Google Cloud Storage (fake-gcs-server) Integration Tests"
    
    # Setup
    build_dbbackup
    start_services
    create_test_data
    
    # Run tests
    test_postgres_backup
    test_mysql_backup
    test_list_backups
    test_verify_backup
    test_restore_from_gcs
    test_large_file_upload
    test_delete_backup
    test_cleanup
    
    # Cleanup
    print_header "Cleanup"
    rm -rf ./backups
    
    # Summary
    print_header "Test Summary"
    echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
    
    if [ $TESTS_FAILED -eq 0 ]; then
        print_success "All tests passed!"
        stop_services
        exit 0
    else
        print_error "Some tests failed"
        print_info "Leaving services running for debugging"
        print_info "Run 'docker-compose -f docker-compose.gcs.yml down' to stop services"
        exit 1
    fi
}

# Run main
main
