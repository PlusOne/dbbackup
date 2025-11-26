package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"dbbackup/internal/crypto"
)

// loadEncryptionKey loads encryption key from file or environment variable
func loadEncryptionKey(keyFile, keyEnvVar string) ([]byte, error) {
	// Priority 1: Key file
	if keyFile != "" {
		keyData, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read encryption key file: %w", err)
		}
		
		// Try to decode as base64 first
		if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(keyData))); err == nil && len(decoded) == crypto.KeySize {
			return decoded, nil
		}
		
		// Use raw bytes if exactly 32 bytes
		if len(keyData) == crypto.KeySize {
			return keyData, nil
		}
		
		// Otherwise treat as passphrase and derive key
		salt, err := crypto.GenerateSalt()
		if err != nil {
			return nil, fmt.Errorf("failed to generate salt: %w", err)
		}
		key := crypto.DeriveKey([]byte(strings.TrimSpace(string(keyData))), salt)
		return key, nil
	}
	
	// Priority 2: Environment variable
	if keyEnvVar != "" {
		keyData := os.Getenv(keyEnvVar)
		if keyData == "" {
			return nil, fmt.Errorf("encryption enabled but %s environment variable not set", keyEnvVar)
		}
		
		// Try to decode as base64 first
		if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(keyData)); err == nil && len(decoded) == crypto.KeySize {
			return decoded, nil
		}
		
		// Otherwise treat as passphrase and derive key
		salt, err := crypto.GenerateSalt()
		if err != nil {
			return nil, fmt.Errorf("failed to generate salt: %w", err)
		}
		key := crypto.DeriveKey([]byte(strings.TrimSpace(keyData)), salt)
		return key, nil
	}
	
	return nil, fmt.Errorf("encryption enabled but no key source specified (use --encryption-key-file or set %s)", keyEnvVar)
}

// isEncryptionEnabled checks if encryption is requested
func isEncryptionEnabled() bool {
	return encryptBackupFlag
}

// generateEncryptionKey generates a new random encryption key
func generateEncryptionKey() ([]byte, error) {
	salt, err := crypto.GenerateSalt()
	if err != nil {
		return nil, err
	}
	// For key generation, use salt as both password and salt (random)
	return crypto.DeriveKey(salt, salt), nil
}
