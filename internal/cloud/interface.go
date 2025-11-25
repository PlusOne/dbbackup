package cloud

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Backend defines the interface for cloud storage providers
type Backend interface {
	// Upload uploads a file to cloud storage
	Upload(ctx context.Context, localPath, remotePath string, progress ProgressCallback) error
	
	// Download downloads a file from cloud storage
	Download(ctx context.Context, remotePath, localPath string, progress ProgressCallback) error
	
	// List lists all backup files in cloud storage
	List(ctx context.Context, prefix string) ([]BackupInfo, error)
	
	// Delete deletes a file from cloud storage
	Delete(ctx context.Context, remotePath string) error
	
	// Exists checks if a file exists in cloud storage
	Exists(ctx context.Context, remotePath string) (bool, error)
	
	// GetSize returns the size of a remote file
	GetSize(ctx context.Context, remotePath string) (int64, error)
	
	// Name returns the backend name (e.g., "s3", "azure", "gcs")
	Name() string
}

// BackupInfo contains information about a backup in cloud storage
type BackupInfo struct {
	Key          string    // Full path/key in cloud storage
	Name         string    // Base filename
	Size         int64     // Size in bytes
	LastModified time.Time // Last modification time
	ETag         string    // Entity tag (version identifier)
	StorageClass string    // Storage class (e.g., STANDARD, GLACIER)
}

// ProgressCallback is called during upload/download to report progress
type ProgressCallback func(bytesTransferred, totalBytes int64)

// Config contains common configuration for cloud backends
type Config struct {
	Provider    string // "s3", "minio", "azure", "gcs", "b2"
	Bucket      string // Bucket or container name
	Region      string // Region (for S3)
	Endpoint    string // Custom endpoint (for MinIO, S3-compatible)
	AccessKey   string // Access key or account ID
	SecretKey   string // Secret key or access token
	UseSSL      bool   // Use SSL/TLS (default: true)
	PathStyle   bool   // Use path-style addressing (for MinIO)
	Prefix      string // Prefix for all operations (e.g., "backups/")
	Timeout     int    // Timeout in seconds (default: 300)
	MaxRetries  int    // Maximum retry attempts (default: 3)
	Concurrency int    // Upload/download concurrency (default: 5)
}

// NewBackend creates a new cloud storage backend based on the provider
func NewBackend(cfg *Config) (Backend, error) {
	switch cfg.Provider {
	case "s3", "aws":
		return NewS3Backend(cfg)
	case "minio":
		// MinIO uses S3 backend with custom endpoint
		cfg.PathStyle = true
		if cfg.Endpoint == "" {
			return nil, fmt.Errorf("endpoint required for MinIO")
		}
		return NewS3Backend(cfg)
	case "b2", "backblaze":
		// Backblaze B2 uses S3-compatible API
		cfg.PathStyle = false
		if cfg.Endpoint == "" {
			return nil, fmt.Errorf("endpoint required for Backblaze B2")
		}
		return NewS3Backend(cfg)
	case "azure", "azblob":
		return NewAzureBackend(cfg)
	case "gs", "gcs", "google":
		return NewGCSBackend(cfg)
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s (supported: s3, minio, b2, azure, gcs)", cfg.Provider)
	}
}

// FormatSize returns human-readable size
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Provider:    "s3",
		UseSSL:      true,
		PathStyle:   false,
		Timeout:     300,
		MaxRetries:  3,
		Concurrency: 5,
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if c.Bucket == "" {
		return fmt.Errorf("bucket name is required")
	}
	if c.Provider == "s3" || c.Provider == "aws" {
		if c.Region == "" && c.Endpoint == "" {
			return fmt.Errorf("region or endpoint is required for S3")
		}
	}
	if c.Provider == "minio" || c.Provider == "b2" {
		if c.Endpoint == "" {
			return fmt.Errorf("endpoint is required for %s", c.Provider)
		}
	}
	return nil
}

// ProgressReader wraps an io.Reader to track progress
type ProgressReader struct {
	reader    io.Reader
	total     int64
	read      int64
	callback  ProgressCallback
	lastReport time.Time
}

// NewProgressReader creates a progress tracking reader
func NewProgressReader(r io.Reader, total int64, callback ProgressCallback) *ProgressReader {
	return &ProgressReader{
		reader:     r,
		total:      total,
		callback:   callback,
		lastReport: time.Now(),
	}
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.read += int64(n)
	
	// Report progress every 100ms or when complete
	now := time.Now()
	if now.Sub(pr.lastReport) > 100*time.Millisecond || err == io.EOF {
		if pr.callback != nil {
			pr.callback(pr.read, pr.total)
		}
		pr.lastReport = now
	}
	
	return n, err
}
