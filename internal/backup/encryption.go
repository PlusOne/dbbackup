package backup

import (
	"fmt"
	"os"
	"path/filepath"

	"dbbackup/internal/crypto"
	"dbbackup/internal/logger"
	"dbbackup/internal/metadata"
)

// EncryptBackupFile encrypts a backup file in-place
// The original file is replaced with the encrypted version
func EncryptBackupFile(backupPath string, key []byte, log logger.Logger) error {
	log.Info("Encrypting backup file", "file", filepath.Base(backupPath))
	
	// Validate key
	if err := crypto.ValidateKey(key); err != nil {
		return fmt.Errorf("invalid encryption key: %w", err)
	}

	// Create encryptor
	encryptor := crypto.NewAESEncryptor()

	// Generate encrypted file path
	encryptedPath := backupPath + ".encrypted.tmp"

	// Encrypt file
	if err := encryptor.EncryptFile(backupPath, encryptedPath, key); err != nil {
		// Clean up temp file on failure
		os.Remove(encryptedPath)
		return fmt.Errorf("encryption failed: %w", err)
	}

	// Update metadata to indicate encryption
	metaPath := backupPath + ".meta.json"
	if _, err := os.Stat(metaPath); err == nil {
		// Load existing metadata
		meta, err := metadata.Load(metaPath)
		if err != nil {
			log.Warn("Failed to load metadata for encryption update", "error", err)
		} else {
			// Mark as encrypted
			meta.Encrypted = true
			meta.EncryptionAlgorithm = string(crypto.AlgorithmAES256GCM)

			// Save updated metadata
			if err := metadata.Save(metaPath, meta); err != nil {
				log.Warn("Failed to update metadata with encryption info", "error", err)
			}
		}
	}

	// Remove original unencrypted file
	if err := os.Remove(backupPath); err != nil {
		log.Warn("Failed to remove original unencrypted file", "error", err)
		// Don't fail - encrypted file exists
	}

	// Rename encrypted file to original name
	if err := os.Rename(encryptedPath, backupPath); err != nil {
		return fmt.Errorf("failed to rename encrypted file: %w", err)
	}

	log.Info("Backup encrypted successfully", "file", filepath.Base(backupPath))
	return nil
}

// IsBackupEncrypted checks if a backup file is encrypted
func IsBackupEncrypted(backupPath string) bool {
	// Check metadata first - try cluster metadata (for cluster backups)
	// Try cluster metadata first
	if clusterMeta, err := metadata.LoadCluster(backupPath); err == nil {
		// For cluster backups, check if ANY database is encrypted
		for _, db := range clusterMeta.Databases {
			if db.Encrypted {
				return true
			}
		}
		// All databases are unencrypted
		return false
	}
	
	// Try single database metadata
	if meta, err := metadata.Load(backupPath); err == nil {
		return meta.Encrypted
	}
	
	// Fallback: check if file starts with encryption nonce
	file, err := os.Open(backupPath)
	if err != nil {
		return false
	}
	defer file.Close()
	
	// Try to read nonce - if it succeeds, likely encrypted
	nonce := make([]byte, crypto.NonceSize)
	if n, err := file.Read(nonce); err != nil || n != crypto.NonceSize {
		return false
	}
	
	return true
}

// DecryptBackupFile decrypts an encrypted backup file
// Creates a new decrypted file
func DecryptBackupFile(encryptedPath, outputPath string, key []byte, log logger.Logger) error {
	log.Info("Decrypting backup file", "file", filepath.Base(encryptedPath))

	// Validate key
	if err := crypto.ValidateKey(key); err != nil {
		return fmt.Errorf("invalid decryption key: %w", err)
	}

	// Create encryptor
	encryptor := crypto.NewAESEncryptor()

	// Decrypt file
	if err := encryptor.DecryptFile(encryptedPath, outputPath, key); err != nil {
		return fmt.Errorf("decryption failed (wrong key?): %w", err)
	}

	log.Info("Backup decrypted successfully", "output", filepath.Base(outputPath))
	return nil
}
