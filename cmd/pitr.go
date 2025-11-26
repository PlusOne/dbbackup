package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"dbbackup/internal/wal"
)

var (
	// PITR enable flags
	pitrArchiveDir string
	pitrForce      bool

	// WAL archive flags
	walArchiveDir string
	walCompress   bool
	walEncrypt    bool

	// PITR restore flags
	pitrTargetTime      string
	pitrTargetXID       string
	pitrTargetName      string
	pitrTargetLSN       string
	pitrTargetImmediate bool
	pitrRecoveryAction  string
	pitrWALSource       string
)

// pitrCmd represents the pitr command group
var pitrCmd = &cobra.Command{
	Use:   "pitr",
	Short: "Point-in-Time Recovery (PITR) operations",
	Long: `Manage PostgreSQL Point-in-Time Recovery (PITR) with WAL archiving.

PITR allows you to restore your database to any point in time, not just
to the time of your last backup. This requires continuous WAL archiving.

Commands:
  enable   - Configure PostgreSQL for PITR
  disable  - Disable PITR
  status   - Show current PITR configuration
`,
}

// pitrEnableCmd enables PITR
var pitrEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable Point-in-Time Recovery",
	Long: `Configure PostgreSQL for Point-in-Time Recovery by enabling WAL archiving.

This command will:
1. Create WAL archive directory
2. Update postgresql.conf with PITR settings
3. Set archive_mode = on
4. Configure archive_command to use dbbackup

Note: PostgreSQL restart is required after enabling PITR.

Example:
  dbbackup pitr enable --archive-dir /backups/wal_archive
`,
	RunE: runPITREnable,
}

// pitrDisableCmd disables PITR
var pitrDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable Point-in-Time Recovery",
	Long: `Disable PITR by turning off WAL archiving.

This sets archive_mode = off in postgresql.conf.
Requires PostgreSQL restart to take effect.

Example:
  dbbackup pitr disable
`,
	RunE: runPITRDisable,
}

// pitrStatusCmd shows PITR status
var pitrStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show PITR configuration and WAL archive status",
	Long: `Display current PITR settings and WAL archive statistics.

Shows:
- archive_mode, wal_level, archive_command
- Number of archived WAL files
- Total archive size
- Oldest and newest WAL archives

Example:
  dbbackup pitr status
`,
	RunE: runPITRStatus,
}

// walCmd represents the wal command group
var walCmd = &cobra.Command{
	Use:   "wal",
	Short: "WAL (Write-Ahead Log) operations",
	Long: `Manage PostgreSQL Write-Ahead Log (WAL) files.

WAL files contain all changes made to the database and are essential
for Point-in-Time Recovery (PITR).
`,
}

// walArchiveCmd archives a WAL file
var walArchiveCmd = &cobra.Command{
	Use:   "archive <wal_path> <wal_filename>",
	Short: "Archive a WAL file (called by PostgreSQL)",
	Long: `Archive a PostgreSQL WAL file to the archive directory.

This command is typically called automatically by PostgreSQL via the
archive_command setting. It can also be run manually for testing.

Arguments:
  wal_path     - Full path to the WAL file (e.g., /var/lib/postgresql/data/pg_wal/0000...)
  wal_filename - WAL filename only (e.g., 000000010000000000000001)

Example:
  dbbackup wal archive /var/lib/postgresql/data/pg_wal/000000010000000000000001 000000010000000000000001 --archive-dir /backups/wal
`,
	Args: cobra.ExactArgs(2),
	RunE: runWALArchive,
}

// walListCmd lists archived WAL files
var walListCmd = &cobra.Command{
	Use:   "list",
	Short: "List archived WAL files",
	Long: `List all WAL files in the archive directory.

Shows timeline, segment number, size, and archive time for each WAL file.

Example:
  dbbackup wal list --archive-dir /backups/wal_archive
`,
	RunE: runWALList,
}

// walCleanupCmd cleans up old WAL archives
var walCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove old WAL archives based on retention policy",
	Long: `Delete WAL archives older than the specified retention period.

WAL files older than --retention-days will be permanently deleted.

Example:
  dbbackup wal cleanup --archive-dir /backups/wal_archive --retention-days 7
`,
	RunE: runWALCleanup,
}

func init() {
	rootCmd.AddCommand(pitrCmd)
	rootCmd.AddCommand(walCmd)

	// PITR subcommands
	pitrCmd.AddCommand(pitrEnableCmd)
	pitrCmd.AddCommand(pitrDisableCmd)
	pitrCmd.AddCommand(pitrStatusCmd)

	// WAL subcommands
	walCmd.AddCommand(walArchiveCmd)
	walCmd.AddCommand(walListCmd)
	walCmd.AddCommand(walCleanupCmd)

	// PITR enable flags
	pitrEnableCmd.Flags().StringVar(&pitrArchiveDir, "archive-dir", "/var/backups/wal_archive", "Directory to store WAL archives")
	pitrEnableCmd.Flags().BoolVar(&pitrForce, "force", false, "Overwrite existing PITR configuration")

	// WAL archive flags
	walArchiveCmd.Flags().StringVar(&walArchiveDir, "archive-dir", "", "WAL archive directory (required)")
	walArchiveCmd.Flags().BoolVar(&walCompress, "compress", false, "Compress WAL files with gzip")
	walArchiveCmd.Flags().BoolVar(&walEncrypt, "encrypt", false, "Encrypt WAL files")
	walArchiveCmd.MarkFlagRequired("archive-dir")

	// WAL list flags
	walListCmd.Flags().StringVar(&walArchiveDir, "archive-dir", "/var/backups/wal_archive", "WAL archive directory")

	// WAL cleanup flags
	walCleanupCmd.Flags().StringVar(&walArchiveDir, "archive-dir", "/var/backups/wal_archive", "WAL archive directory")
	walCleanupCmd.Flags().IntVar(&cfg.RetentionDays, "retention-days", 7, "Days to keep WAL archives")
}

// Command implementations

func runPITREnable(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	if !cfg.IsPostgreSQL() {
		return fmt.Errorf("PITR is only supported for PostgreSQL (detected: %s)", cfg.DisplayDatabaseType())
	}

	log.Info("Enabling Point-in-Time Recovery (PITR)", "archive_dir", pitrArchiveDir)

	pitrManager := wal.NewPITRManager(cfg, log)
	if err := pitrManager.EnablePITR(ctx, pitrArchiveDir); err != nil {
		return fmt.Errorf("failed to enable PITR: %w", err)
	}

	log.Info("✅ PITR enabled successfully!")
	log.Info("")
	log.Info("Next steps:")
	log.Info("1. Restart PostgreSQL: sudo systemctl restart postgresql")
	log.Info("2. Create a base backup: dbbackup backup single <database>")
	log.Info("3. WAL files will be automatically archived to: " + pitrArchiveDir)
	log.Info("")
	log.Info("To restore to a point in time, use:")
	log.Info("  dbbackup restore pitr <backup> --target-time '2024-01-15 14:30:00'")

	return nil
}

func runPITRDisable(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	if !cfg.IsPostgreSQL() {
		return fmt.Errorf("PITR is only supported for PostgreSQL")
	}

	log.Info("Disabling Point-in-Time Recovery (PITR)")

	pitrManager := wal.NewPITRManager(cfg, log)
	if err := pitrManager.DisablePITR(ctx); err != nil {
		return fmt.Errorf("failed to disable PITR: %w", err)
	}

	log.Info("✅ PITR disabled successfully!")
	log.Info("PostgreSQL restart required: sudo systemctl restart postgresql")

	return nil
}

func runPITRStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	if !cfg.IsPostgreSQL() {
		return fmt.Errorf("PITR is only supported for PostgreSQL")
	}

	pitrManager := wal.NewPITRManager(cfg, log)
	config, err := pitrManager.GetCurrentPITRConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get PITR configuration: %w", err)
	}

	// Display PITR configuration
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  Point-in-Time Recovery (PITR) Status")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	if config.Enabled {
		fmt.Println("Status:          ✅ ENABLED")
	} else {
		fmt.Println("Status:          ❌ DISABLED")
	}

	fmt.Printf("WAL Level:       %s\n", config.WALLevel)
	fmt.Printf("Archive Mode:    %s\n", config.ArchiveMode)
	fmt.Printf("Archive Command: %s\n", config.ArchiveCommand)
	
	if config.MaxWALSenders > 0 {
		fmt.Printf("Max WAL Senders: %d\n", config.MaxWALSenders)
	}
	if config.WALKeepSize != "" {
		fmt.Printf("WAL Keep Size:   %s\n", config.WALKeepSize)
	}

	// Show WAL archive statistics if archive directory can be determined
	if config.ArchiveCommand != "" {
		// Extract archive dir from command (simple parsing)
		fmt.Println()
		fmt.Println("WAL Archive Statistics:")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		// TODO: Parse archive dir and show stats
		fmt.Println("  (Use 'dbbackup wal list --archive-dir <dir>' to view archives)")
	}

	return nil
}

func runWALArchive(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	walPath := args[0]
	walFilename := args[1]

	archiver := wal.NewArchiver(cfg, log)
	archiveConfig := wal.ArchiveConfig{
		ArchiveDir:  walArchiveDir,
		CompressWAL: walCompress,
		EncryptWAL:  walEncrypt,
	}

	info, err := archiver.ArchiveWALFile(ctx, walPath, walFilename, archiveConfig)
	if err != nil {
		return fmt.Errorf("WAL archiving failed: %w", err)
	}

	log.Info("WAL file archived successfully",
		"wal", info.WALFileName,
		"archive", info.ArchivePath,
		"original_size", info.OriginalSize,
		"archived_size", info.ArchivedSize,
		"timeline", info.Timeline,
		"segment", info.Segment)

	return nil
}

func runWALList(cmd *cobra.Command, args []string) error {
	archiver := wal.NewArchiver(cfg, log)
	archiveConfig := wal.ArchiveConfig{
		ArchiveDir: walArchiveDir,
	}

	archives, err := archiver.ListArchivedWALFiles(archiveConfig)
	if err != nil {
		return fmt.Errorf("failed to list WAL archives: %w", err)
	}

	if len(archives) == 0 {
		fmt.Println("No WAL archives found in: " + walArchiveDir)
		return nil
	}

	// Display archives
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  WAL Archives (%d files)\n", len(archives))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	fmt.Printf("%-28s  %10s  %10s  %8s  %s\n", "WAL Filename", "Timeline", "Segment", "Size", "Archived At")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────")

	for _, archive := range archives {
		size := formatWALSize(archive.ArchivedSize)
		timeStr := archive.ArchivedAt.Format("2006-01-02 15:04")
		
		flags := ""
		if archive.Compressed {
			flags += "C"
		}
		if archive.Encrypted {
			flags += "E"
		}
		if flags != "" {
			flags = " [" + flags + "]"
		}

		fmt.Printf("%-28s  %10d  0x%08X  %8s  %s%s\n",
			archive.WALFileName,
			archive.Timeline,
			archive.Segment,
			size,
			timeStr,
			flags)
	}

	// Show statistics
	stats, _ := archiver.GetArchiveStats(archiveConfig)
	if stats != nil {
		fmt.Println()
		fmt.Printf("Total Size: %s\n", stats.FormatSize())
		if stats.CompressedFiles > 0 {
			fmt.Printf("Compressed: %d files\n", stats.CompressedFiles)
		}
		if stats.EncryptedFiles > 0 {
			fmt.Printf("Encrypted:  %d files\n", stats.EncryptedFiles)
		}
		if !stats.OldestArchive.IsZero() {
			fmt.Printf("Oldest:     %s\n", stats.OldestArchive.Format("2006-01-02 15:04"))
			fmt.Printf("Newest:     %s\n", stats.NewestArchive.Format("2006-01-02 15:04"))
		}
	}

	return nil
}

func runWALCleanup(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	archiver := wal.NewArchiver(cfg, log)
	archiveConfig := wal.ArchiveConfig{
		ArchiveDir:    walArchiveDir,
		RetentionDays: cfg.RetentionDays,
	}

	if archiveConfig.RetentionDays <= 0 {
		return fmt.Errorf("--retention-days must be greater than 0")
	}

	deleted, err := archiver.CleanupOldWALFiles(ctx, archiveConfig)
	if err != nil {
		return fmt.Errorf("WAL cleanup failed: %w", err)
	}

	log.Info("✅ WAL cleanup completed", "deleted", deleted, "retention_days", archiveConfig.RetentionDays)
	return nil
}

// Helper functions

func formatWALSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
	)

	if bytes >= MB {
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	}
	return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
}
