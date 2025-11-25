package security

import (
	"fmt"
	"path/filepath"
	"strings"
)

// CleanPath sanitizes a file path to prevent path traversal attacks
func CleanPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Clean the path (removes .., ., //)
	cleaned := filepath.Clean(path)

	// Detect path traversal attempts
	if strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("path traversal detected: %s", path)
	}

	return cleaned, nil
}

// ValidateBackupPath ensures backup path is safe
func ValidateBackupPath(path string) (string, error) {
	cleaned, err := CleanPath(path)
	if err != nil {
		return "", err
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	return absPath, nil
}

// ValidateArchivePath validates an archive file path
func ValidateArchivePath(path string) (string, error) {
	cleaned, err := CleanPath(path)
	if err != nil {
		return "", err
	}

	// Must have a valid archive extension
	ext := strings.ToLower(filepath.Ext(cleaned))
	validExtensions := []string{".dump", ".sql", ".gz", ".tar"}
	
	valid := false
	for _, validExt := range validExtensions {
		if strings.HasSuffix(cleaned, validExt) {
			valid = true
			break
		}
	}

	if !valid {
		return "", fmt.Errorf("invalid archive extension: %s (must be .dump, .sql, .gz, or .tar)", ext)
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	return absPath, nil
}
