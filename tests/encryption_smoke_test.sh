#!/bin/bash
# Quick smoke test for encryption feature

set -e

echo "==================================="
echo "Encryption Feature Smoke Test"
echo "==================================="
echo

# Setup
TEST_DIR="/tmp/dbbackup_encrypt_test_$$"
mkdir -p "$TEST_DIR"
cd "$TEST_DIR"

# Generate test key
echo "Step 1: Generate test key..."
echo "test-encryption-key-32bytes!!" > key.txt
KEY_BASE64=$(base64 -w 0 < key.txt)
export DBBACKUP_ENCRYPTION_KEY="$KEY_BASE64"

# Create test backup file
echo "Step 2: Create test backup..."
echo "This is test backup data for encryption testing." > test_backup.dump
echo "It contains multiple lines to ensure proper encryption." >> test_backup.dump
echo "$(date)" >> test_backup.dump

# Create metadata
cat > test_backup.dump.meta.json <<EOF
{
  "version": "2.3.0",
  "timestamp": "$(date -Iseconds)",
  "database": "testdb",
  "database_type": "postgresql",
  "backup_file": "$TEST_DIR/test_backup.dump",
  "size_bytes": $(stat -c%s test_backup.dump),
  "sha256": "test",
  "compression": "none",
  "backup_type": "full",
  "encrypted": false
}
EOF

echo "Original backup size: $(stat -c%s test_backup.dump) bytes"
echo "Original content hash: $(md5sum test_backup.dump | cut -d' ' -f1)"
echo

# Test encryption via Go code
echo "Step 3: Test encryption..."
cat > encrypt_test.go <<'GOCODE'
package main

import (
	"fmt"
	"os"
	
	"dbbackup/internal/backup"
	"dbbackup/internal/crypto"
	"dbbackup/internal/logger"
)

func main() {
	log := logger.New("info", "text")
	
	// Load key from env
	keyB64 := os.Getenv("DBBACKUP_ENCRYPTION_KEY")
	if keyB64 == "" {
		fmt.Println("ERROR: DBBACKUP_ENCRYPTION_KEY not set")
		os.Exit(1)
	}
	
	// Derive key
	salt, _ := crypto.GenerateSalt()
	key := crypto.DeriveKey([]byte(keyB64), salt)
	
	// Encrypt
	if err := backup.EncryptBackupFile("test_backup.dump", key, log); err != nil {
		fmt.Printf("ERROR: Encryption failed: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("✅ Encryption successful")
	
	// Decrypt
	if err := backup.DecryptBackupFile("test_backup.dump", "test_backup_decrypted.dump", key, log); err != nil {
		fmt.Printf("ERROR: Decryption failed: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("✅ Decryption successful")
}
GOCODE

# Temporarily copy go.mod
cp /root/dbbackup/go.mod .
cp /root/dbbackup/go.sum . 2>/dev/null || true

# Run encryption test
echo "Running Go encryption test..."
go run encrypt_test.go
echo

# Verify decrypted content
echo "Step 4: Verify decrypted content..."
if diff -q test_backup_decrypted.dump <(echo "This is test backup data for encryption testing."; echo "It contains multiple lines to ensure proper encryption.") >/dev/null 2>&1; then
	echo "✅ Content verification: PASS (decrypted matches original - first 2 lines)"
else
	echo "❌ Content verification: FAIL"
	echo "Expected first 2 lines to match"
	exit 1
fi

echo
echo "==================================="
echo "✅ All encryption tests PASSED"
echo "==================================="

# Cleanup
cd /
rm -rf "$TEST_DIR"
