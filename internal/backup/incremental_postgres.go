package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"dbbackup/internal/logger"
)

// PostgresIncrementalEngine implements incremental backups for PostgreSQL
type PostgresIncrementalEngine struct {
	log logger.Logger
}

// NewPostgresIncrementalEngine creates a new PostgreSQL incremental backup engine
func NewPostgresIncrementalEngine(log logger.Logger) *PostgresIncrementalEngine {
	return &PostgresIncrementalEngine{
		log: log,
	}
}

// FindChangedFiles identifies files that changed since the base backup
// This is a simple mtime-based implementation. Production should use pg_basebackup with incremental support.
func (e *PostgresIncrementalEngine) FindChangedFiles(ctx context.Context, config *IncrementalBackupConfig) ([]ChangedFile, error) {
	e.log.Info("Finding changed files for incremental backup",
		"base_backup", config.BaseBackupPath,
		"data_dir", config.DataDirectory)

	// Load base backup metadata to get timestamp
	baseInfo, err := e.loadBackupInfo(config.BaseBackupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load base backup info: %w", err)
	}

	if baseInfo.BackupType != BackupTypeFull {
		return nil, fmt.Errorf("base backup must be a full backup, got: %s", baseInfo.BackupType)
	}

	baseTimestamp := baseInfo.Timestamp
	e.log.Info("Base backup timestamp", "timestamp", baseTimestamp)

	// Scan data directory for changed files
	var changedFiles []ChangedFile
	
	err = filepath.Walk(config.DataDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip temporary files, lock files, and sockets
		if e.shouldSkipFile(path, info) {
			return nil
		}

		// Check if file was modified after base backup
		if info.ModTime().After(baseTimestamp) {
			relPath, err := filepath.Rel(config.DataDirectory, path)
			if err != nil {
				e.log.Warn("Failed to get relative path", "path", path, "error", err)
				return nil
			}

			changedFiles = append(changedFiles, ChangedFile{
				RelativePath: relPath,
				AbsolutePath: path,
				Size:         info.Size(),
				ModTime:      info.ModTime(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan data directory: %w", err)
	}

	e.log.Info("Found changed files", "count", len(changedFiles))
	return changedFiles, nil
}

// shouldSkipFile determines if a file should be excluded from incremental backup
func (e *PostgresIncrementalEngine) shouldSkipFile(path string, info os.FileInfo) bool {
	name := info.Name()

	// Skip temporary files
	if strings.HasSuffix(name, ".tmp") {
		return true
	}

	// Skip lock files
	if strings.HasSuffix(name, ".lock") || name == "postmaster.pid" {
		return true
	}

	// Skip sockets
	if info.Mode()&os.ModeSocket != 0 {
		return true
	}

	// Skip pg_wal symlink target (WAL handled separately if needed)
	if strings.Contains(path, "pg_wal") || strings.Contains(path, "pg_xlog") {
		return true
	}

	// Skip pg_replslot (replication slots)
	if strings.Contains(path, "pg_replslot") {
		return true
	}

	// Skip postmaster.opts (runtime config, regenerated on startup)
	if name == "postmaster.opts" {
		return true
	}

	return false
}

// loadBackupInfo loads backup metadata from .info file
func (e *PostgresIncrementalEngine) loadBackupInfo(backupPath string) (*BackupInfo, error) {
	// Remove .tar.gz extension and add .info
	infoPath := strings.TrimSuffix(backupPath, ".tar.gz") + ".info"
	
	data, err := os.ReadFile(infoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read info file %s: %w", infoPath, err)
	}

	var info BackupInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse info file: %w", err)
	}

	return &info, nil
}

// CreateIncrementalBackup creates a new incremental backup archive
func (e *PostgresIncrementalEngine) CreateIncrementalBackup(ctx context.Context, config *IncrementalBackupConfig, changedFiles []ChangedFile) error {
	e.log.Info("Creating incremental backup",
		"changed_files", len(changedFiles),
		"base_backup", config.BaseBackupPath)

	// TODO: Implementation in next step
	// 1. Create tar.gz with only changed files
	// 2. Generate metadata with base backup reference
	// 3. Write .info file with incremental metadata
	// 4. Calculate checksums

	return fmt.Errorf("not implemented yet")
}

// RestoreIncremental restores an incremental backup on top of a base
func (e *PostgresIncrementalEngine) RestoreIncremental(ctx context.Context, baseBackupPath, incrementalPath, targetDir string) error {
	e.log.Info("Restoring incremental backup",
		"base", baseBackupPath,
		"incremental", incrementalPath,
		"target", targetDir)

	// TODO: Implementation in next step
	// 1. Extract base backup to target
	// 2. Extract incremental backup, overwriting files
	// 3. Verify checksums
	// 4. Update permissions

	return fmt.Errorf("not implemented yet")
}

// CalculateFileChecksum computes SHA-256 hash of a file
func (e *PostgresIncrementalEngine) CalculateFileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
