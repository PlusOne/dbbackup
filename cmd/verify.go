package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dbbackup/internal/metadata"
	"dbbackup/internal/verification"
	"github.com/spf13/cobra"
)

var verifyBackupCmd = &cobra.Command{
	Use:   "verify-backup [backup-file]",
	Short: "Verify backup file integrity with checksums",
	Long: `Verify the integrity of one or more backup files by comparing their SHA-256 checksums
against the stored metadata. This ensures that backups have not been corrupted.

Examples:
  # Verify a single backup
  dbbackup verify-backup /backups/mydb_20260115.dump

  # Verify all backups in a directory
  dbbackup verify-backup /backups/*.dump

  # Quick verification (size check only, no checksum)
  dbbackup verify-backup /backups/mydb.dump --quick

  # Verify and show detailed information
  dbbackup verify-backup /backups/mydb.dump --verbose`,
	Args: cobra.MinimumNArgs(1),
	RunE: runVerifyBackup,
}

var (
	quickVerify   bool
	verboseVerify bool
)

func init() {
	rootCmd.AddCommand(verifyBackupCmd)
	verifyBackupCmd.Flags().BoolVar(&quickVerify, "quick", false, "Quick verification (size check only)")
	verifyBackupCmd.Flags().BoolVarP(&verboseVerify, "verbose", "v", false, "Show detailed information")
}

func runVerifyBackup(cmd *cobra.Command, args []string) error {
	// Expand glob patterns
	var backupFiles []string
	for _, pattern := range args {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}
		if len(matches) == 0 {
			// Not a glob, use as-is
			backupFiles = append(backupFiles, pattern)
		} else {
			backupFiles = append(backupFiles, matches...)
		}
	}

	if len(backupFiles) == 0 {
		return fmt.Errorf("no backup files found")
	}

	fmt.Printf("Verifying %d backup file(s)...\n\n", len(backupFiles))

	successCount := 0
	failureCount := 0

	for _, backupFile := range backupFiles {
		// Skip metadata files
		if strings.HasSuffix(backupFile, ".meta.json") || 
		   strings.HasSuffix(backupFile, ".sha256") || 
		   strings.HasSuffix(backupFile, ".info") {
			continue
		}

		fmt.Printf("📁 %s\n", filepath.Base(backupFile))

		if quickVerify {
			// Quick check: size only
			err := verification.QuickCheck(backupFile)
			if err != nil {
				fmt.Printf("   ❌ FAILED: %v\n\n", err)
				failureCount++
				continue
			}
			fmt.Printf("   ✅ VALID (quick check)\n\n")
			successCount++
		} else {
			// Full verification with SHA-256
			result, err := verification.Verify(backupFile)
			if err != nil {
				return fmt.Errorf("verification error: %w", err)
			}

			if result.Valid {
				fmt.Printf("   ✅ VALID\n")
				if verboseVerify {
					meta, _ := metadata.Load(backupFile)
					fmt.Printf("   Size: %s\n", metadata.FormatSize(meta.SizeBytes))
					fmt.Printf("   SHA-256: %s\n", meta.SHA256)
					fmt.Printf("   Database: %s (%s)\n", meta.Database, meta.DatabaseType)
					fmt.Printf("   Created: %s\n", meta.Timestamp.Format(time.RFC3339))
				}
				fmt.Println()
				successCount++
			} else {
				fmt.Printf("   ❌ FAILED: %v\n", result.Error)
				if verboseVerify {
					if !result.FileExists {
						fmt.Printf("   File does not exist\n")
					} else if !result.MetadataExists {
						fmt.Printf("   Metadata file missing\n")
					} else if !result.SizeMatch {
						fmt.Printf("   Size mismatch\n")
					} else {
						fmt.Printf("   Expected: %s\n", result.ExpectedSHA256)
						fmt.Printf("   Got:      %s\n", result.CalculatedSHA256)
					}
				}
				fmt.Println()
				failureCount++
			}
		}
	}

	// Summary
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Total: %d backups\n", len(backupFiles))
	fmt.Printf("✅ Valid: %d\n", successCount)
	if failureCount > 0 {
		fmt.Printf("❌ Failed: %d\n", failureCount)
		os.Exit(1)
	}

	return nil
}
