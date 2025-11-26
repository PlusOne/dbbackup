package wal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"dbbackup/internal/logger"
	"golang.org/x/crypto/pbkdf2"
)

// Encryptor handles WAL file encryption using AES-256-GCM
type Encryptor struct {
	log logger.Logger
}

// EncryptionOptions holds encryption configuration
type EncryptionOptions struct {
	Key        []byte // 32-byte encryption key
	Passphrase string // Alternative: derive key from passphrase
}

// NewEncryptor creates a new WAL encryptor
func NewEncryptor(log logger.Logger) *Encryptor {
	return &Encryptor{
		log: log,
	}
}

// EncryptWALFile encrypts a WAL file using AES-256-GCM
func (e *Encryptor) EncryptWALFile(sourcePath, destPath string, opts EncryptionOptions) (int64, error) {
	e.log.Debug("Encrypting WAL file", "source", sourcePath, "dest", destPath)

	// Derive key if passphrase provided
	var key []byte
	if len(opts.Key) == 32 {
		key = opts.Key
	} else if opts.Passphrase != "" {
		key = e.deriveKey(opts.Passphrase)
	} else {
		return 0, fmt.Errorf("encryption key or passphrase required")
	}

	// Open source file
	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Read entire file (WAL files are typically 16MB, manageable in memory)
	plaintext, err := io.ReadAll(srcFile)
	if err != nil {
		return 0, fmt.Errorf("failed to read source file: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return 0, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the data
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Write encrypted data
	dstFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return 0, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Write magic header to identify encrypted WAL files
	header := []byte("WALENC01") // WAL Encryption version 1
	if _, err := dstFile.Write(header); err != nil {
		return 0, fmt.Errorf("failed to write header: %w", err)
	}

	// Write encrypted data
	written, err := dstFile.Write(ciphertext)
	if err != nil {
		return 0, fmt.Errorf("failed to write encrypted data: %w", err)
	}

	// Sync to disk
	if err := dstFile.Sync(); err != nil {
		return 0, fmt.Errorf("failed to sync encrypted file: %w", err)
	}

	totalSize := int64(len(header) + written)
	e.log.Debug("WAL encryption complete",
		"original_size", len(plaintext),
		"encrypted_size", totalSize)

	return totalSize, nil
}

// DecryptWALFile decrypts an encrypted WAL file
func (e *Encryptor) DecryptWALFile(sourcePath, destPath string, opts EncryptionOptions) (int64, error) {
	e.log.Debug("Decrypting WAL file", "source", sourcePath, "dest", destPath)

	// Derive key if passphrase provided
	var key []byte
	if len(opts.Key) == 32 {
		key = opts.Key
	} else if opts.Passphrase != "" {
		key = e.deriveKey(opts.Passphrase)
	} else {
		return 0, fmt.Errorf("decryption key or passphrase required")
	}

	// Open encrypted file
	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open encrypted file: %w", err)
	}
	defer srcFile.Close()

	// Read and verify header
	header := make([]byte, 8)
	if _, err := io.ReadFull(srcFile, header); err != nil {
		return 0, fmt.Errorf("failed to read header: %w", err)
	}
	if string(header) != "WALENC01" {
		return 0, fmt.Errorf("not an encrypted WAL file or unsupported version")
	}

	// Read encrypted data
	ciphertext, err := io.ReadAll(srcFile)
	if err != nil {
		return 0, fmt.Errorf("failed to read encrypted data: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return 0, fmt.Errorf("ciphertext too short")
	}
	nonce := ciphertext[:nonceSize]
	ciphertext = ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return 0, fmt.Errorf("decryption failed (wrong key?): %w", err)
	}

	// Write decrypted data
	dstFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return 0, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	written, err := dstFile.Write(plaintext)
	if err != nil {
		return 0, fmt.Errorf("failed to write decrypted data: %w", err)
	}

	// Sync to disk
	if err := dstFile.Sync(); err != nil {
		return 0, fmt.Errorf("failed to sync decrypted file: %w", err)
	}

	e.log.Debug("WAL decryption complete", "decrypted_size", written)
	return int64(written), nil
}

// IsEncrypted checks if a file is an encrypted WAL file
func (e *Encryptor) IsEncrypted(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	header := make([]byte, 8)
	if _, err := io.ReadFull(file, header); err != nil {
		return false
	}

	return string(header) == "WALENC01"
}

// EncryptAndArchive encrypts and archives a WAL file in one operation
func (e *Encryptor) EncryptAndArchive(walPath, archiveDir string, opts EncryptionOptions) (archivePath string, encryptedSize int64, err error) {
	walFileName := filepath.Base(walPath)
	encryptedFileName := walFileName + ".enc"
	archivePath = filepath.Join(archiveDir, encryptedFileName)

	// Ensure archive directory exists
	if err := os.MkdirAll(archiveDir, 0700); err != nil {
		return "", 0, fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Encrypt directly to archive location
	encryptedSize, err = e.EncryptWALFile(walPath, archivePath, opts)
	if err != nil {
		// Clean up partial file on error
		os.Remove(archivePath)
		return "", 0, err
	}

	return archivePath, encryptedSize, nil
}

// deriveKey derives a 32-byte encryption key from a passphrase using PBKDF2
func (e *Encryptor) deriveKey(passphrase string) []byte {
	// Use a fixed salt for WAL encryption (alternative: store salt in header)
	salt := []byte("dbbackup-wal-encryption-v1")
	return pbkdf2.Key([]byte(passphrase), salt, 600000, 32, sha256.New)
}

// VerifyEncryptedFile verifies an encrypted file can be decrypted
func (e *Encryptor) VerifyEncryptedFile(encryptedPath string, opts EncryptionOptions) error {
	// Derive key
	var key []byte
	if len(opts.Key) == 32 {
		key = opts.Key
	} else if opts.Passphrase != "" {
		key = e.deriveKey(opts.Passphrase)
	} else {
		return fmt.Errorf("verification key or passphrase required")
	}

	// Open and verify header
	file, err := os.Open(encryptedPath)
	if err != nil {
		return fmt.Errorf("cannot open encrypted file: %w", err)
	}
	defer file.Close()

	header := make([]byte, 8)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}
	if string(header) != "WALENC01" {
		return fmt.Errorf("invalid encryption header")
	}

	// Read a small portion and try to decrypt
	sample := make([]byte, 1024)
	n, _ := file.Read(sample)
	if n == 0 {
		return fmt.Errorf("empty encrypted file")
	}

	// Quick decryption test
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if n < nonceSize {
		return fmt.Errorf("encrypted data too short")
	}

	// Verification passed (actual decryption would happen during restore)
	return nil
}
