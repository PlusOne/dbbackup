package encryption

import (
	"bytes"
	"io"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	// Test data
	original := []byte("This is a secret database backup that needs encryption! 🔒")
	
	// Test with passphrase
	t.Run("Passphrase", func(t *testing.T) {
		var encrypted bytes.Buffer
		
		// Encrypt
		writer, err := NewEncryptionWriter(&encrypted, EncryptionOptions{
			Passphrase: "super-secret-password",
		})
		if err != nil {
			t.Fatalf("Failed to create encryption writer: %v", err)
		}
		
		if _, err := writer.Write(original); err != nil {
			t.Fatalf("Failed to write data: %v", err)
		}
		
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}
		
		t.Logf("Original size: %d bytes", len(original))
		t.Logf("Encrypted size: %d bytes", encrypted.Len())
		
		// Verify encrypted data is different from original
		if bytes.Contains(encrypted.Bytes(), original) {
			t.Error("Encrypted data contains plaintext - encryption failed!")
		}
		
		// Decrypt
		reader, err := NewDecryptionReader(&encrypted, EncryptionOptions{
			Passphrase: "super-secret-password",
		})
		if err != nil {
			t.Fatalf("Failed to create decryption reader: %v", err)
		}
		
		decrypted, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("Failed to read decrypted data: %v", err)
		}
		
		// Verify decrypted matches original
		if !bytes.Equal(decrypted, original) {
			t.Errorf("Decrypted data doesn't match original\nOriginal: %s\nDecrypted: %s",
				string(original), string(decrypted))
		}
		
		t.Log("✅ Encryption/decryption successful")
	})
	
	// Test with direct key
	t.Run("DirectKey", func(t *testing.T) {
		key, err := GenerateKey()
		if err != nil {
			t.Fatalf("Failed to generate key: %v", err)
		}
		
		var encrypted bytes.Buffer
		
		// Encrypt
		writer, err := NewEncryptionWriter(&encrypted, EncryptionOptions{
			Key: key,
		})
		if err != nil {
			t.Fatalf("Failed to create encryption writer: %v", err)
		}
		
		if _, err := writer.Write(original); err != nil {
			t.Fatalf("Failed to write data: %v", err)
		}
		
		if err := writer.Close(); err != nil {
			t.Fatalf("Failed to close writer: %v", err)
		}
		
		// Decrypt
		reader, err := NewDecryptionReader(&encrypted, EncryptionOptions{
			Key: key,
		})
		if err != nil {
			t.Fatalf("Failed to create decryption reader: %v", err)
		}
		
		decrypted, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("Failed to read decrypted data: %v", err)
		}
		
		if !bytes.Equal(decrypted, original) {
			t.Errorf("Decrypted data doesn't match original")
		}
		
		t.Log("✅ Direct key encryption/decryption successful")
	})
	
	// Test wrong password
	t.Run("WrongPassword", func(t *testing.T) {
		var encrypted bytes.Buffer
		
		// Encrypt
		writer, err := NewEncryptionWriter(&encrypted, EncryptionOptions{
			Passphrase: "correct-password",
		})
		if err != nil {
			t.Fatalf("Failed to create encryption writer: %v", err)
		}
		
		writer.Write(original)
		writer.Close()
		
		// Try to decrypt with wrong password
		reader, err := NewDecryptionReader(&encrypted, EncryptionOptions{
			Passphrase: "wrong-password",
		})
		if err != nil {
			t.Fatalf("Failed to create decryption reader: %v", err)
		}
		
		_, err = io.ReadAll(reader)
		if err == nil {
			t.Error("Expected decryption to fail with wrong password, but it succeeded")
		}
		
		t.Logf("✅ Wrong password correctly rejected: %v", err)
	})
}

func TestLargeData(t *testing.T) {
	// Test with large data (1MB) to test chunking
	original := make([]byte, 1024*1024)
	for i := range original {
		original[i] = byte(i % 256)
	}
	
	var encrypted bytes.Buffer
	
	// Encrypt
	writer, err := NewEncryptionWriter(&encrypted, EncryptionOptions{
		Passphrase: "test-password",
	})
	if err != nil {
		t.Fatalf("Failed to create encryption writer: %v", err)
	}
	
	if _, err := writer.Write(original); err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}
	
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
	
	t.Logf("Original size: %d bytes", len(original))
	t.Logf("Encrypted size: %d bytes", encrypted.Len())
	t.Logf("Overhead: %.2f%%", float64(encrypted.Len()-len(original))/float64(len(original))*100)
	
	// Decrypt
	reader, err := NewDecryptionReader(&encrypted, EncryptionOptions{
		Passphrase: "test-password",
	})
	if err != nil {
		t.Fatalf("Failed to create decryption reader: %v", err)
	}
	
	decrypted, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read decrypted data: %v", err)
	}
	
	if !bytes.Equal(decrypted, original) {
		t.Errorf("Large data decryption failed")
	}
	
	t.Log("✅ Large data encryption/decryption successful")
}

func TestKeyGeneration(t *testing.T) {
	// Test key generation
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	
	if len(key1) != KeySize {
		t.Errorf("Key size mismatch: expected %d, got %d", KeySize, len(key1))
	}
	
	// Generate another key and verify it's different
	key2, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate second key: %v", err)
	}
	
	if bytes.Equal(key1, key2) {
		t.Error("Generated keys are identical - randomness broken!")
	}
	
	t.Log("✅ Key generation successful")
}

func TestKeyDerivation(t *testing.T) {
	passphrase := "my-secret-passphrase"
	salt1, _ := GenerateSalt()
	
	// Derive key twice with same salt - should be identical
	key1 := DeriveKey(passphrase, salt1)
	key2 := DeriveKey(passphrase, salt1)
	
	if !bytes.Equal(key1, key2) {
		t.Error("Key derivation not deterministic")
	}
	
	// Derive with different salt - should be different
	salt2, _ := GenerateSalt()
	key3 := DeriveKey(passphrase, salt2)
	
	if bytes.Equal(key1, key3) {
		t.Error("Different salts produced same key")
	}
	
	t.Log("✅ Key derivation successful")
}
