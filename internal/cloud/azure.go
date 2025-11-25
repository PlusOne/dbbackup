package cloud

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

// AzureBackend implements the Backend interface for Azure Blob Storage
type AzureBackend struct {
	client        *azblob.Client
	containerName string
	config        *Config
}

// NewAzureBackend creates a new Azure Blob Storage backend
func NewAzureBackend(cfg *Config) (*AzureBackend, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("container name is required for Azure backend")
	}

	var client *azblob.Client
	var err error

	// Support for Azurite emulator (uses endpoint override)
	if cfg.Endpoint != "" {
		// For Azurite and custom endpoints
		accountName := cfg.AccessKey
		accountKey := cfg.SecretKey

		if accountName == "" {
			// Default Azurite account
			accountName = "devstoreaccount1"
		}
		if accountKey == "" {
			// Default Azurite key
			accountKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
		}

		// Create credential
		cred, err := azblob.NewSharedKeyCredential(accountName, accountKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create Azure credential: %w", err)
		}

		// Build service URL for Azurite: http://endpoint/accountName
		serviceURL := cfg.Endpoint
		if !strings.Contains(serviceURL, accountName) {
			// Ensure URL ends with slash
			if !strings.HasSuffix(serviceURL, "/") {
				serviceURL += "/"
			}
			serviceURL += accountName
		}

		client, err = azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create Azure client: %w", err)
		}
	} else {
		// Production Azure using connection string or managed identity
		if cfg.AccessKey != "" && cfg.SecretKey != "" {
			// Use account name and key
			accountName := cfg.AccessKey
			accountKey := cfg.SecretKey

			cred, err := azblob.NewSharedKeyCredential(accountName, accountKey)
			if err != nil {
				return nil, fmt.Errorf("failed to create Azure credential: %w", err)
			}

			serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", accountName)
			client, err = azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create Azure client: %w", err)
			}
		} else {
			// Use default Azure credential (managed identity, environment variables, etc.)
			return nil, fmt.Errorf("Azure authentication requires account name and key, or use AZURE_STORAGE_CONNECTION_STRING environment variable")
		}
	}

	backend := &AzureBackend{
		client:        client,
		containerName: cfg.Bucket,
		config:        cfg,
	}

	// Create container if it doesn't exist
	// Note: Container creation should be done manually or via Azure portal
	if false { // Disabled: cfg.CreateBucket not in Config
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		containerClient := client.ServiceClient().NewContainerClient(cfg.Bucket)
		_, err = containerClient.Create(ctx, &container.CreateOptions{})
		if err != nil {
			// Ignore if container already exists
			if !strings.Contains(err.Error(), "ContainerAlreadyExists") {
				return nil, fmt.Errorf("failed to create container: %w", err)
			}
		}
	}

	return backend, nil
}

// Name returns the backend name
func (a *AzureBackend) Name() string {
	return "azure"
}

// Upload uploads a file to Azure Blob Storage
func (a *AzureBackend) Upload(ctx context.Context, localPath, remotePath string, progress ProgressCallback) error {
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
	blobName := strings.TrimPrefix(remotePath, "/")

	// Use block blob upload for large files (>256MB), simple upload for smaller
	const blockUploadThreshold = 256 * 1024 * 1024 // 256 MB

	if fileSize > blockUploadThreshold {
		return a.uploadBlocks(ctx, file, blobName, fileSize, progress)
	}

	return a.uploadSimple(ctx, file, blobName, fileSize, progress)
}

// uploadSimple uploads a file using simple upload (single request)
func (a *AzureBackend) uploadSimple(ctx context.Context, file *os.File, blobName string, fileSize int64, progress ProgressCallback) error {
	blockBlobClient := a.client.ServiceClient().NewContainerClient(a.containerName).NewBlockBlobClient(blobName)

	// Wrap reader with progress tracking
	reader := NewProgressReader(file, fileSize, progress)

	// Calculate MD5 hash for integrity
	hash := sha256.New()
	teeReader := io.TeeReader(reader, hash)

	_, err := blockBlobClient.UploadStream(ctx, teeReader, &blockblob.UploadStreamOptions{
		BlockSize: 4 * 1024 * 1024, // 4MB blocks
	})
	if err != nil {
		return fmt.Errorf("failed to upload blob: %w", err)
	}

	// Store checksum as metadata
	checksum := hex.EncodeToString(hash.Sum(nil))
	metadata := map[string]*string{
		"sha256": &checksum,
	}

	_, err = blockBlobClient.SetMetadata(ctx, metadata, nil)
	if err != nil {
		// Non-fatal: upload succeeded but metadata failed
		fmt.Fprintf(os.Stderr, "Warning: failed to set blob metadata: %v\n", err)
	}

	return nil
}

// uploadBlocks uploads a file using block blob staging (for large files)
func (a *AzureBackend) uploadBlocks(ctx context.Context, file *os.File, blobName string, fileSize int64, progress ProgressCallback) error {
	blockBlobClient := a.client.ServiceClient().NewContainerClient(a.containerName).NewBlockBlobClient(blobName)

	const blockSize = 100 * 1024 * 1024 // 100MB per block
	numBlocks := (fileSize + blockSize - 1) / blockSize

	blockIDs := make([]string, 0, numBlocks)
	hash := sha256.New()
	var totalUploaded int64

	for i := int64(0); i < numBlocks; i++ {
		blockID := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("block-%08d", i)))
		blockIDs = append(blockIDs, blockID)

		// Calculate block size
		currentBlockSize := blockSize
		if i == numBlocks-1 {
			currentBlockSize = int(fileSize - i*blockSize)
		}

		// Read block
		blockData := make([]byte, currentBlockSize)
		n, err := io.ReadFull(file, blockData)
		if err != nil && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("failed to read block %d: %w", i, err)
		}
		blockData = blockData[:n]

		// Update hash
		hash.Write(blockData)

		// Upload block
		reader := bytes.NewReader(blockData)
		_, err = blockBlobClient.StageBlock(ctx, blockID, streaming.NopCloser(reader), nil)
		if err != nil {
			return fmt.Errorf("failed to stage block %d: %w", i, err)
		}

		// Update progress
		totalUploaded += int64(n)
		if progress != nil {
			progress(totalUploaded, fileSize)
		}
	}

	// Commit all blocks
	_, err := blockBlobClient.CommitBlockList(ctx, blockIDs, nil)
	if err != nil {
		return fmt.Errorf("failed to commit block list: %w", err)
	}

	// Store checksum as metadata
	checksum := hex.EncodeToString(hash.Sum(nil))
	metadata := map[string]*string{
		"sha256": &checksum,
	}

	_, err = blockBlobClient.SetMetadata(ctx, metadata, nil)
	if err != nil {
		// Non-fatal
		fmt.Fprintf(os.Stderr, "Warning: failed to set blob metadata: %v\n", err)
	}

	return nil
}

// Download downloads a file from Azure Blob Storage
func (a *AzureBackend) Download(ctx context.Context, remotePath, localPath string, progress ProgressCallback) error {
	blobName := strings.TrimPrefix(remotePath, "/")
	blockBlobClient := a.client.ServiceClient().NewContainerClient(a.containerName).NewBlockBlobClient(blobName)

	// Get blob properties to know size
	props, err := blockBlobClient.GetProperties(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get blob properties: %w", err)
	}

	fileSize := *props.ContentLength

	// Download blob
	resp, err := blockBlobClient.DownloadStream(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to download blob: %w", err)
	}
	defer resp.Body.Close()

	// Create local file
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Wrap reader with progress tracking
	reader := NewProgressReader(resp.Body, fileSize, progress)

	// Copy with progress
	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Delete deletes a file from Azure Blob Storage
func (a *AzureBackend) Delete(ctx context.Context, remotePath string) error {
	blobName := strings.TrimPrefix(remotePath, "/")
	blockBlobClient := a.client.ServiceClient().NewContainerClient(a.containerName).NewBlockBlobClient(blobName)

	_, err := blockBlobClient.Delete(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete blob: %w", err)
	}

	return nil
}

// List lists files in Azure Blob Storage with a given prefix
func (a *AzureBackend) List(ctx context.Context, prefix string) ([]BackupInfo, error) {
	prefix = strings.TrimPrefix(prefix, "/")
	containerClient := a.client.ServiceClient().NewContainerClient(a.containerName)

	pager := containerClient.NewListBlobsFlatPager(&container.ListBlobsFlatOptions{
		Prefix: &prefix,
	})

	var files []BackupInfo

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list blobs: %w", err)
		}

		for _, blob := range page.Segment.BlobItems {
			if blob.Name == nil || blob.Properties == nil {
				continue
			}

			file := BackupInfo{
				Key:          *blob.Name,
				Name:         filepath.Base(*blob.Name),
				Size:         *blob.Properties.ContentLength,
				LastModified: *blob.Properties.LastModified,
			}

			// Try to get SHA256 from metadata
			if blob.Metadata != nil {
				if sha256Val, ok := blob.Metadata["sha256"]; ok && sha256Val != nil {
					file.ETag = *sha256Val
				}
			}

			files = append(files, file)
		}
	}

	return files, nil
}

// Exists checks if a file exists in Azure Blob Storage
func (a *AzureBackend) Exists(ctx context.Context, remotePath string) (bool, error) {
	blobName := strings.TrimPrefix(remotePath, "/")
	blockBlobClient := a.client.ServiceClient().NewContainerClient(a.containerName).NewBlockBlobClient(blobName)

	_, err := blockBlobClient.GetProperties(ctx, nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if respErr != nil && respErr.StatusCode == 404 {
			return false, nil
		}
		// Check if error message contains "not found"
		if strings.Contains(err.Error(), "BlobNotFound") || strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check blob existence: %w", err)
	}

	return true, nil
}

// GetSize returns the size of a file in Azure Blob Storage
func (a *AzureBackend) GetSize(ctx context.Context, remotePath string) (int64, error) {
	blobName := strings.TrimPrefix(remotePath, "/")
	blockBlobClient := a.client.ServiceClient().NewContainerClient(a.containerName).NewBlockBlobClient(blobName)

	props, err := blockBlobClient.GetProperties(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get blob properties: %w", err)
	}

	return *props.ContentLength, nil
}
