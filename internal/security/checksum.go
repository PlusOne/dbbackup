package security

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// ChecksumFile calculates SHA-256 checksum of a file
func ChecksumFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// VerifyChecksum verifies a file's checksum against expected value
func VerifyChecksum(path string, expectedChecksum string) error {
	actualChecksum, err := ChecksumFile(path)
	if err != nil {
		return err
	}

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// SaveChecksum saves checksum to a .sha256 file alongside the archive
func SaveChecksum(archivePath string, checksum string) error {
	checksumPath := archivePath + ".sha256"
	content := fmt.Sprintf("%s  %s\n", checksum, archivePath)
	
	if err := os.WriteFile(checksumPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save checksum: %w", err)
	}

	return nil
}

// LoadChecksum loads checksum from a .sha256 file
func LoadChecksum(archivePath string) (string, error) {
	checksumPath := archivePath + ".sha256"
	
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return "", fmt.Errorf("failed to read checksum file: %w", err)
	}

	// Parse "checksum  filename" format
	parts := []byte{}
	for i, b := range data {
		if b == ' ' {
			parts = data[:i]
			break
		}
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("invalid checksum file format")
	}

	return string(parts), nil
}

// LoadAndVerifyChecksum loads checksum from .sha256 file and verifies the archive
// Returns nil if checksum file doesn't exist (optional verification)
// Returns error if checksum file exists but verification fails
func LoadAndVerifyChecksum(archivePath string) error {
	expectedChecksum, err := LoadChecksum(archivePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Checksum file doesn't exist, skip verification
		}
		return err
	}

	return VerifyChecksum(archivePath, expectedChecksum)
}
