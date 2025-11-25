package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dbbackup/internal/cloud"
	"github.com/spf13/cobra"
)

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Cloud storage operations",
	Long: `Manage backups in cloud storage (S3, MinIO, Backblaze B2).

Supports:
- AWS S3
- MinIO (S3-compatible)
- Backblaze B2 (S3-compatible)
- Any S3-compatible storage

Configuration via flags or environment variables:
  --cloud-provider      DBBACKUP_CLOUD_PROVIDER
  --cloud-bucket        DBBACKUP_CLOUD_BUCKET
  --cloud-region        DBBACKUP_CLOUD_REGION
  --cloud-endpoint      DBBACKUP_CLOUD_ENDPOINT
  --cloud-access-key    DBBACKUP_CLOUD_ACCESS_KEY (or AWS_ACCESS_KEY_ID)
  --cloud-secret-key    DBBACKUP_CLOUD_SECRET_KEY (or AWS_SECRET_ACCESS_KEY)`,
}

var cloudUploadCmd = &cobra.Command{
	Use:   "upload [backup-file]",
	Short: "Upload backup to cloud storage",
	Long: `Upload one or more backup files to cloud storage.

Examples:
  # Upload single backup
  dbbackup cloud upload /backups/mydb.dump

  # Upload with progress
  dbbackup cloud upload /backups/mydb.dump --verbose

  # Upload multiple files
  dbbackup cloud upload /backups/*.dump`,
	Args: cobra.MinimumNArgs(1),
	RunE: runCloudUpload,
}

var cloudDownloadCmd = &cobra.Command{
	Use:   "download [remote-file] [local-path]",
	Short: "Download backup from cloud storage",
	Long: `Download a backup file from cloud storage.

Examples:
  # Download to current directory
  dbbackup cloud download mydb.dump .

  # Download to specific path
  dbbackup cloud download mydb.dump /backups/mydb.dump

  # Download with progress
  dbbackup cloud download mydb.dump . --verbose`,
	Args: cobra.ExactArgs(2),
	RunE: runCloudDownload,
}

var cloudListCmd = &cobra.Command{
	Use:   "list [prefix]",
	Short: "List backups in cloud storage",
	Long: `List all backup files in cloud storage.

Examples:
  # List all backups
  dbbackup cloud list

  # List backups with prefix
  dbbackup cloud list mydb_

  # List with detailed information
  dbbackup cloud list --verbose`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCloudList,
}

var cloudDeleteCmd = &cobra.Command{
	Use:   "delete [remote-file]",
	Short: "Delete backup from cloud storage",
	Long: `Delete a backup file from cloud storage.

Examples:
  # Delete single backup
  dbbackup cloud delete mydb_20251125.dump

  # Delete with confirmation
  dbbackup cloud delete mydb.dump --confirm`,
	Args: cobra.ExactArgs(1),
	RunE: runCloudDelete,
}

var (
	cloudProvider  string
	cloudBucket    string
	cloudRegion    string
	cloudEndpoint  string
	cloudAccessKey string
	cloudSecretKey string
	cloudPrefix    string
	cloudVerbose   bool
	cloudConfirm   bool
)

func init() {
	rootCmd.AddCommand(cloudCmd)
	cloudCmd.AddCommand(cloudUploadCmd, cloudDownloadCmd, cloudListCmd, cloudDeleteCmd)

	// Cloud configuration flags
	for _, cmd := range []*cobra.Command{cloudUploadCmd, cloudDownloadCmd, cloudListCmd, cloudDeleteCmd} {
		cmd.Flags().StringVar(&cloudProvider, "cloud-provider", getEnv("DBBACKUP_CLOUD_PROVIDER", "s3"), "Cloud provider (s3, minio, b2)")
		cmd.Flags().StringVar(&cloudBucket, "cloud-bucket", getEnv("DBBACKUP_CLOUD_BUCKET", ""), "Bucket name")
		cmd.Flags().StringVar(&cloudRegion, "cloud-region", getEnv("DBBACKUP_CLOUD_REGION", "us-east-1"), "Region")
		cmd.Flags().StringVar(&cloudEndpoint, "cloud-endpoint", getEnv("DBBACKUP_CLOUD_ENDPOINT", ""), "Custom endpoint (for MinIO)")
		cmd.Flags().StringVar(&cloudAccessKey, "cloud-access-key", getEnv("DBBACKUP_CLOUD_ACCESS_KEY", getEnv("AWS_ACCESS_KEY_ID", "")), "Access key")
		cmd.Flags().StringVar(&cloudSecretKey, "cloud-secret-key", getEnv("DBBACKUP_CLOUD_SECRET_KEY", getEnv("AWS_SECRET_ACCESS_KEY", "")), "Secret key")
		cmd.Flags().StringVar(&cloudPrefix, "cloud-prefix", getEnv("DBBACKUP_CLOUD_PREFIX", ""), "Key prefix")
		cmd.Flags().BoolVarP(&cloudVerbose, "verbose", "v", false, "Verbose output")
	}

	cloudDeleteCmd.Flags().BoolVar(&cloudConfirm, "confirm", false, "Skip confirmation prompt")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getCloudBackend() (cloud.Backend, error) {
	cfg := &cloud.Config{
		Provider:   cloudProvider,
		Bucket:     cloudBucket,
		Region:     cloudRegion,
		Endpoint:   cloudEndpoint,
		AccessKey:  cloudAccessKey,
		SecretKey:  cloudSecretKey,
		Prefix:     cloudPrefix,
		UseSSL:     true,
		PathStyle:  cloudProvider == "minio",
		Timeout:    300,
		MaxRetries: 3,
	}

	if cfg.Bucket == "" {
		return nil, fmt.Errorf("bucket name is required (use --cloud-bucket or DBBACKUP_CLOUD_BUCKET)")
	}

	backend, err := cloud.NewBackend(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud backend: %w", err)
	}

	return backend, nil
}

func runCloudUpload(cmd *cobra.Command, args []string) error {
	backend, err := getCloudBackend()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Expand glob patterns
	var files []string
	for _, pattern := range args {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}
		if len(matches) == 0 {
			files = append(files, pattern)
		} else {
			files = append(files, matches...)
		}
	}

	fmt.Printf("☁️  Uploading %d file(s) to %s...\n\n", len(files), backend.Name())

	successCount := 0
	for _, localPath := range files {
		filename := filepath.Base(localPath)
		fmt.Printf("📤 %s\n", filename)

		// Progress callback
		var lastPercent int
		progress := func(transferred, total int64) {
			if !cloudVerbose {
				return
			}
			percent := int(float64(transferred) / float64(total) * 100)
			if percent != lastPercent && percent%10 == 0 {
				fmt.Printf("   Progress: %d%% (%s / %s)\n", 
					percent, 
					cloud.FormatSize(transferred), 
					cloud.FormatSize(total))
				lastPercent = percent
			}
		}

		err := backend.Upload(ctx, localPath, filename, progress)
		if err != nil {
			fmt.Printf("   ❌ Failed: %v\n\n", err)
			continue
		}

		// Get file size
		if info, err := os.Stat(localPath); err == nil {
			fmt.Printf("   ✅ Uploaded (%s)\n\n", cloud.FormatSize(info.Size()))
		} else {
			fmt.Printf("   ✅ Uploaded\n\n")
		}
		successCount++
	}

	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("✅ Successfully uploaded %d/%d file(s)\n", successCount, len(files))

	return nil
}

func runCloudDownload(cmd *cobra.Command, args []string) error {
	backend, err := getCloudBackend()
	if err != nil {
		return err
	}

	ctx := context.Background()
	remotePath := args[0]
	localPath := args[1]

	// If localPath is a directory, use the remote filename
	if info, err := os.Stat(localPath); err == nil && info.IsDir() {
		localPath = filepath.Join(localPath, filepath.Base(remotePath))
	}

	fmt.Printf("☁️  Downloading from %s...\n\n", backend.Name())
	fmt.Printf("📥 %s → %s\n", remotePath, localPath)

	// Progress callback
	var lastPercent int
	progress := func(transferred, total int64) {
		if !cloudVerbose {
			return
		}
		percent := int(float64(transferred) / float64(total) * 100)
		if percent != lastPercent && percent%10 == 0 {
			fmt.Printf("   Progress: %d%% (%s / %s)\n", 
				percent, 
				cloud.FormatSize(transferred), 
				cloud.FormatSize(total))
			lastPercent = percent
		}
	}

	err = backend.Download(ctx, remotePath, localPath, progress)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Get file size
	if info, err := os.Stat(localPath); err == nil {
		fmt.Printf("   ✅ Downloaded (%s)\n", cloud.FormatSize(info.Size()))
	} else {
		fmt.Printf("   ✅ Downloaded\n")
	}

	return nil
}

func runCloudList(cmd *cobra.Command, args []string) error {
	backend, err := getCloudBackend()
	if err != nil {
		return err
	}

	ctx := context.Background()
	prefix := ""
	if len(args) > 0 {
		prefix = args[0]
	}

	fmt.Printf("☁️  Listing backups in %s/%s...\n\n", backend.Name(), cloudBucket)

	backups, err := backend.List(ctx, prefix)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	if len(backups) == 0 {
		fmt.Println("No backups found")
		return nil
	}

	var totalSize int64
	for _, backup := range backups {
		totalSize += backup.Size
		
		if cloudVerbose {
			fmt.Printf("📦 %s\n", backup.Name)
			fmt.Printf("   Size: %s\n", cloud.FormatSize(backup.Size))
			fmt.Printf("   Modified: %s\n", backup.LastModified.Format(time.RFC3339))
			if backup.StorageClass != "" {
				fmt.Printf("   Storage: %s\n", backup.StorageClass)
			}
			fmt.Println()
		} else {
			age := time.Since(backup.LastModified)
			ageStr := formatAge(age)
			fmt.Printf("%-50s %12s  %s\n", 
				backup.Name, 
				cloud.FormatSize(backup.Size),
				ageStr)
		}
	}

	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Total: %d backup(s), %s\n", len(backups), cloud.FormatSize(totalSize))

	return nil
}

func runCloudDelete(cmd *cobra.Command, args []string) error {
	backend, err := getCloudBackend()
	if err != nil {
		return err
	}

	ctx := context.Background()
	remotePath := args[0]

	// Check if file exists
	exists, err := backend.Exists(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("failed to check file: %w", err)
	}
	if !exists {
		return fmt.Errorf("file not found: %s", remotePath)
	}

	// Get file info
	size, err := backend.GetSize(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Confirmation prompt
	if !cloudConfirm {
		fmt.Printf("⚠️  Delete %s (%s) from cloud storage?\n", remotePath, cloud.FormatSize(size))
		fmt.Print("Type 'yes' to confirm: ")
		var response string
		fmt.Scanln(&response)
		if response != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	fmt.Printf("🗑️  Deleting %s...\n", remotePath)

	err = backend.Delete(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	fmt.Printf("✅ Deleted %s (%s)\n", remotePath, cloud.FormatSize(size))

	return nil
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	} else if d < time.Hour {
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	} else {
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}
