package retention

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"dbbackup/internal/metadata"
)

// Policy defines the retention rules
type Policy struct {
	RetentionDays int
	MinBackups    int
	DryRun        bool
}

// CleanupResult contains information about cleanup operations
type CleanupResult struct {
	TotalBackups    int
	EligibleForDeletion int
	Deleted         []string
	Kept            []string
	SpaceFreed      int64
	Errors          []error
}

// ApplyPolicy enforces the retention policy on backups in a directory
func ApplyPolicy(backupDir string, policy Policy) (*CleanupResult, error) {
	result := &CleanupResult{
		Deleted: make([]string, 0),
		Kept:    make([]string, 0),
		Errors:  make([]error, 0),
	}

	// List all backups in directory
	backups, err := metadata.ListBackups(backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	result.TotalBackups = len(backups)

	// Sort backups by timestamp (oldest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.Before(backups[j].Timestamp)
	})

	// Calculate cutoff date
	cutoffDate := time.Now().AddDate(0, 0, -policy.RetentionDays)

	// Determine which backups to delete
	for i, backup := range backups {
		// Always keep minimum number of backups (most recent ones)
		backupsRemaining := len(backups) - i
		if backupsRemaining <= policy.MinBackups {
			result.Kept = append(result.Kept, backup.BackupFile)
			continue
		}

		// Check if backup is older than retention period
		if backup.Timestamp.Before(cutoffDate) {
			result.EligibleForDeletion++
			
			if policy.DryRun {
				result.Deleted = append(result.Deleted, backup.BackupFile)
			} else {
				// Delete backup file and associated metadata
				if err := deleteBackup(backup.BackupFile); err != nil {
					result.Errors = append(result.Errors, 
						fmt.Errorf("failed to delete %s: %w", backup.BackupFile, err))
				} else {
					result.Deleted = append(result.Deleted, backup.BackupFile)
					result.SpaceFreed += backup.SizeBytes
				}
			}
		} else {
			result.Kept = append(result.Kept, backup.BackupFile)
		}
	}

	return result, nil
}

// deleteBackup removes a backup file and all associated files
func deleteBackup(backupFile string) error {
	// Delete main backup file
	if err := os.Remove(backupFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete backup file: %w", err)
	}

	// Delete metadata file
	metaFile := backupFile + ".meta.json"
	if err := os.Remove(metaFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete metadata file: %w", err)
	}

	// Delete legacy .sha256 file if exists
	sha256File := backupFile + ".sha256"
	if err := os.Remove(sha256File); err != nil && !os.IsNotExist(err) {
		// Don't fail if .sha256 doesn't exist (new format)
	}

	// Delete legacy .info file if exists
	infoFile := backupFile + ".info"
	if err := os.Remove(infoFile); err != nil && !os.IsNotExist(err) {
		// Don't fail if .info doesn't exist (new format)
	}

	return nil
}

// GetOldestBackups returns the N oldest backups in a directory
func GetOldestBackups(backupDir string, count int) ([]*metadata.BackupMetadata, error) {
	backups, err := metadata.ListBackups(backupDir)
	if err != nil {
		return nil, err
	}

	// Sort by timestamp (oldest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.Before(backups[j].Timestamp)
	})

	if count > len(backups) {
		count = len(backups)
	}

	return backups[:count], nil
}

// GetNewestBackups returns the N newest backups in a directory
func GetNewestBackups(backupDir string, count int) ([]*metadata.BackupMetadata, error) {
	backups, err := metadata.ListBackups(backupDir)
	if err != nil {
		return nil, err
	}

	// Sort by timestamp (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	if count > len(backups) {
		count = len(backups)
	}

	return backups[:count], nil
}

// CleanupByPattern removes backups matching a specific pattern
func CleanupByPattern(backupDir, pattern string, policy Policy) (*CleanupResult, error) {
	result := &CleanupResult{
		Deleted: make([]string, 0),
		Kept:    make([]string, 0),
		Errors:  make([]error, 0),
	}

	// Find matching backup files
	searchPattern := filepath.Join(backupDir, pattern)
	matches, err := filepath.Glob(searchPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to match pattern: %w", err)
	}

	// Filter to only .dump or .sql files
	var backupFiles []string
	for _, match := range matches {
		ext := filepath.Ext(match)
		if ext == ".dump" || ext == ".sql" {
			backupFiles = append(backupFiles, match)
		}
	}

	// Load metadata for matched backups
	var backups []*metadata.BackupMetadata
	for _, file := range backupFiles {
		meta, err := metadata.Load(file)
		if err != nil {
			// Skip files without metadata
			continue
		}
		backups = append(backups, meta)
	}

	result.TotalBackups = len(backups)

	// Sort by timestamp
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.Before(backups[j].Timestamp)
	})

	cutoffDate := time.Now().AddDate(0, 0, -policy.RetentionDays)

	// Apply policy
	for i, backup := range backups {
		backupsRemaining := len(backups) - i
		if backupsRemaining <= policy.MinBackups {
			result.Kept = append(result.Kept, backup.BackupFile)
			continue
		}

		if backup.Timestamp.Before(cutoffDate) {
			result.EligibleForDeletion++
			
			if policy.DryRun {
				result.Deleted = append(result.Deleted, backup.BackupFile)
			} else {
				if err := deleteBackup(backup.BackupFile); err != nil {
					result.Errors = append(result.Errors, err)
				} else {
					result.Deleted = append(result.Deleted, backup.BackupFile)
					result.SpaceFreed += backup.SizeBytes
				}
			}
		} else {
			result.Kept = append(result.Kept, backup.BackupFile)
		}
	}

	return result, nil
}
