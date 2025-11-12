package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"dbbackup/internal/database"
	"dbbackup/internal/restore"

	"github.com/spf13/cobra"
)

var (
	restoreConfirm   bool
	restoreDryRun    bool
	restoreForce     bool
	restoreClean     bool
	restoreCreate    bool
	restoreJobs      int
	restoreTarget    string
	restoreVerbose   bool
	restoreNoProgress bool
)

// restoreCmd represents the restore command
var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore databases from backup archives",
	Long: `Restore databases from backup archives.

By default, restore runs in dry-run mode showing what would be restored.
Use --confirm flag to perform actual restoration.

Examples:
  # Preview restore (dry-run)
  dbbackup restore single mydb.dump.gz

  # Restore single database
  dbbackup restore single mydb.dump.gz --confirm

  # Restore to different database name
  dbbackup restore single mydb.dump.gz --target mydb_restored --confirm

  # Restore cluster backup
  dbbackup restore cluster cluster_backup_20240101_120000.tar.gz --confirm

  # List backup archives
  dbbackup restore list
`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// restoreSingleCmd restores a single database
var restoreSingleCmd = &cobra.Command{
	Use:   "single [archive-file]",
	Short: "Restore a single database from archive",
	Long: `Restore a single database from a backup archive.

Supported formats:
  - PostgreSQL: .dump, .dump.gz, .sql, .sql.gz
  - MySQL: .sql, .sql.gz

Safety features:
  - Dry-run by default (use --confirm to execute)
  - Archive validation before restore
  - Disk space verification
  - Optional database backup before restore

Examples:
  # Preview restore
  dbbackup restore single mydb.dump.gz

  # Restore to original database
  dbbackup restore single mydb.dump.gz --confirm

  # Restore to different database
  dbbackup restore single mydb.dump.gz --target mydb_test --confirm

  # Clean target database before restore
  dbbackup restore single mydb.sql.gz --clean --confirm

  # Create database if it doesn't exist
  dbbackup restore single mydb.sql --create --confirm
`,
	Args: cobra.ExactArgs(1),
	RunE: runRestoreSingle,
}

// restoreClusterCmd restores a full cluster
var restoreClusterCmd = &cobra.Command{
	Use:   "cluster [archive-file]",
	Short: "Restore full cluster from tar.gz archive",
	Long: `Restore a complete database cluster from a tar.gz archive.

This command restores all databases that were backed up together
in a cluster backup operation.

Safety features:
  - Dry-run by default (use --confirm to execute)
  - Archive validation and listing
  - Disk space verification
  - Sequential database restoration

Examples:
  # Preview cluster restore
  dbbackup restore cluster cluster_backup_20240101_120000.tar.gz

  # Restore full cluster
  dbbackup restore cluster cluster_backup_20240101_120000.tar.gz --confirm

  # Use parallel decompression
  dbbackup restore cluster cluster_backup.tar.gz --jobs 4 --confirm
`,
	Args: cobra.ExactArgs(1),
	RunE: runRestoreCluster,
}

// restoreListCmd lists available backup archives
var restoreListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available backup archives",
	Long: `List all backup archives in the backup directory.

Shows information about each archive:
  - Filename and path
  - Archive format (PostgreSQL dump, MySQL SQL, cluster)
  - File size
  - Last modification time
  - Database name (if detectable)
`,
	RunE: runRestoreList,
}

func init() {
	rootCmd.AddCommand(restoreCmd)
	restoreCmd.AddCommand(restoreSingleCmd)
	restoreCmd.AddCommand(restoreClusterCmd)
	restoreCmd.AddCommand(restoreListCmd)

	// Single restore flags
	restoreSingleCmd.Flags().BoolVar(&restoreConfirm, "confirm", false, "Confirm and execute restore (required)")
	restoreSingleCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, "Show what would be done without executing")
	restoreSingleCmd.Flags().BoolVar(&restoreForce, "force", false, "Skip safety checks and confirmations")
	restoreSingleCmd.Flags().BoolVar(&restoreClean, "clean", false, "Drop and recreate target database")
	restoreSingleCmd.Flags().BoolVar(&restoreCreate, "create", false, "Create target database if it doesn't exist")
	restoreSingleCmd.Flags().StringVar(&restoreTarget, "target", "", "Target database name (defaults to original)")
	restoreSingleCmd.Flags().BoolVar(&restoreVerbose, "verbose", false, "Show detailed restore progress")
	restoreSingleCmd.Flags().BoolVar(&restoreNoProgress, "no-progress", false, "Disable progress indicators")

	// Cluster restore flags
	restoreClusterCmd.Flags().BoolVar(&restoreConfirm, "confirm", false, "Confirm and execute restore (required)")
	restoreClusterCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, "Show what would be done without executing")
	restoreClusterCmd.Flags().BoolVar(&restoreForce, "force", false, "Skip safety checks and confirmations")
	restoreClusterCmd.Flags().IntVar(&restoreJobs, "jobs", 0, "Number of parallel decompression jobs (0 = auto)")
	restoreClusterCmd.Flags().BoolVar(&restoreVerbose, "verbose", false, "Show detailed restore progress")
	restoreClusterCmd.Flags().BoolVar(&restoreNoProgress, "no-progress", false, "Disable progress indicators")
}

// runRestoreSingle restores a single database
func runRestoreSingle(cmd *cobra.Command, args []string) error {
	archivePath := args[0]

	// Convert to absolute path
	if !filepath.IsAbs(archivePath) {
		absPath, err := filepath.Abs(archivePath)
		if err != nil {
			return fmt.Errorf("invalid archive path: %w", err)
		}
		archivePath = absPath
	}

	// Check if file exists
	if _, err := os.Stat(archivePath); err != nil {
		return fmt.Errorf("archive not found: %s", archivePath)
	}

	// Detect format
	format := restore.DetectArchiveFormat(archivePath)
	if format == restore.FormatUnknown {
		return fmt.Errorf("unknown archive format: %s", archivePath)
	}

	log.Info("Archive information",
		"file", filepath.Base(archivePath),
		"format", format.String(),
		"compressed", format.IsCompressed())

	// Extract database name from filename if target not specified
	targetDB := restoreTarget
	if targetDB == "" {
		targetDB = extractDBNameFromArchive(archivePath)
		if targetDB == "" {
			return fmt.Errorf("cannot determine database name, please specify --target")
		}
	} else {
		// If target was explicitly provided, also strip common file extensions
		// in case user included them in the target name
		targetDB = stripFileExtensions(targetDB)
	}

	// Safety checks
	safety := restore.NewSafety(cfg, log)

	if !restoreForce {
		log.Info("Validating archive...")
		if err := safety.ValidateArchive(archivePath); err != nil {
			return fmt.Errorf("archive validation failed: %w", err)
		}

		log.Info("Checking disk space...")
		multiplier := 3.0 // Assume 3x expansion for safety
		if err := safety.CheckDiskSpace(archivePath, multiplier); err != nil {
			return fmt.Errorf("disk space check failed: %w", err)
		}

		// Verify tools
		dbType := "postgres"
		if format.IsMySQL() {
			dbType = "mysql"
		}
		if err := safety.VerifyTools(dbType); err != nil {
			return fmt.Errorf("tool verification failed: %w", err)
		}
	}

	// Dry-run mode or confirmation required
	isDryRun := restoreDryRun || !restoreConfirm

	if isDryRun {
		fmt.Println("\n🔍 DRY-RUN MODE - No changes will be made")
		fmt.Printf("\nWould restore:\n")
		fmt.Printf("  Archive: %s\n", archivePath)
		fmt.Printf("  Format: %s\n", format.String())
		fmt.Printf("  Target Database: %s\n", targetDB)
		fmt.Printf("  Clean Before Restore: %v\n", restoreClean)
		fmt.Printf("  Create If Missing: %v\n", restoreCreate)
		fmt.Println("\nTo execute this restore, add --confirm flag")
		return nil
	}

	// Create database instance
	db, err := database.New(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create database instance: %w", err)
	}
	defer db.Close()

	// Create restore engine
	engine := restore.New(cfg, log, db)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan) // Ensure signal cleanup on exit
	
	go func() {
		<-sigChan
		log.Warn("Restore interrupted by user")
		cancel()
	}()

	// Execute restore
	log.Info("Starting restore...", "database", targetDB)

	if err := engine.RestoreSingle(ctx, archivePath, targetDB, restoreClean, restoreCreate); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	log.Info("✅ Restore completed successfully", "database", targetDB)
	return nil
}

// runRestoreCluster restores a full cluster
func runRestoreCluster(cmd *cobra.Command, args []string) error {
	archivePath := args[0]

	// Convert to absolute path
	if !filepath.IsAbs(archivePath) {
		absPath, err := filepath.Abs(archivePath)
		if err != nil {
			return fmt.Errorf("invalid archive path: %w", err)
		}
		archivePath = absPath
	}

	// Check if file exists
	if _, err := os.Stat(archivePath); err != nil {
		return fmt.Errorf("archive not found: %s", archivePath)
	}

	// Verify it's a cluster backup
	format := restore.DetectArchiveFormat(archivePath)
	if !format.IsClusterBackup() {
		return fmt.Errorf("not a cluster backup: %s (format: %s)", archivePath, format.String())
	}

	log.Info("Cluster archive information",
		"file", filepath.Base(archivePath),
		"format", format.String())

	// Safety checks
	safety := restore.NewSafety(cfg, log)

	if !restoreForce {
		log.Info("Validating archive...")
		if err := safety.ValidateArchive(archivePath); err != nil {
			return fmt.Errorf("archive validation failed: %w", err)
		}

		log.Info("Checking disk space...")
		multiplier := 4.0 // Cluster needs more space for extraction
		if err := safety.CheckDiskSpace(archivePath, multiplier); err != nil {
			return fmt.Errorf("disk space check failed: %w", err)
		}

		// Verify tools (assume PostgreSQL for cluster backups)
		if err := safety.VerifyTools("postgres"); err != nil {
			return fmt.Errorf("tool verification failed: %w", err)
		}
	}

	// Dry-run mode or confirmation required
	isDryRun := restoreDryRun || !restoreConfirm

	if isDryRun {
		fmt.Println("\n🔍 DRY-RUN MODE - No changes will be made")
		fmt.Printf("\nWould restore cluster:\n")
		fmt.Printf("  Archive: %s\n", archivePath)
		fmt.Printf("  Parallel Jobs: %d (0 = auto)\n", restoreJobs)
		fmt.Println("\nTo execute this restore, add --confirm flag")
		return nil
	}

	// Create database instance
	db, err := database.New(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create database instance: %w", err)
	}
	defer db.Close()

	// Create restore engine
	engine := restore.New(cfg, log, db)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan) // Ensure signal cleanup on exit
	
	go func() {
		<-sigChan
		log.Warn("Restore interrupted by user")
		cancel()
	}()

	// Execute cluster restore
	log.Info("Starting cluster restore...")

	if err := engine.RestoreCluster(ctx, archivePath); err != nil {
		return fmt.Errorf("cluster restore failed: %w", err)
	}

	log.Info("✅ Cluster restore completed successfully")
	return nil
}

// runRestoreList lists available backup archives
func runRestoreList(cmd *cobra.Command, args []string) error {
	backupDir := cfg.BackupDir

	// Check if backup directory exists
	if _, err := os.Stat(backupDir); err != nil {
		return fmt.Errorf("backup directory not found: %s", backupDir)
	}

	// List all backup files
	files, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("cannot read backup directory: %w", err)
	}

	var archives []archiveInfo

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		format := restore.DetectArchiveFormat(name)

		if format == restore.FormatUnknown {
			continue // Skip non-backup files
		}

		info, _ := file.Info()
		archives = append(archives, archiveInfo{
			Name:     name,
			Format:   format,
			Size:     info.Size(),
			Modified: info.ModTime(),
			DBName:   extractDBNameFromArchive(name),
		})
	}

	if len(archives) == 0 {
		fmt.Println("No backup archives found in:", backupDir)
		return nil
	}

	// Print header
	fmt.Printf("\n📦 Available backup archives in %s\n\n", backupDir)
	fmt.Printf("%-40s %-25s %-12s %-20s %s\n",
		"FILENAME", "FORMAT", "SIZE", "MODIFIED", "DATABASE")
	fmt.Println(strings.Repeat("-", 120))

	// Print archives
	for _, archive := range archives {
		fmt.Printf("%-40s %-25s %-12s %-20s %s\n",
			truncate(archive.Name, 40),
			truncate(archive.Format.String(), 25),
			formatSize(archive.Size),
			archive.Modified.Format("2006-01-02 15:04:05"),
			archive.DBName)
	}

	fmt.Printf("\nTotal: %d archive(s)\n", len(archives))
	fmt.Println("\nTo restore: dbbackup restore single <filename> --confirm")
	fmt.Println("           dbbackup restore cluster <filename> --confirm")

	return nil
}

// archiveInfo holds information about a backup archive
type archiveInfo struct {
	Name     string
	Format   restore.ArchiveFormat
	Size     int64
	Modified time.Time
	DBName   string
}

// stripFileExtensions removes common backup file extensions from a name
func stripFileExtensions(name string) string {
	// Remove extensions (handle double extensions like .sql.gz.sql.gz)
	for {
		oldName := name
		name = strings.TrimSuffix(name, ".tar.gz")
		name = strings.TrimSuffix(name, ".dump.gz")
		name = strings.TrimSuffix(name, ".sql.gz")
		name = strings.TrimSuffix(name, ".dump")
		name = strings.TrimSuffix(name, ".sql")
		// If no change, we're done
		if name == oldName {
			break
		}
	}
	return name
}

// extractDBNameFromArchive extracts database name from archive filename
func extractDBNameFromArchive(filename string) string {
	base := filepath.Base(filename)

	// Remove extensions
	base = stripFileExtensions(base)

	// Remove timestamp patterns (YYYYMMDD_HHMMSS)
	parts := strings.Split(base, "_")
	for i := len(parts) - 1; i >= 0; i-- {
		// Check if part looks like a date
		if len(parts[i]) == 8 || len(parts[i]) == 6 {
			// Could be date or time, remove it
			parts = parts[:i]
		} else {
			break
		}
	}

	if len(parts) > 0 {
		return parts[0]
	}

	return base
}

// formatSize formats file size
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// truncate truncates string to max length
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
