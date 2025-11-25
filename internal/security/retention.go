package security

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"dbbackup/internal/logger"
)

// RetentionPolicy defines backup retention rules
type RetentionPolicy struct {
	RetentionDays int
	MinBackups    int // Minimum backups to keep regardless of age
	log           logger.Logger
}

// NewRetentionPolicy creates a new retention policy
func NewRetentionPolicy(retentionDays, minBackups int, log logger.Logger) *RetentionPolicy {
	return &RetentionPolicy{
		RetentionDays: retentionDays,
		MinBackups:    minBackups,
		log:           log,
	}
}

// ArchiveInfo holds information about a backup archive
type ArchiveInfo struct {
	Path     string
	ModTime  time.Time
	Size     int64
	Database string
}

// CleanupOldBackups removes backups older than retention period
func (rp *RetentionPolicy) CleanupOldBackups(backupDir string) (int, int64, error) {
	if rp.RetentionDays <= 0 {
		return 0, 0, nil // Retention disabled
	}

	archives, err := rp.scanBackupArchives(backupDir)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to scan backup directory: %w", err)
	}

	if len(archives) <= rp.MinBackups {
		rp.log.Debug("Keeping all backups (below minimum threshold)", 
			"count", len(archives), "min_backups", rp.MinBackups)
		return 0, 0, nil
	}

	cutoffTime := time.Now().AddDate(0, 0, -rp.RetentionDays)
	
	// Sort by modification time (oldest first)
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].ModTime.Before(archives[j].ModTime)
	})

	var deletedCount int
	var freedSpace int64

	for i, archive := range archives {
		// Keep minimum number of backups
		remaining := len(archives) - i
		if remaining <= rp.MinBackups {
			rp.log.Debug("Stopped cleanup to maintain minimum backups", 
				"remaining", remaining, "min_backups", rp.MinBackups)
			break
		}

		// Delete if older than retention period
		if archive.ModTime.Before(cutoffTime) {
			rp.log.Info("Removing old backup", 
				"file", filepath.Base(archive.Path),
				"age_days", int(time.Since(archive.ModTime).Hours()/24),
				"size_mb", archive.Size/1024/1024)

			if err := os.Remove(archive.Path); err != nil {
				rp.log.Warn("Failed to remove old backup", "file", archive.Path, "error", err)
				continue
			}

			// Also remove checksum file if exists
			checksumPath := archive.Path + ".sha256"
			if _, err := os.Stat(checksumPath); err == nil {
				os.Remove(checksumPath)
			}

			// Also remove metadata file if exists
			metadataPath := archive.Path + ".meta"
			if _, err := os.Stat(metadataPath); err == nil {
				os.Remove(metadataPath)
			}

			deletedCount++
			freedSpace += archive.Size
		}
	}

	if deletedCount > 0 {
		rp.log.Info("Cleanup completed", 
			"deleted_backups", deletedCount,
			"freed_space_mb", freedSpace/1024/1024,
			"retention_days", rp.RetentionDays)
	}

	return deletedCount, freedSpace, nil
}

// scanBackupArchives scans directory for backup archives
func (rp *RetentionPolicy) scanBackupArchives(backupDir string) ([]ArchiveInfo, error) {
	var archives []ArchiveInfo

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		
		// Skip non-backup files
		if !isBackupArchive(name) {
			continue
		}

		path := filepath.Join(backupDir, name)
		info, err := entry.Info()
		if err != nil {
			rp.log.Warn("Failed to get file info", "file", name, "error", err)
			continue
		}

		archives = append(archives, ArchiveInfo{
			Path:     path,
			ModTime:  info.ModTime(),
			Size:     info.Size(),
			Database: extractDatabaseName(name),
		})
	}

	return archives, nil
}

// isBackupArchive checks if filename is a backup archive
func isBackupArchive(name string) bool {
	return (filepath.Ext(name) == ".dump" ||
		filepath.Ext(name) == ".sql" ||
		filepath.Ext(name) == ".gz" ||
		filepath.Ext(name) == ".tar") &&
		name != ".sha256" &&
		name != ".meta"
}

// extractDatabaseName extracts database name from archive filename
func extractDatabaseName(filename string) string {
	base := filepath.Base(filename)
	
	// Remove extensions
	for {
		oldBase := base
		base = removeExtension(base)
		if base == oldBase {
			break
		}
	}
	
	// Remove timestamp patterns
	if len(base) > 20 {
		// Typically: db_name_20240101_120000
		underscoreCount := 0
		for i := len(base) - 1; i >= 0; i-- {
			if base[i] == '_' {
				underscoreCount++
				if underscoreCount >= 2 {
					return base[:i]
				}
			}
		}
	}
	
	return base
}

// removeExtension removes one extension from filename
func removeExtension(name string) string {
	if ext := filepath.Ext(name); ext != "" {
		return name[:len(name)-len(ext)]
	}
	return name
}
