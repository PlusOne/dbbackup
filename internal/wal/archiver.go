package wal

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
)

// Archiver handles PostgreSQL Write-Ahead Log (WAL) archiving for PITR
type Archiver struct {
	cfg *config.Config
	log logger.Logger
}

// ArchiveConfig holds WAL archiving configuration
type ArchiveConfig struct {
	ArchiveDir      string // Directory to store archived WAL files
	CompressWAL     bool   // Compress WAL files with gzip
	EncryptWAL      bool   // Encrypt WAL files
	EncryptionKey   []byte // 32-byte key for AES-256-GCM encryption
	RetentionDays   int    // Days to keep WAL archives
	VerifyChecksum  bool   // Verify WAL file checksums
}

// WALArchiveInfo contains metadata about an archived WAL file
type WALArchiveInfo struct {
	WALFileName    string    `json:"wal_filename"`
	ArchivePath    string    `json:"archive_path"`
	OriginalSize   int64     `json:"original_size"`
	ArchivedSize   int64     `json:"archived_size"`
	Checksum       string    `json:"checksum"`
	Timeline       uint32    `json:"timeline"`
	Segment        uint64    `json:"segment"`
	ArchivedAt     time.Time `json:"archived_at"`
	Compressed     bool      `json:"compressed"`
	Encrypted      bool      `json:"encrypted"`
}

// NewArchiver creates a new WAL archiver
func NewArchiver(cfg *config.Config, log logger.Logger) *Archiver {
	return &Archiver{
		cfg: cfg,
		log: log,
	}
}

// ArchiveWALFile archives a single WAL file to the archive directory
// This is called by PostgreSQL's archive_command
func (a *Archiver) ArchiveWALFile(ctx context.Context, walFilePath, walFileName string, config ArchiveConfig) (*WALArchiveInfo, error) {
	a.log.Info("Archiving WAL file", "wal", walFileName, "source", walFilePath)

	// Validate WAL file exists
	stat, err := os.Stat(walFilePath)
	if err != nil {
		return nil, fmt.Errorf("WAL file not found: %s: %w", walFilePath, err)
	}

	// Ensure archive directory exists
	if err := os.MkdirAll(config.ArchiveDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create WAL archive directory %s: %w", config.ArchiveDir, err)
	}

	// Parse WAL filename to extract timeline and segment
	timeline, segment, err := ParseWALFileName(walFileName)
	if err != nil {
		a.log.Warn("Could not parse WAL filename (continuing anyway)", "file", walFileName, "error", err)
		timeline, segment = 0, 0 // Use defaults for non-standard names
	}

	// Process WAL file: compression and/or encryption
	var archivePath string
	var archivedSize int64
	
	if config.CompressWAL && config.EncryptWAL {
		// Compress then encrypt
		archivePath, archivedSize, err = a.compressAndEncryptWAL(walFilePath, walFileName, config)
	} else if config.CompressWAL {
		// Compress only
		archivePath, archivedSize, err = a.compressWAL(walFilePath, walFileName, config)
	} else if config.EncryptWAL {
		// Encrypt only
		archivePath, archivedSize, err = a.encryptWAL(walFilePath, walFileName, config)
	} else {
		// Plain copy
		archivePath, archivedSize, err = a.copyWAL(walFilePath, walFileName, config)
	}

	if err != nil {
		return nil, err
	}

	info := &WALArchiveInfo{
		WALFileName:  walFileName,
		ArchivePath:  archivePath,
		OriginalSize: stat.Size(),
		ArchivedSize: archivedSize,
		Timeline:     timeline,
		Segment:      segment,
		ArchivedAt:   time.Now(),
		Compressed:   config.CompressWAL,
		Encrypted:    config.EncryptWAL,
	}

	a.log.Info("WAL file archived successfully",
		"wal", walFileName,
		"archive", archivePath,
		"original_size", stat.Size(),
		"archived_size", archivedSize,
		"timeline", timeline,
		"segment", segment)

	return info, nil
}

// copyWAL performs a simple file copy
func (a *Archiver) copyWAL(walFilePath, walFileName string, config ArchiveConfig) (string, int64, error) {
	archivePath := filepath.Join(config.ArchiveDir, walFileName)

	srcFile, err := os.Open(walFilePath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to open WAL file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create archive file: %w", err)
	}
	defer dstFile.Close()

	written, err := io.Copy(dstFile, srcFile)
	if err != nil {
		return "", 0, fmt.Errorf("failed to copy WAL file: %w", err)
	}

	if err := dstFile.Sync(); err != nil {
		return "", 0, fmt.Errorf("failed to sync WAL archive: %w", err)
	}

	return archivePath, written, nil
}

// compressWAL compresses a WAL file using gzip
func (a *Archiver) compressWAL(walFilePath, walFileName string, config ArchiveConfig) (string, int64, error) {
	archivePath := filepath.Join(config.ArchiveDir, walFileName+".gz")
	
	compressor := NewCompressor(a.log)
	compressedSize, err := compressor.CompressWALFile(walFilePath, archivePath, 6) // gzip level 6 (balanced)
	if err != nil {
		return "", 0, fmt.Errorf("WAL compression failed: %w", err)
	}

	return archivePath, compressedSize, nil
}

// encryptWAL encrypts a WAL file
func (a *Archiver) encryptWAL(walFilePath, walFileName string, config ArchiveConfig) (string, int64, error) {
	archivePath := filepath.Join(config.ArchiveDir, walFileName+".enc")
	
	encryptor := NewEncryptor(a.log)
	encOpts := EncryptionOptions{
		Key: config.EncryptionKey,
	}
	
	encryptedSize, err := encryptor.EncryptWALFile(walFilePath, archivePath, encOpts)
	if err != nil {
		return "", 0, fmt.Errorf("WAL encryption failed: %w", err)
	}

	return archivePath, encryptedSize, nil
}

// compressAndEncryptWAL compresses then encrypts a WAL file
func (a *Archiver) compressAndEncryptWAL(walFilePath, walFileName string, config ArchiveConfig) (string, int64, error) {
	// Step 1: Compress to temp file
	tempDir := filepath.Join(config.ArchiveDir, ".tmp")
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return "", 0, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up temp dir

	tempCompressed := filepath.Join(tempDir, walFileName+".gz")
	compressor := NewCompressor(a.log)
	_, err := compressor.CompressWALFile(walFilePath, tempCompressed, 6)
	if err != nil {
		return "", 0, fmt.Errorf("WAL compression failed: %w", err)
	}

	// Step 2: Encrypt compressed file
	archivePath := filepath.Join(config.ArchiveDir, walFileName+".gz.enc")
	encryptor := NewEncryptor(a.log)
	encOpts := EncryptionOptions{
		Key: config.EncryptionKey,
	}
	
	encryptedSize, err := encryptor.EncryptWALFile(tempCompressed, archivePath, encOpts)
	if err != nil {
		return "", 0, fmt.Errorf("WAL encryption failed: %w", err)
	}

	return archivePath, encryptedSize, nil
}

// ParseWALFileName extracts timeline and segment number from WAL filename
// WAL filename format: 000000010000000000000001
// - First 8 hex digits: timeline ID
// - Next 8 hex digits: log file ID
// - Last 8 hex digits: segment number
func ParseWALFileName(filename string) (timeline uint32, segment uint64, err error) {
	// Remove any extensions (.gz, .enc, etc.)
	base := filepath.Base(filename)
	base = strings.TrimSuffix(base, ".gz")
	base = strings.TrimSuffix(base, ".enc")

	// WAL files are 24 hex characters
	if len(base) != 24 {
		return 0, 0, fmt.Errorf("invalid WAL filename length: expected 24 characters, got %d", len(base))
	}

	// Parse timeline (first 8 chars)
	_, err = fmt.Sscanf(base[0:8], "%08X", &timeline)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse timeline from WAL filename: %w", err)
	}

	// Parse segment (last 16 chars as combined log file + segment)
	_, err = fmt.Sscanf(base[8:24], "%016X", &segment)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse segment from WAL filename: %w", err)
	}

	return timeline, segment, nil
}

// ListArchivedWALFiles returns all WAL files in the archive directory
func (a *Archiver) ListArchivedWALFiles(config ArchiveConfig) ([]WALArchiveInfo, error) {
	entries, err := os.ReadDir(config.ArchiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []WALArchiveInfo{}, nil // Empty archive is valid
		}
		return nil, fmt.Errorf("failed to read WAL archive directory: %w", err)
	}

	var archives []WALArchiveInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		// Skip non-WAL files (must be 24 hex chars possibly with .gz/.enc extensions)
		baseName := strings.TrimSuffix(strings.TrimSuffix(filename, ".gz"), ".enc")
		if len(baseName) != 24 {
			continue
		}

		timeline, segment, err := ParseWALFileName(filename)
		if err != nil {
			a.log.Warn("Skipping invalid WAL file", "file", filename, "error", err)
			continue
		}

		info, err := entry.Info()
		if err != nil {
			a.log.Warn("Could not stat WAL file", "file", filename, "error", err)
			continue
		}

		archives = append(archives, WALArchiveInfo{
			WALFileName:  baseName,
			ArchivePath:  filepath.Join(config.ArchiveDir, filename),
			ArchivedSize: info.Size(),
			Timeline:     timeline,
			Segment:      segment,
			ArchivedAt:   info.ModTime(),
			Compressed:   strings.HasSuffix(filename, ".gz"),
			Encrypted:    strings.HasSuffix(filename, ".enc"),
		})
	}

	return archives, nil
}

// CleanupOldWALFiles removes WAL archives older than retention period
func (a *Archiver) CleanupOldWALFiles(ctx context.Context, config ArchiveConfig) (int, error) {
	if config.RetentionDays <= 0 {
		return 0, nil // No cleanup if retention not set
	}

	cutoffTime := time.Now().AddDate(0, 0, -config.RetentionDays)
	a.log.Info("Cleaning up WAL archives", "older_than", cutoffTime.Format("2006-01-02"), "retention_days", config.RetentionDays)

	archives, err := a.ListArchivedWALFiles(config)
	if err != nil {
		return 0, fmt.Errorf("failed to list WAL archives: %w", err)
	}

	deleted := 0
	for _, archive := range archives {
		if archive.ArchivedAt.Before(cutoffTime) {
			a.log.Debug("Removing old WAL archive", "file", archive.WALFileName, "archived_at", archive.ArchivedAt)
			if err := os.Remove(archive.ArchivePath); err != nil {
				a.log.Warn("Failed to remove old WAL archive", "file", archive.ArchivePath, "error", err)
				continue
			}
			deleted++
		}
	}

	a.log.Info("WAL cleanup completed", "deleted", deleted, "total_archives", len(archives))
	return deleted, nil
}

// GetArchiveStats returns statistics about WAL archives
func (a *Archiver) GetArchiveStats(config ArchiveConfig) (*ArchiveStats, error) {
	archives, err := a.ListArchivedWALFiles(config)
	if err != nil {
		return nil, err
	}

	stats := &ArchiveStats{
		TotalFiles:      len(archives),
		CompressedFiles: 0,
		EncryptedFiles:  0,
		TotalSize:       0,
	}

	if len(archives) > 0 {
		stats.OldestArchive = archives[0].ArchivedAt
		stats.NewestArchive = archives[0].ArchivedAt
	}

	for _, archive := range archives {
		stats.TotalSize += archive.ArchivedSize
		
		if archive.Compressed {
			stats.CompressedFiles++
		}
		if archive.Encrypted {
			stats.EncryptedFiles++
		}

		if archive.ArchivedAt.Before(stats.OldestArchive) {
			stats.OldestArchive = archive.ArchivedAt
		}
		if archive.ArchivedAt.After(stats.NewestArchive) {
			stats.NewestArchive = archive.ArchivedAt
		}
	}

	return stats, nil
}

// ArchiveStats contains statistics about WAL archives
type ArchiveStats struct {
	TotalFiles      int       `json:"total_files"`
	CompressedFiles int       `json:"compressed_files"`
	EncryptedFiles  int       `json:"encrypted_files"`
	TotalSize       int64     `json:"total_size"`
	OldestArchive   time.Time `json:"oldest_archive"`
	NewestArchive   time.Time `json:"newest_archive"`
}

// FormatSize returns human-readable size
func (s *ArchiveStats) FormatSize() string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	size := float64(s.TotalSize)
	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", size/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", size/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", size/KB)
	default:
		return fmt.Sprintf("%d B", s.TotalSize)
	}
}
