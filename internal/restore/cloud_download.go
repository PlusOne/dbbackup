package restore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"dbbackup/internal/cloud"
	"dbbackup/internal/logger"
	"dbbackup/internal/metadata"
)

// CloudDownloader handles downloading backups from cloud storage
type CloudDownloader struct {
	backend cloud.Backend
	log     logger.Logger
}

// NewCloudDownloader creates a new cloud downloader
func NewCloudDownloader(backend cloud.Backend, log logger.Logger) *CloudDownloader {
	return &CloudDownloader{
		backend: backend,
		log:     log,
	}
}

// DownloadOptions contains options for downloading from cloud
type DownloadOptions struct {
	VerifyChecksum bool   // Verify SHA-256 checksum after download
	KeepLocal      bool   // Keep downloaded file (don't delete temp)
	TempDir        string // Temp directory (default: os.TempDir())
}

// DownloadResult contains information about a downloaded backup
type DownloadResult struct {
	LocalPath    string // Path to downloaded file
	RemotePath   string // Original remote path
	Size         int64  // File size in bytes
	SHA256       string // SHA-256 checksum (if verified)
	MetadataPath string // Path to downloaded metadata (if exists)
	IsTempFile   bool   // Whether the file is in a temp directory
}

// Download downloads a backup from cloud storage
func (d *CloudDownloader) Download(ctx context.Context, remotePath string, opts DownloadOptions) (*DownloadResult, error) {
	// Determine temp directory
	tempDir := opts.TempDir
	if tempDir == "" {
		tempDir = os.TempDir()
	}

	// Create unique temp subdirectory
	tempSubDir := filepath.Join(tempDir, fmt.Sprintf("dbbackup-download-%d", os.Getpid()))
	if err := os.MkdirAll(tempSubDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Extract filename from remote path
	filename := filepath.Base(remotePath)
	localPath := filepath.Join(tempSubDir, filename)

	d.log.Info("Downloading backup from cloud", "remote", remotePath, "local", localPath)

	// Get file size for progress tracking
	size, err := d.backend.GetSize(ctx, remotePath)
	if err != nil {
		d.log.Warn("Could not get remote file size", "error", err)
		size = 0 // Continue anyway
	}

	// Progress callback
	var lastPercent int
	progressCallback := func(transferred, total int64) {
		if total > 0 {
			percent := int(float64(transferred) / float64(total) * 100)
			if percent != lastPercent && percent%10 == 0 {
				d.log.Info("Download progress", "percent", percent, "transferred", cloud.FormatSize(transferred), "total", cloud.FormatSize(total))
				lastPercent = percent
			}
		}
	}

	// Download file
	if err := d.backend.Download(ctx, remotePath, localPath, progressCallback); err != nil {
		// Cleanup on failure
		os.RemoveAll(tempSubDir)
		return nil, fmt.Errorf("download failed: %w", err)
	}

	result := &DownloadResult{
		LocalPath:  localPath,
		RemotePath: remotePath,
		Size:       size,
		IsTempFile: !opts.KeepLocal,
	}

	// Try to download metadata file
	metaRemotePath := remotePath + ".meta.json"
	exists, err := d.backend.Exists(ctx, metaRemotePath)
	if err == nil && exists {
		metaLocalPath := localPath + ".meta.json"
		if err := d.backend.Download(ctx, metaRemotePath, metaLocalPath, nil); err != nil {
			d.log.Warn("Failed to download metadata", "error", err)
		} else {
			result.MetadataPath = metaLocalPath
			d.log.Debug("Downloaded metadata", "path", metaLocalPath)
		}
	}

	// Verify checksum if requested
	if opts.VerifyChecksum {
		d.log.Info("Verifying checksum...")
		checksum, err := calculateSHA256(localPath)
		if err != nil {
			// Cleanup on verification failure
			os.RemoveAll(tempSubDir)
			return nil, fmt.Errorf("checksum calculation failed: %w", err)
		}
		result.SHA256 = checksum

		// Check against metadata if available
		if result.MetadataPath != "" {
			meta, err := metadata.Load(result.MetadataPath)
			if err != nil {
				d.log.Warn("Failed to load metadata for verification", "error", err)
			} else if meta.SHA256 != "" && meta.SHA256 != checksum {
				// Cleanup on verification failure
				os.RemoveAll(tempSubDir)
				return nil, fmt.Errorf("checksum mismatch: expected %s, got %s", meta.SHA256, checksum)
			} else if meta.SHA256 == checksum {
				d.log.Info("Checksum verified successfully", "sha256", checksum)
			}
		}
	}

	d.log.Info("Download completed", "path", localPath, "size", cloud.FormatSize(result.Size))

	return result, nil
}

// DownloadFromURI downloads a backup using a cloud URI
func (d *CloudDownloader) DownloadFromURI(ctx context.Context, uri string, opts DownloadOptions) (*DownloadResult, error) {
	// Parse URI
	cloudURI, err := cloud.ParseCloudURI(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid cloud URI: %w", err)
	}

	// Download using the path from URI
	return d.Download(ctx, cloudURI.Path, opts)
}

// Cleanup removes downloaded temp files
func (r *DownloadResult) Cleanup() error {
	if !r.IsTempFile {
		return nil // Don't delete non-temp files
	}

	// Remove the entire temp directory
	tempDir := filepath.Dir(r.LocalPath)
	if err := os.RemoveAll(tempDir); err != nil {
		return fmt.Errorf("failed to cleanup temp files: %w", err)
	}

	return nil
}

// calculateSHA256 calculates the SHA-256 checksum of a file
func calculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
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

// DownloadFromCloudURI is a convenience function to download from a cloud URI
func DownloadFromCloudURI(ctx context.Context, uri string, opts DownloadOptions) (*DownloadResult, error) {
	// Parse URI
	cloudURI, err := cloud.ParseCloudURI(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid cloud URI: %w", err)
	}

	// Create config from URI
	cfg := cloudURI.ToConfig()

	// Create backend
	backend, err := cloud.NewBackend(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud backend: %w", err)
	}

	// Create downloader
	log := logger.New("info", "text")
	downloader := NewCloudDownloader(backend, log)

	// Download
	return downloader.Download(ctx, cloudURI.Path, opts)
}
