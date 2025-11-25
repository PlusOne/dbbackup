#!/bin/bash
set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}dbbackup Cloud Storage Integration Test${NC}"
echo -e "${BLUE}========================================${NC}"
echo

# Configuration
MINIO_ENDPOINT="http://localhost:9000"
MINIO_ACCESS_KEY="minioadmin"
MINIO_SECRET_KEY="minioadmin123"
MINIO_BUCKET="test-backups"
POSTGRES_HOST="localhost"
POSTGRES_PORT="5433"
POSTGRES_USER="testuser"
POSTGRES_PASS="testpass123"
POSTGRES_DB="cloudtest"

# Export credentials
export AWS_ACCESS_KEY_ID="$MINIO_ACCESS_KEY"
export AWS_SECRET_ACCESS_KEY="$MINIO_SECRET_KEY"
export AWS_ENDPOINT_URL="$MINIO_ENDPOINT"
export AWS_REGION="us-east-1"

# Check if dbbackup binary exists
if [ ! -f "./dbbackup" ]; then
    echo -e "${YELLOW}Building dbbackup...${NC}"
    go build -o dbbackup .
    echo -e "${GREEN}✓ Build successful${NC}"
fi

# Function to wait for service
wait_for_service() {
    local service=$1
    local host=$2
    local port=$3
    local max_attempts=30
    local attempt=1
    
    echo -e "${YELLOW}Waiting for $service to be ready...${NC}"
    
    while ! nc -z $host $port 2>/dev/null; do
        if [ $attempt -ge $max_attempts ]; then
            echo -e "${RED}✗ $service did not start in time${NC}"
            return 1
        fi
        echo -n "."
        sleep 1
        attempt=$((attempt + 1))
    done
    
    echo -e "${GREEN}✓ $service is ready${NC}"
}

# Step 1: Start services
echo -e "${BLUE}Step 1: Starting services with Docker Compose${NC}"
docker-compose -f docker-compose.minio.yml up -d

# Wait for services
wait_for_service "MinIO" "localhost" "9000"
wait_for_service "PostgreSQL" "localhost" "5433"

sleep 5

# Step 2: Create test database
echo -e "\n${BLUE}Step 2: Creating test database${NC}"
PGPASSWORD=$POSTGRES_PASS psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -c "DROP DATABASE IF EXISTS $POSTGRES_DB;" postgres 2>/dev/null || true
PGPASSWORD=$POSTGRES_PASS psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -c "CREATE DATABASE $POSTGRES_DB;" postgres
PGPASSWORD=$POSTGRES_PASS psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d $POSTGRES_DB << EOF
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users (name, email) VALUES
    ('Alice', 'alice@example.com'),
    ('Bob', 'bob@example.com'),
    ('Charlie', 'charlie@example.com');

CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200),
    price DECIMAL(10,2)
);

INSERT INTO products (name, price) VALUES
    ('Widget', 19.99),
    ('Gadget', 29.99),
    ('Doohickey', 39.99);
EOF

echo -e "${GREEN}✓ Test database created with sample data${NC}"

# Step 3: Test local backup
echo -e "\n${BLUE}Step 3: Creating local backup${NC}"
./dbbackup backup single $POSTGRES_DB \
    --db-type postgres \
    --host $POSTGRES_HOST \
    --port $POSTGRES_PORT \
    --user $POSTGRES_USER \
    --password $POSTGRES_PASS \
    --output-dir /tmp/dbbackup-test

LOCAL_BACKUP=$(ls -t /tmp/dbbackup-test/${POSTGRES_DB}_*.dump 2>/dev/null | head -1)
if [ -z "$LOCAL_BACKUP" ]; then
    echo -e "${RED}✗ Local backup failed${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Local backup created: $LOCAL_BACKUP${NC}"

# Step 4: Test cloud upload
echo -e "\n${BLUE}Step 4: Uploading backup to MinIO (S3)${NC}"
./dbbackup cloud upload "$LOCAL_BACKUP" \
    --cloud-provider minio \
    --cloud-bucket $MINIO_BUCKET \
    --cloud-endpoint $MINIO_ENDPOINT

echo -e "${GREEN}✓ Upload successful${NC}"

# Step 5: Test cloud list
echo -e "\n${BLUE}Step 5: Listing cloud backups${NC}"
./dbbackup cloud list \
    --cloud-provider minio \
    --cloud-bucket $MINIO_BUCKET \
    --cloud-endpoint $MINIO_ENDPOINT \
    --verbose

# Step 6: Test backup with cloud URI
echo -e "\n${BLUE}Step 6: Testing backup with cloud URI${NC}"
./dbbackup backup single $POSTGRES_DB \
    --db-type postgres \
    --host $POSTGRES_HOST \
    --port $POSTGRES_PORT \
    --user $POSTGRES_USER \
    --password $POSTGRES_PASS \
    --output-dir /tmp/dbbackup-test \
    --cloud minio://$MINIO_BUCKET/uri-test/

echo -e "${GREEN}✓ Backup with cloud URI successful${NC}"

# Step 7: Drop database for restore test
echo -e "\n${BLUE}Step 7: Dropping database for restore test${NC}"
PGPASSWORD=$POSTGRES_PASS psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -c "DROP DATABASE $POSTGRES_DB;" postgres

# Step 8: Test restore from cloud URI
echo -e "\n${BLUE}Step 8: Restoring from cloud URI${NC}"
CLOUD_URI="minio://$MINIO_BUCKET/$(basename $LOCAL_BACKUP)"
./dbbackup restore single "$CLOUD_URI" \
    --target $POSTGRES_DB \
    --create \
    --confirm \
    --db-type postgres \
    --host $POSTGRES_HOST \
    --port $POSTGRES_PORT \
    --user $POSTGRES_USER \
    --password $POSTGRES_PASS

echo -e "${GREEN}✓ Restore from cloud successful${NC}"

# Step 9: Verify data
echo -e "\n${BLUE}Step 9: Verifying restored data${NC}"
USER_COUNT=$(PGPASSWORD=$POSTGRES_PASS psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d $POSTGRES_DB -t -c "SELECT COUNT(*) FROM users;")
PRODUCT_COUNT=$(PGPASSWORD=$POSTGRES_PASS psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d $POSTGRES_DB -t -c "SELECT COUNT(*) FROM products;")

if [ "$USER_COUNT" -eq 3 ] && [ "$PRODUCT_COUNT" -eq 3 ]; then
    echo -e "${GREEN}✓ Data verification successful (users: $USER_COUNT, products: $PRODUCT_COUNT)${NC}"
else
    echo -e "${RED}✗ Data verification failed (users: $USER_COUNT, products: $PRODUCT_COUNT)${NC}"
    exit 1
fi

# Step 10: Test verify command
echo -e "\n${BLUE}Step 10: Verifying cloud backup integrity${NC}"
./dbbackup verify-backup "$CLOUD_URI"

echo -e "${GREEN}✓ Backup verification successful${NC}"

# Step 11: Test cloud cleanup
echo -e "\n${BLUE}Step 11: Testing cloud cleanup (dry-run)${NC}"
./dbbackup cleanup "minio://$MINIO_BUCKET/" \
    --retention-days 0 \
    --min-backups 1 \
    --dry-run

# Step 12: Create multiple backups for cleanup test
echo -e "\n${BLUE}Step 12: Creating multiple backups for cleanup test${NC}"
for i in {1..5}; do
    echo "Creating backup $i/5..."
    ./dbbackup backup single $POSTGRES_DB \
        --db-type postgres \
        --host $POSTGRES_HOST \
        --port $POSTGRES_PORT \
        --user $POSTGRES_USER \
        --password $POSTGRES_PASS \
        --output-dir /tmp/dbbackup-test \
        --cloud minio://$MINIO_BUCKET/cleanup-test/
    sleep 1
done

echo -e "${GREEN}✓ Multiple backups created${NC}"

# Step 13: Test actual cleanup
echo -e "\n${BLUE}Step 13: Testing cloud cleanup (actual)${NC}"
./dbbackup cleanup "minio://$MINIO_BUCKET/cleanup-test/" \
    --retention-days 0 \
    --min-backups 2

echo -e "${GREEN}✓ Cloud cleanup successful${NC}"

# Step 14: Test large file upload (multipart)
echo -e "\n${BLUE}Step 14: Testing large file upload (>100MB for multipart)${NC}"
echo "Creating 150MB test file..."
dd if=/dev/zero of=/tmp/large-test-file.bin bs=1M count=150 2>/dev/null

echo "Uploading large file..."
./dbbackup cloud upload /tmp/large-test-file.bin \
    --cloud-provider minio \
    --cloud-bucket $MINIO_BUCKET \
    --cloud-endpoint $MINIO_ENDPOINT \
    --verbose

echo -e "${GREEN}✓ Large file multipart upload successful${NC}"

# Cleanup
echo -e "\n${BLUE}Cleanup${NC}"
rm -f /tmp/large-test-file.bin
rm -rf /tmp/dbbackup-test

echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}✓ ALL TESTS PASSED!${NC}"
echo -e "${GREEN}========================================${NC}"
echo
echo -e "${YELLOW}To stop services:${NC}"
echo -e "  docker-compose -f docker-compose.minio.yml down"
echo
echo -e "${YELLOW}To view MinIO console:${NC}"
echo -e "  http://localhost:9001 (minioadmin / minioadmin123)"
echo
echo -e "${YELLOW}To keep services running for manual testing:${NC}"
echo -e "  export AWS_ACCESS_KEY_ID=minioadmin"
echo -e "  export AWS_SECRET_ACCESS_KEY=minioadmin123"
echo -e "  export AWS_ENDPOINT_URL=http://localhost:9000"
echo -e "  ./dbbackup cloud list --cloud-provider minio --cloud-bucket test-backups"
