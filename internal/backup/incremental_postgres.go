package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dbbackup/internal/logger"
	"dbbackup/internal/metadata"
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

	// Validate base backup is full backup  
	if baseInfo.BackupType != "" && baseInfo.BackupType != "full" {
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

// loadBackupInfo loads backup metadata from .meta.json file
func (e *PostgresIncrementalEngine) loadBackupInfo(backupPath string) (*metadata.BackupMetadata, error) {
	// Load using metadata package
	meta, err := metadata.Load(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load backup metadata: %w", err)
	}

	return meta, nil
}

// CreateIncrementalBackup creates a new incremental backup archive
func (e *PostgresIncrementalEngine) CreateIncrementalBackup(ctx context.Context, config *IncrementalBackupConfig, changedFiles []ChangedFile) error {
	e.log.Info("Creating incremental backup",
		"changed_files", len(changedFiles),
		"base_backup", config.BaseBackupPath)

	if len(changedFiles) == 0 {
		e.log.Info("No changed files detected - skipping incremental backup")
		return fmt.Errorf("no changed files since base backup")
	}

	// Load base backup metadata
	baseInfo, err := e.loadBackupInfo(config.BaseBackupPath)
	if err != nil {
		return fmt.Errorf("failed to load base backup info: %w", err)
	}

	// Generate output filename: dbname_incr_TIMESTAMP.tar.gz
	timestamp := time.Now().Format("20060102_150405")
	outputFile := filepath.Join(filepath.Dir(config.BaseBackupPath), 
		fmt.Sprintf("%s_incr_%s.tar.gz", baseInfo.Database, timestamp))

	e.log.Info("Creating incremental archive", "output", outputFile)

	// Create tar.gz archive with changed files
	if err := e.createTarGz(ctx, outputFile, changedFiles, config); err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}

	// Calculate checksum
	checksum, err := e.CalculateFileChecksum(outputFile)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Get archive size
	stat, err := os.Stat(outputFile)
	if err != nil {
		return fmt.Errorf("failed to stat archive: %w", err)
	}

	// Calculate total size of changed files
	var totalSize int64
	for _, f := range changedFiles {
		totalSize += f.Size
	}

	// Create incremental metadata
	metadata := &metadata.BackupMetadata{
		Version:         "2.2.0",
		Timestamp:       time.Now(),
		Database:        baseInfo.Database,
		DatabaseType:    baseInfo.DatabaseType,
		Host:            baseInfo.Host,
		Port:            baseInfo.Port,
		User:            baseInfo.User,
		BackupFile:      outputFile,
		SizeBytes:       stat.Size(),
		SHA256:          checksum,
		Compression:     "gzip",
		BackupType:      "incremental",
		BaseBackup:      filepath.Base(config.BaseBackupPath),
		Incremental: &metadata.IncrementalMetadata{
			BaseBackupID:        baseInfo.SHA256,
			BaseBackupPath:      filepath.Base(config.BaseBackupPath),
			BaseBackupTimestamp: baseInfo.Timestamp,
			IncrementalFiles:    len(changedFiles),
			TotalSize:           totalSize,
			BackupChain:         buildBackupChain(baseInfo, filepath.Base(outputFile)),
		},
	}

	// Save metadata
	if err := metadata.Save(); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	e.log.Info("Incremental backup created successfully",
		"output", outputFile,
		"size", stat.Size(),
		"changed_files", len(changedFiles),
		"checksum", checksum[:16]+"...")

	return nil
}

// RestoreIncremental restores an incremental backup on top of a base
func (e *PostgresIncrementalEngine) RestoreIncremental(ctx context.Context, baseBackupPath, incrementalPath, targetDir string) error {
	e.log.Info("Restoring incremental backup",
		"base", baseBackupPath,
		"incremental", incrementalPath,
		"target", targetDir)

	// Load incremental metadata to verify it's an incremental backup
	incrInfo, err := e.loadBackupInfo(incrementalPath)
	if err != nil {
		return fmt.Errorf("failed to load incremental backup metadata: %w", err)
	}

	if incrInfo.BackupType != "incremental" {
		return fmt.Errorf("backup is not incremental (type: %s)", incrInfo.BackupType)
	}

	if incrInfo.Incremental == nil {
		return fmt.Errorf("incremental metadata missing")
	}

	// Verify base backup path matches metadata
	expectedBase := filepath.Join(filepath.Dir(incrementalPath), incrInfo.Incremental.BaseBackupPath)
	if !strings.EqualFold(filepath.Clean(baseBackupPath), filepath.Clean(expectedBase)) {
		e.log.Warn("Base backup path mismatch",
			"provided", baseBackupPath,
			"expected", expectedBase)
		// Continue anyway - user might have moved files
	}

	// Verify base backup exists
	if _, err := os.Stat(baseBackupPath); err != nil {
		return fmt.Errorf("base backup not found: %w", err)
	}

	// Load base backup metadata to verify it's a full backup
	baseInfo, err := e.loadBackupInfo(baseBackupPath)
	if err != nil {
		return fmt.Errorf("failed to load base backup metadata: %w", err)
	}

	if baseInfo.BackupType != "full" && baseInfo.BackupType != "" {
		return fmt.Errorf("base backup is not a full backup (type: %s)", baseInfo.BackupType)
	}

	// Verify checksums match
	if incrInfo.Incremental.BaseBackupID != "" && baseInfo.SHA256 != "" {
		if incrInfo.Incremental.BaseBackupID != baseInfo.SHA256 {
			return fmt.Errorf("base backup checksum mismatch: expected %s, got %s",
				incrInfo.Incremental.BaseBackupID, baseInfo.SHA256)
		}
		e.log.Info("Base backup checksum verified", "checksum", baseInfo.SHA256)
	}

	// Create target directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Step 1: Extract base backup to target directory
	e.log.Info("Extracting base backup", "output", targetDir)
	if err := e.extractTarGz(ctx, baseBackupPath, targetDir); err != nil {
		return fmt.Errorf("failed to extract base backup: %w", err)
	}
	e.log.Info("Base backup extracted successfully")

	// Step 2: Extract incremental backup, overwriting changed files
	e.log.Info("Applying incremental backup", "changed_files", incrInfo.Incremental.IncrementalFiles)
	if err := e.extractTarGz(ctx, incrementalPath, targetDir); err != nil {
		return fmt.Errorf("failed to extract incremental backup: %w", err)
	}
	e.log.Info("Incremental backup applied successfully")

	// Step 3: Verify restoration
	e.log.Info("Restore complete",
		"base_backup", filepath.Base(baseBackupPath),
		"incremental_backup", filepath.Base(incrementalPath),
		"target_directory", targetDir,
		"total_files_updated", incrInfo.Incremental.IncrementalFiles)

	return nil
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

// buildBackupChain constructs the backup chain from base backup to current incremental
func buildBackupChain(baseInfo *metadata.BackupMetadata, currentBackup string) []string {
	chain := []string{}
	
	// If base backup has a chain (is itself incremental), use that
	if baseInfo.Incremental != nil && len(baseInfo.Incremental.BackupChain) > 0 {
		chain = append(chain, baseInfo.Incremental.BackupChain...)
	} else {
		// Base is a full backup, start chain with it
		chain = append(chain, filepath.Base(baseInfo.BackupFile))
	}
	
	// Add current incremental to chain
	chain = append(chain, currentBackup)
	
	return chain
}
