package verification

import (
	"fmt"
	"os"

	"dbbackup/internal/metadata"
)

// Result represents the outcome of a verification operation
type Result struct {
	Valid           bool
	BackupFile      string
	ExpectedSHA256  string
	CalculatedSHA256 string
	SizeMatch       bool
	FileExists      bool
	MetadataExists  bool
	Error           error
}

// Verify checks the integrity of a backup file
func Verify(backupFile string) (*Result, error) {
	result := &Result{
		BackupFile: backupFile,
	}

	// Check if backup file exists
	info, err := os.Stat(backupFile)
	if err != nil {
		result.FileExists = false
		result.Error = fmt.Errorf("backup file does not exist: %w", err)
		return result, nil
	}
	result.FileExists = true

	// Load metadata
	meta, err := metadata.Load(backupFile)
	if err != nil {
		result.MetadataExists = false
		result.Error = fmt.Errorf("failed to load metadata: %w", err)
		return result, nil
	}
	result.MetadataExists = true
	result.ExpectedSHA256 = meta.SHA256

	// Check size match
	if info.Size() != meta.SizeBytes {
		result.SizeMatch = false
		result.Error = fmt.Errorf("size mismatch: expected %d bytes, got %d bytes", 
			meta.SizeBytes, info.Size())
		return result, nil
	}
	result.SizeMatch = true

	// Calculate actual SHA-256
	actualSHA256, err := metadata.CalculateSHA256(backupFile)
	if err != nil {
		result.Error = fmt.Errorf("failed to calculate checksum: %w", err)
		return result, nil
	}
	result.CalculatedSHA256 = actualSHA256

	// Compare checksums
	if actualSHA256 != meta.SHA256 {
		result.Valid = false
		result.Error = fmt.Errorf("checksum mismatch: expected %s, got %s", 
			meta.SHA256, actualSHA256)
		return result, nil
	}

	// All checks passed
	result.Valid = true
	return result, nil
}

// VerifyMultiple verifies multiple backup files
func VerifyMultiple(backupFiles []string) ([]*Result, error) {
	var results []*Result
	
	for _, file := range backupFiles {
		result, err := Verify(file)
		if err != nil {
			return nil, fmt.Errorf("verification error for %s: %w", file, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// QuickCheck performs a fast check without full checksum calculation
// Only validates metadata existence and file size
func QuickCheck(backupFile string) error {
	// Check file exists
	info, err := os.Stat(backupFile)
	if err != nil {
		return fmt.Errorf("backup file does not exist: %w", err)
	}

	// Load metadata
	meta, err := metadata.Load(backupFile)
	if err != nil {
		return fmt.Errorf("metadata missing or invalid: %w", err)
	}

	// Check size
	if info.Size() != meta.SizeBytes {
		return fmt.Errorf("size mismatch: expected %d bytes, got %d bytes", 
			meta.SizeBytes, info.Size())
	}

	return nil
}
