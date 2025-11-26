package crypto

import (
	"bytes"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestAESEncryptionDecryption(t *testing.T) {
	encryptor := NewAESEncryptor()

	// Generate a random key
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	testData := []byte("This is test data for encryption and decryption. It contains multiple bytes to ensure proper streaming.")

	// Test streaming encryption/decryption
	t.Run("StreamingEncryptDecrypt", func(t *testing.T) {
		// Encrypt
		reader := bytes.NewReader(testData)
		encReader, err := encryptor.Encrypt(reader, key)
		if err != nil {
			t.Fatalf("Encryption failed: %v", err)
		}

		// Read all encrypted data
		encryptedData, err := io.ReadAll(encReader)
		if err != nil {
			t.Fatalf("Failed to read encrypted data: %v", err)
		}

		// Verify encrypted data is different from original
		if bytes.Equal(encryptedData, testData) {
			t.Error("Encrypted data should not equal plaintext")
		}

		// Decrypt
		decReader, err := encryptor.Decrypt(bytes.NewReader(encryptedData), key)
		if err != nil {
			t.Fatalf("Decryption failed: %v", err)
		}

		// Read decrypted data
		decryptedData, err := io.ReadAll(decReader)
		if err != nil {
			t.Fatalf("Failed to read decrypted data: %v", err)
		}

		// Verify decrypted data matches original
		if !bytes.Equal(decryptedData, testData) {
			t.Errorf("Decrypted data does not match original.\nExpected: %s\nGot: %s",
				string(testData), string(decryptedData))
		}
	})

	// Test file encryption/decryption
	t.Run("FileEncryptDecrypt", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "crypto_test_*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create test file
		testFile := filepath.Join(tempDir, "test.txt")
		if err := os.WriteFile(testFile, testData, 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Encrypt file
		encryptedFile := filepath.Join(tempDir, "test.txt.enc")
		if err := encryptor.EncryptFile(testFile, encryptedFile, key); err != nil {
			t.Fatalf("File encryption failed: %v", err)
		}

		// Verify encrypted file exists and is different
		encData, err := os.ReadFile(encryptedFile)
		if err != nil {
			t.Fatalf("Failed to read encrypted file: %v", err)
		}
		if bytes.Equal(encData, testData) {
			t.Error("Encrypted file should not equal plaintext")
		}

		// Decrypt file
		decryptedFile := filepath.Join(tempDir, "test.txt.dec")
		if err := encryptor.DecryptFile(encryptedFile, decryptedFile, key); err != nil {
			t.Fatalf("File decryption failed: %v", err)
		}

		// Verify decrypted file matches original
		decData, err := os.ReadFile(decryptedFile)
		if err != nil {
			t.Fatalf("Failed to read decrypted file: %v", err)
		}
		if !bytes.Equal(decData, testData) {
			t.Errorf("Decrypted file does not match original")
		}
	})

	// Test wrong key
	t.Run("WrongKey", func(t *testing.T) {
		wrongKey := make([]byte, KeySize)
		if _, err := io.ReadFull(rand.Reader, wrongKey); err != nil {
			t.Fatalf("Failed to generate wrong key: %v", err)
		}

		// Encrypt with correct key
		reader := bytes.NewReader(testData)
		encReader, err := encryptor.Encrypt(reader, key)
		if err != nil {
			t.Fatalf("Encryption failed: %v", err)
		}

		encryptedData, err := io.ReadAll(encReader)
		if err != nil {
			t.Fatalf("Failed to read encrypted data: %v", err)
		}

		// Try to decrypt with wrong key
		decReader, err := encryptor.Decrypt(bytes.NewReader(encryptedData), wrongKey)
		if err != nil {
			// Error during decrypt setup is OK
			return
		}

		// Try to read - should fail
		_, err = io.ReadAll(decReader)
		if err == nil {
			t.Error("Expected decryption to fail with wrong key")
		}
	})
}

func TestKeyDerivation(t *testing.T) {
	password := []byte("test-password-12345")

	// Generate salt
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("Failed to generate salt: %v", err)
	}

	if len(salt) != SaltSize {
		t.Errorf("Expected salt size %d, got %d", SaltSize, len(salt))
	}

	// Derive key
	key := DeriveKey(password, salt)
	if len(key) != KeySize {
		t.Errorf("Expected key size %d, got %d", KeySize, len(key))
	}

	// Verify same password+salt produces same key
	key2 := DeriveKey(password, salt)
	if !bytes.Equal(key, key2) {
		t.Error("Same password and salt should produce same key")
	}

	// Verify different salt produces different key
	salt2, _ := GenerateSalt()
	key3 := DeriveKey(password, salt2)
	if bytes.Equal(key, key3) {
		t.Error("Different salt should produce different key")
	}
}

func TestKeyValidation(t *testing.T) {
	validKey := make([]byte, KeySize)
	if err := ValidateKey(validKey); err != nil {
		t.Errorf("Valid key should pass validation: %v", err)
	}

	shortKey := make([]byte, 16)
	if err := ValidateKey(shortKey); err == nil {
		t.Error("Short key should fail validation")
	}

	longKey := make([]byte, 64)
	if err := ValidateKey(longKey); err == nil {
		t.Error("Long key should fail validation")
	}
}

func TestLargeData(t *testing.T) {
	encryptor := NewAESEncryptor()

	// Generate key
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Create large test data (1MB)
	largeData := make([]byte, 1024*1024)
	if _, err := io.ReadFull(rand.Reader, largeData); err != nil {
		t.Fatalf("Failed to generate large data: %v", err)
	}

	// Encrypt
	encReader, err := encryptor.Encrypt(bytes.NewReader(largeData), key)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	encryptedData, err := io.ReadAll(encReader)
	if err != nil {
		t.Fatalf("Failed to read encrypted data: %v", err)
	}

	// Decrypt
	decReader, err := encryptor.Decrypt(bytes.NewReader(encryptedData), key)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	decryptedData, err := io.ReadAll(decReader)
	if err != nil {
		t.Fatalf("Failed to read decrypted data: %v", err)
	}

	// Verify
	if !bytes.Equal(decryptedData, largeData) {
		t.Error("Decrypted large data does not match original")
	}
}
