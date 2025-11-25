package cloud

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

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCSBackend implements the Backend interface for Google Cloud Storage
type GCSBackend struct {
	client     *storage.Client
	bucketName string
	config     *Config
}

// NewGCSBackend creates a new Google Cloud Storage backend
func NewGCSBackend(cfg *Config) (*GCSBackend, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("bucket name is required for GCS backend")
	}

	var client *storage.Client
	var err error
	ctx := context.Background()

	// Support for fake-gcs-server emulator (uses endpoint override)
	if cfg.Endpoint != "" {
		// For fake-gcs-server and custom endpoints
		client, err = storage.NewClient(ctx, option.WithEndpoint(cfg.Endpoint), option.WithoutAuthentication())
		if err != nil {
			return nil, fmt.Errorf("failed to create GCS client: %w", err)
		}
	} else {
		// Production GCS using Application Default Credentials or service account
		if cfg.AccessKey != "" {
			// Use service account JSON key file
			client, err = storage.NewClient(ctx, option.WithCredentialsFile(cfg.AccessKey))
			if err != nil {
				return nil, fmt.Errorf("failed to create GCS client with credentials file: %w", err)
			}
		} else {
			// Use default credentials (ADC, environment variables, etc.)
			client, err = storage.NewClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to create GCS client: %w", err)
			}
		}
	}

	backend := &GCSBackend{
		client:     client,
		bucketName: cfg.Bucket,
		config:     cfg,
	}

	// Create bucket if it doesn't exist
	// Note: Bucket creation should be done manually or via gcloud CLI
	if false { // Disabled: cfg.CreateBucket not in Config
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		bucket := client.Bucket(cfg.Bucket)
		_, err = bucket.Attrs(ctx)
		if err == storage.ErrBucketNotExist {
			// Create bucket with default settings
			if err := bucket.Create(ctx, cfg.AccessKey, nil); err != nil {
				return nil, fmt.Errorf("failed to create bucket: %w", err)
			}
		} else if err != nil {
			return nil, fmt.Errorf("failed to check bucket: %w", err)
		}
	}

	return backend, nil
}

// Name returns the backend name
func (g *GCSBackend) Name() string {
	return "gcs"
}

// Upload uploads a file to Google Cloud Storage
func (g *GCSBackend) Upload(ctx context.Context, localPath, remotePath string, progress ProgressCallback) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := fileInfo.Size()

	// Remove leading slash from remote path
	objectName := strings.TrimPrefix(remotePath, "/")

	bucket := g.client.Bucket(g.bucketName)
	object := bucket.Object(objectName)

	// Create writer with automatic chunking for large files
	writer := object.NewWriter(ctx)
	writer.ChunkSize = 16 * 1024 * 1024 // 16MB chunks for streaming

	// Wrap reader with progress tracking and hash calculation
	hash := sha256.New()
	reader := NewProgressReader(io.TeeReader(file, hash), fileSize, progress)

	// Upload with progress tracking
	_, err = io.Copy(writer, reader)
	if err != nil {
		writer.Close()
		return fmt.Errorf("failed to upload object: %w", err)
	}

	// Close writer (finalizes upload)
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to finalize upload: %w", err)
	}

	// Store checksum as metadata
	checksum := hex.EncodeToString(hash.Sum(nil))
	_, err = object.Update(ctx, storage.ObjectAttrsToUpdate{
		Metadata: map[string]string{
			"sha256": checksum,
		},
	})
	if err != nil {
		// Non-fatal: upload succeeded but metadata failed
		fmt.Fprintf(os.Stderr, "Warning: failed to set object metadata: %v\n", err)
	}

	return nil
}

// Download downloads a file from Google Cloud Storage
func (g *GCSBackend) Download(ctx context.Context, remotePath, localPath string, progress ProgressCallback) error {
	objectName := strings.TrimPrefix(remotePath, "/")

	bucket := g.client.Bucket(g.bucketName)
	object := bucket.Object(objectName)

	// Get object attributes to know size
	attrs, err := object.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get object attributes: %w", err)
	}

	fileSize := attrs.Size

	// Create reader
	reader, err := object.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to download object: %w", err)
	}
	defer reader.Close()

	// Create local file
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Wrap reader with progress tracking
	progressReader := NewProgressReader(reader, fileSize, progress)

	// Copy with progress
	_, err = io.Copy(file, progressReader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Delete deletes a file from Google Cloud Storage
func (g *GCSBackend) Delete(ctx context.Context, remotePath string) error {
	objectName := strings.TrimPrefix(remotePath, "/")

	bucket := g.client.Bucket(g.bucketName)
	object := bucket.Object(objectName)

	if err := object.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// List lists files in Google Cloud Storage with a given prefix
func (g *GCSBackend) List(ctx context.Context, prefix string) ([]BackupInfo, error) {
	prefix = strings.TrimPrefix(prefix, "/")

	bucket := g.client.Bucket(g.bucketName)
	query := &storage.Query{
		Prefix: prefix,
	}

	it := bucket.Objects(ctx, query)

	var files []BackupInfo

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		file := BackupInfo{
			Key:          attrs.Name,
			Name:         filepath.Base(attrs.Name),
			Size:         attrs.Size,
			LastModified: attrs.Updated,
		}

		// Try to get SHA256 from metadata
		if attrs.Metadata != nil {
			if sha256Val, ok := attrs.Metadata["sha256"]; ok {
				file.ETag = sha256Val
			}
		}

		files = append(files, file)
	}

	return files, nil
}

// Exists checks if a file exists in Google Cloud Storage
func (g *GCSBackend) Exists(ctx context.Context, remotePath string) (bool, error) {
	objectName := strings.TrimPrefix(remotePath, "/")

	bucket := g.client.Bucket(g.bucketName)
	object := bucket.Object(objectName)

	_, err := object.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}

	return true, nil
}

// GetSize returns the size of a file in Google Cloud Storage
func (g *GCSBackend) GetSize(ctx context.Context, remotePath string) (int64, error) {
	objectName := strings.TrimPrefix(remotePath, "/")

	bucket := g.client.Bucket(g.bucketName)
	object := bucket.Object(objectName)

	attrs, err := object.Attrs(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get object attributes: %w", err)
	}

	return attrs.Size, nil
}
