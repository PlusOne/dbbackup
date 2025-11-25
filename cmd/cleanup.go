package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dbbackup/internal/metadata"
	"dbbackup/internal/retention"
	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup [backup-directory]",
	Short: "Clean up old backups based on retention policy",
	Long: `Remove old backup files based on retention policy while maintaining minimum backup count.

The retention policy ensures:
1. Backups older than --retention-days are eligible for deletion
2. At least --min-backups most recent backups are always kept
3. Both conditions must be met for deletion

Examples:
  # Clean up backups older than 30 days (keep at least 5)
  dbbackup cleanup /backups --retention-days 30 --min-backups 5

  # Dry run to see what would be deleted
  dbbackup cleanup /backups --retention-days 7 --dry-run

  # Clean up specific database backups only
  dbbackup cleanup /backups --pattern "mydb_*.dump"

  # Aggressive cleanup (keep only 3 most recent)
  dbbackup cleanup /backups --retention-days 1 --min-backups 3`,
	Args: cobra.ExactArgs(1),
	RunE: runCleanup,
}

var (
	retentionDays int
	minBackups    int
	dryRun        bool
	cleanupPattern string
)

func init() {
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.Flags().IntVar(&retentionDays, "retention-days", 30, "Delete backups older than this many days")
	cleanupCmd.Flags().IntVar(&minBackups, "min-backups", 5, "Always keep at least this many backups")
	cleanupCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be deleted without actually deleting")
	cleanupCmd.Flags().StringVar(&cleanupPattern, "pattern", "", "Only clean up backups matching this pattern (e.g., 'mydb_*.dump')")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	backupDir := args[0]

	// Validate directory exists
	if !dirExists(backupDir) {
		return fmt.Errorf("backup directory does not exist: %s", backupDir)
	}

	// Create retention policy
	policy := retention.Policy{
		RetentionDays: retentionDays,
		MinBackups:    minBackups,
		DryRun:        dryRun,
	}

	fmt.Printf("🗑️  Cleanup Policy:\n")
	fmt.Printf("   Directory: %s\n", backupDir)
	fmt.Printf("   Retention: %d days\n", policy.RetentionDays)
	fmt.Printf("   Min backups: %d\n", policy.MinBackups)
	if cleanupPattern != "" {
		fmt.Printf("   Pattern: %s\n", cleanupPattern)
	}
	if dryRun {
		fmt.Printf("   Mode: DRY RUN (no files will be deleted)\n")
	}
	fmt.Println()

	var result *retention.CleanupResult
	var err error

	// Apply policy
	if cleanupPattern != "" {
		result, err = retention.CleanupByPattern(backupDir, cleanupPattern, policy)
	} else {
		result, err = retention.ApplyPolicy(backupDir, policy)
	}

	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	// Display results
	fmt.Printf("📊 Results:\n")
	fmt.Printf("   Total backups: %d\n", result.TotalBackups)
	fmt.Printf("   Eligible for deletion: %d\n", result.EligibleForDeletion)
	
	if len(result.Deleted) > 0 {
		fmt.Printf("\n")
		if dryRun {
			fmt.Printf("🔍 Would delete %d backup(s):\n", len(result.Deleted))
		} else {
			fmt.Printf("✅ Deleted %d backup(s):\n", len(result.Deleted))
		}
		for _, file := range result.Deleted {
			fmt.Printf("   - %s\n", filepath.Base(file))
		}
	}

	if len(result.Kept) > 0 && len(result.Kept) <= 10 {
		fmt.Printf("\n📦 Kept %d backup(s):\n", len(result.Kept))
		for _, file := range result.Kept {
			fmt.Printf("   - %s\n", filepath.Base(file))
		}
	} else if len(result.Kept) > 10 {
		fmt.Printf("\n📦 Kept %d backup(s)\n", len(result.Kept))
	}

	if !dryRun && result.SpaceFreed > 0 {
		fmt.Printf("\n💾 Space freed: %s\n", metadata.FormatSize(result.SpaceFreed))
	}

	if len(result.Errors) > 0 {
		fmt.Printf("\n⚠️  Errors:\n")
		for _, err := range result.Errors {
			fmt.Printf("   - %v\n", err)
		}
	}

	fmt.Println(strings.Repeat("─", 50))
	
	if dryRun {
		fmt.Println("✅ Dry run completed (no files were deleted)")
	} else if len(result.Deleted) > 0 {
		fmt.Println("✅ Cleanup completed successfully")
	} else {
		fmt.Println("ℹ️  No backups eligible for deletion")
	}

	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
