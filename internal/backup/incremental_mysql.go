package backup

import (
	"archive/tar"
	"compress/gzip"
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

// MySQLIncrementalEngine implements incremental backups for MySQL/MariaDB
type MySQLIncrementalEngine struct {
	log logger.Logger
}

// NewMySQLIncrementalEngine creates a new MySQL incremental backup engine
func NewMySQLIncrementalEngine(log logger.Logger) *MySQLIncrementalEngine {
	return &MySQLIncrementalEngine{
		log: log,
	}
}

// FindChangedFiles identifies files that changed since the base backup
// Uses mtime-based detection. Production could integrate with MySQL binary logs for more precision.
func (e *MySQLIncrementalEngine) FindChangedFiles(ctx context.Context, config *IncrementalBackupConfig) ([]ChangedFile, error) {
	e.log.Info("Finding changed files for incremental backup (MySQL)",
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

		// Skip temporary files, relay logs, and other MySQL-specific files
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

// shouldSkipFile determines if a file should be excluded from incremental backup (MySQL-specific)
func (e *MySQLIncrementalEngine) shouldSkipFile(path string, info os.FileInfo) bool {
	name := info.Name()
	lowerPath := strings.ToLower(path)

	// Skip temporary files
	if strings.HasSuffix(name, ".tmp") || strings.HasPrefix(name, "#sql") {
		return true
	}

	// Skip MySQL lock files
	if strings.HasSuffix(name, ".lock") || name == "auto.cnf.lock" {
		return true
	}

	// Skip MySQL pid file
	if strings.HasSuffix(name, ".pid") || name == "mysqld.pid" {
		return true
	}

	// Skip sockets
	if info.Mode()&os.ModeSocket != 0 || strings.HasSuffix(name, ".sock") {
		return true
	}

	// Skip MySQL relay logs (replication)
	if strings.Contains(lowerPath, "relay-log") || strings.Contains(name, "relay-bin") {
		return true
	}

	// Skip MySQL binary logs (handled separately if needed)
	// Note: For production incremental backups, binary logs should be backed up separately
	if strings.Contains(name, "mysql-bin") || strings.Contains(name, "binlog") {
		return true
	}

	// Skip InnoDB redo logs (ib_logfile*)
	if strings.HasPrefix(name, "ib_logfile") {
		return true
	}

	// Skip InnoDB undo logs (undo_*)
	if strings.HasPrefix(name, "undo_") {
		return true
	}

	// Skip MySQL error logs
	if strings.HasSuffix(name, ".err") || name == "error.log" {
		return true
	}

	// Skip MySQL slow query logs
	if strings.Contains(name, "slow") && strings.HasSuffix(name, ".log") {
		return true
	}

	// Skip general query logs
	if name == "general.log" || name == "query.log" {
		return true
	}

	// Skip performance schema (in-memory only)
	if strings.Contains(lowerPath, "performance_schema") {
		return true
	}

	// Skip MySQL Cluster temporary files
	if strings.HasPrefix(name, "ndb_") {
		return true
	}

	return false
}

// loadBackupInfo loads backup metadata from .meta.json file
func (e *MySQLIncrementalEngine) loadBackupInfo(backupPath string) (*metadata.BackupMetadata, error) {
	// Load using metadata package
	meta, err := metadata.Load(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load backup metadata: %w", err)
	}

	return meta, nil
}

// CreateIncrementalBackup creates a new incremental backup archive for MySQL
func (e *MySQLIncrementalEngine) CreateIncrementalBackup(ctx context.Context, config *IncrementalBackupConfig, changedFiles []ChangedFile) error {
	e.log.Info("Creating incremental backup (MySQL)",
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
		Version:         "2.3.0",
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

	e.log.Info("Incremental backup created successfully (MySQL)",
		"output", outputFile,
		"size", stat.Size(),
		"changed_files", len(changedFiles),
		"checksum", checksum[:16]+"...")

	return nil
}

// RestoreIncremental restores a MySQL incremental backup on top of a base
func (e *MySQLIncrementalEngine) RestoreIncremental(ctx context.Context, baseBackupPath, incrementalPath, targetDir string) error {
	e.log.Info("Restoring incremental backup (MySQL)",
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
	e.log.Info("Extracting base backup (MySQL)", "output", targetDir)
	if err := e.extractTarGz(ctx, baseBackupPath, targetDir); err != nil {
		return fmt.Errorf("failed to extract base backup: %w", err)
	}
	e.log.Info("Base backup extracted successfully")

	// Step 2: Extract incremental backup, overwriting changed files
	e.log.Info("Applying incremental backup (MySQL)", "changed_files", incrInfo.Incremental.IncrementalFiles)
	if err := e.extractTarGz(ctx, incrementalPath, targetDir); err != nil {
		return fmt.Errorf("failed to extract incremental backup: %w", err)
	}
	e.log.Info("Incremental backup applied successfully")

	// Step 3: Verify restoration
	e.log.Info("Restore complete (MySQL)",
		"base_backup", filepath.Base(baseBackupPath),
		"incremental_backup", filepath.Base(incrementalPath),
		"target_directory", targetDir,
		"total_files_updated", incrInfo.Incremental.IncrementalFiles)

	return nil
}

// CalculateFileChecksum computes SHA-256 hash of a file
func (e *MySQLIncrementalEngine) CalculateFileChecksum(path string) (string, error) {
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

// createTarGz creates a tar.gz archive with the specified changed files
func (e *MySQLIncrementalEngine) createTarGz(ctx context.Context, outputFile string, changedFiles []ChangedFile, config *IncrementalBackupConfig) error {
	// Import needed for tar/gzip
	outFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Create gzip writer
	gzWriter, err := gzip.NewWriterLevel(outFile, config.CompressionLevel)
	if err != nil {
		return fmt.Errorf("failed to create gzip writer: %w", err)
	}
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Add each changed file to archive
	for i, changedFile := range changedFiles {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		e.log.Debug("Adding file to archive (MySQL)",
			"file", changedFile.RelativePath,
			"progress", fmt.Sprintf("%d/%d", i+1, len(changedFiles)))

		if err := e.addFileToTar(tarWriter, changedFile); err != nil {
			return fmt.Errorf("failed to add file %s: %w", changedFile.RelativePath, err)
		}
	}

	return nil
}

// addFileToTar adds a single file to the tar archive
func (e *MySQLIncrementalEngine) addFileToTar(tarWriter *tar.Writer, changedFile ChangedFile) error {
	// Open the file
	file, err := os.Open(changedFile.AbsolutePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Skip if file has been deleted/changed since scan
	if info.Size() != changedFile.Size {
		e.log.Warn("File size changed since scan, using current size",
			"file", changedFile.RelativePath,
			"old_size", changedFile.Size,
			"new_size", info.Size())
	}

	// Create tar header
	header := &tar.Header{
		Name:    changedFile.RelativePath,
		Size:    info.Size(),
		Mode:    int64(info.Mode()),
		ModTime: info.ModTime(),
	}

	// Write header
	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	// Copy file content
	if _, err := io.Copy(tarWriter, file); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

// extractTarGz extracts a tar.gz archive to the specified directory
// Files are extracted with their original permissions and timestamps
func (e *MySQLIncrementalEngine) extractTarGz(ctx context.Context, archivePath, targetDir string) error {
	// Open archive file
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer archiveFile.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract each file
	fileCount := 0
	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Build target path
		targetPath := filepath.Join(targetDir, header.Name)

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", header.Name, err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", header.Name, err)
			}

		case tar.TypeReg:
			// Extract regular file
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", header.Name, err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file %s: %w", header.Name, err)
			}
			outFile.Close()

			// Preserve modification time
			if err := os.Chtimes(targetPath, header.ModTime, header.ModTime); err != nil {
				e.log.Warn("Failed to set file modification time", "file", header.Name, "error", err)
			}

			fileCount++
			if fileCount%100 == 0 {
				e.log.Debug("Extraction progress (MySQL)", "files", fileCount)
			}

		case tar.TypeSymlink:
			// Create symlink
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				// Don't fail on symlink errors - just warn
				e.log.Warn("Failed to create symlink", "source", header.Name, "target", header.Linkname, "error", err)
			}

		default:
			e.log.Warn("Unsupported tar entry type", "type", header.Typeflag, "name", header.Name)
		}
	}

	e.log.Info("Archive extracted (MySQL)", "files", fileCount, "archive", filepath.Base(archivePath))
	return nil
}
