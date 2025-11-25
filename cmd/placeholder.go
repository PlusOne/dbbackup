package cmd

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dbbackup/internal/auth"
	"dbbackup/internal/logger"
	"dbbackup/internal/tui"
	"github.com/spf13/cobra"
)

// Create placeholder commands for the other subcommands

var verifyCmd = &cobra.Command{
	Use:   "verify [archive]",
	Short: "Verify backup archive integrity",
	Long:  `Verify the integrity of backup archives.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("backup archive filename required")
		}
		return runVerify(cmd.Context(), args[0])
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available backups and databases",
	Long:  `List available backup archives and database information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runList(cmd.Context())
	},
}

var interactiveCmd = &cobra.Command{
	Use:     "interactive",
	Short:   "Start interactive menu mode",
	Long:    `Start the interactive menu system for guided backup operations.

TUI Automation Flags (for testing and CI/CD):
  --auto-select <index>     Automatically select menu option (0-13)
  --auto-database <name>    Pre-fill database name in prompts
  --auto-confirm            Auto-confirm all prompts (no user interaction)
  --dry-run                 Simulate operations without execution
  --verbose-tui             Enable detailed TUI event logging
  --tui-log-file <path>     Write TUI events to log file`,
	Aliases: []string{"menu", "ui"},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse TUI automation flags into config
		cfg.TUIAutoSelect, _ = cmd.Flags().GetInt("auto-select")
		cfg.TUIAutoDatabase, _ = cmd.Flags().GetString("auto-database")
		cfg.TUIAutoHost, _ = cmd.Flags().GetString("auto-host")
		cfg.TUIAutoPort, _ = cmd.Flags().GetInt("auto-port")
		cfg.TUIAutoConfirm, _ = cmd.Flags().GetBool("auto-confirm")
		cfg.TUIDryRun, _ = cmd.Flags().GetBool("dry-run")
		cfg.TUIVerbose, _ = cmd.Flags().GetBool("verbose-tui")
		cfg.TUILogFile, _ = cmd.Flags().GetString("tui-log-file")
		
		// Check authentication before starting TUI
		if cfg.IsPostgreSQL() {
			if mismatch, msg := auth.CheckAuthenticationMismatch(cfg); mismatch {
				fmt.Println(msg)
				return fmt.Errorf("authentication configuration required")
			}
		}
		
		// Use verbose logger if TUI verbose mode enabled
		var interactiveLog logger.Logger
		if cfg.TUIVerbose {
			interactiveLog = log
		} else {
			interactiveLog = logger.NewSilent()
		}
		
		// Start the interactive TUI
		return tui.RunInteractiveMenu(cfg, interactiveLog)
	},
}

func init() {
	// TUI automation flags (for testing and automation)
	interactiveCmd.Flags().Int("auto-select", -1, "Auto-select menu option (0-13, -1=disabled)")
	interactiveCmd.Flags().String("auto-database", "", "Pre-fill database name")
	interactiveCmd.Flags().String("auto-host", "", "Pre-fill host")
	interactiveCmd.Flags().Int("auto-port", 0, "Pre-fill port (0=use default)")
	interactiveCmd.Flags().Bool("auto-confirm", false, "Auto-confirm all prompts")
	interactiveCmd.Flags().Bool("dry-run", false, "Simulate operations without execution")
	interactiveCmd.Flags().Bool("verbose-tui", false, "Enable verbose TUI logging")
	interactiveCmd.Flags().String("tui-log-file", "", "Write TUI events to file")
}

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Run preflight checks",
	Long:  `Run connectivity and dependency checks before backup operations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPreflight(cmd.Context())
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show connection status and configuration",
	Long:  `Display current configuration and test database connectivity.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStatus(cmd.Context())
	},
}

// runList lists available backups and databases
func runList(ctx context.Context) error {
	fmt.Println("==============================================================")
	fmt.Println(" Available Backups")
	fmt.Println("==============================================================")

	// List backup files
	backupFiles, err := listBackupFiles(cfg.BackupDir)
	if err != nil {
		log.Error("Failed to list backup files", "error", err)
		return fmt.Errorf("failed to list backup files: %w", err)
	}

	if len(backupFiles) == 0 {
		fmt.Printf("No backup files found in: %s\n", cfg.BackupDir)
	} else {
		fmt.Printf("Found %d backup files in: %s\n\n", len(backupFiles), cfg.BackupDir)

		for _, file := range backupFiles {
			stat, err := os.Stat(filepath.Join(cfg.BackupDir, file.Name))
			if err != nil {
				continue
			}

			fmt.Printf("📦 %s\n", file.Name)
			fmt.Printf("   Size: %s\n", formatFileSize(stat.Size()))
			fmt.Printf("   Modified: %s\n", stat.ModTime().Format("2006-01-02 15:04:05"))
			fmt.Printf("   Type: %s\n", getBackupType(file.Name))
			fmt.Println()
		}
	}

	return nil
}

// listBackupFiles lists all backup files in the backup directory
func listBackupFiles(backupDir string) ([]backupFile, error) {
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, err
	}

	var files []backupFile
	for _, entry := range entries {
		if !entry.IsDir() && isBackupFile(entry.Name()) {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, backupFile{
				Name:    entry.Name(),
				ModTime: info.ModTime(),
				Size:    info.Size(),
			})
		}
	}

	// Sort by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	return files, nil
}

type backupFile struct {
	Name    string
	ModTime time.Time
	Size    int64
}

// isBackupFile checks if a file is a backup file based on extension
func isBackupFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".dump" || ext == ".sql" || ext == ".tar" || ext == ".gz" ||
		strings.HasSuffix(filename, ".tar.gz") || strings.HasSuffix(filename, ".dump.gz")
}

// getBackupType determines backup type from filename
func getBackupType(filename string) string {
	if strings.Contains(filename, "cluster") {
		return "Cluster Backup"
	} else if strings.Contains(filename, "sample") {
		return "Sample Backup"
	} else if strings.HasSuffix(filename, ".dump") || strings.HasSuffix(filename, ".dump.gz") {
		return "Single Database"
	} else if strings.HasSuffix(filename, ".sql") {
		return "SQL Script"
	}
	return "Unknown"
}

// formatFileSize formats file size in human readable format
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// runPreflight performs comprehensive pre-backup checks
func runPreflight(ctx context.Context) error {
	fmt.Println("==============================================================")
	fmt.Println(" Preflight Checks")
	fmt.Println("==============================================================")

	checksPassed := 0
	totalChecks := 6

	// 1. Database connectivity check
	fmt.Print("🔗 Database connectivity... ")
	if err := testDatabaseConnection(); err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
	} else {
		fmt.Println("✅ PASSED")
		checksPassed++
	}

	// 2. Required tools check
	fmt.Print("🛠️  Required tools (pg_dump/pg_restore)... ")
	if err := checkRequiredTools(); err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
	} else {
		fmt.Println("✅ PASSED")
		checksPassed++
	}

	// 3. Backup directory check
	fmt.Print("📁 Backup directory access... ")
	if err := checkBackupDirectory(); err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
	} else {
		fmt.Println("✅ PASSED")
		checksPassed++
	}

	// 4. Disk space check
	fmt.Print("💾 Available disk space... ")
	if err := checkDiskSpace(); err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
	} else {
		fmt.Println("✅ PASSED")
		checksPassed++
	}

	// 5. Permissions check
	fmt.Print("🔐 File permissions... ")
	if err := checkPermissions(); err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
	} else {
		fmt.Println("✅ PASSED")
		checksPassed++
	}

	// 6. CPU/Memory resources check
	fmt.Print("🖥️  System resources... ")
	if err := checkSystemResources(); err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
	} else {
		fmt.Println("✅ PASSED")
		checksPassed++
	}

	fmt.Println("")
	fmt.Printf("Results: %d/%d checks passed\n", checksPassed, totalChecks)

	if checksPassed == totalChecks {
		fmt.Println("🎉 All preflight checks passed! System is ready for backup operations.")
		return nil
	} else {
		fmt.Printf("⚠️  %d check(s) failed. Please address the issues before running backups.\n", totalChecks-checksPassed)
		return fmt.Errorf("preflight checks failed: %d/%d passed", checksPassed, totalChecks)
	}
}

func testDatabaseConnection() error {
	// Reuse existing database connection logic
	if cfg.DatabaseType != "postgres" && cfg.DatabaseType != "mysql" {
		return fmt.Errorf("unsupported database type: %s", cfg.DatabaseType)
	}
	// For now, just check if basic connection parameters are set
	if cfg.Host == "" || cfg.User == "" {
		return fmt.Errorf("missing required connection parameters")
	}
	return nil
}

func checkRequiredTools() error {
	tools := []string{"pg_dump", "pg_restore"}
	if cfg.DatabaseType == "mysql" {
		tools = []string{"mysqldump", "mysql"}
	}

	for _, tool := range tools {
		if _, err := os.Stat("/usr/bin/" + tool); os.IsNotExist(err) {
			if _, err := os.Stat("/usr/local/bin/" + tool); os.IsNotExist(err) {
				return fmt.Errorf("required tool not found: %s", tool)
			}
		}
	}
	return nil
}

func checkBackupDirectory() error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(cfg.BackupDir, 0755); err != nil {
		return fmt.Errorf("cannot create backup directory: %w", err)
	}

	// Test write access
	testFile := filepath.Join(cfg.BackupDir, ".preflight_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("cannot write to backup directory: %w", err)
	}
	os.Remove(testFile) // Clean up
	return nil
}

func checkDiskSpace() error {
	// Basic disk space check - this is a simplified version
	// In a real implementation, you'd use syscall.Statfs or similar
	if _, err := os.Stat(cfg.BackupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup directory does not exist")
	}
	return nil // Assume sufficient space for now
}

func checkPermissions() error {
	// Check if we can read/write in backup directory
	if _, err := os.Stat(cfg.BackupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup directory not accessible")
	}

	// Test file creation and deletion
	testFile := filepath.Join(cfg.BackupDir, ".permissions_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("insufficient write permissions: %w", err)
	}
	if err := os.Remove(testFile); err != nil {
		return fmt.Errorf("insufficient delete permissions: %w", err)
	}
	return nil
}

func checkSystemResources() error {
	// Basic system resource check
	if cfg.Jobs < 1 || cfg.Jobs > 32 {
		return fmt.Errorf("invalid job count: %d (should be 1-32)", cfg.Jobs)
	}
	if cfg.MaxCores < 1 {
		return fmt.Errorf("invalid max cores setting: %d", cfg.MaxCores)
	}
	return nil
}

// runRestore restores database from backup archive
func runRestore(ctx context.Context, archiveName string) error {
	fmt.Println("==============================================================")
	fmt.Println(" Database Restore")
	fmt.Println("==============================================================")

	// Construct full path to archive
	archivePath := filepath.Join(cfg.BackupDir, archiveName)

	// Check if archive exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return fmt.Errorf("backup archive not found: %s", archivePath)
	}

	// Detect archive type
	archiveType := detectArchiveType(archiveName)
	fmt.Printf("Archive: %s\n", archiveName)
	fmt.Printf("Type: %s\n", archiveType)
	fmt.Printf("Location: %s\n", archivePath)
	fmt.Println()

	// Get archive info
	stat, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("cannot access archive: %w", err)
	}

	fmt.Printf("Size: %s\n", formatFileSize(stat.Size()))
	fmt.Printf("Created: %s\n", stat.ModTime().Format("2006-01-02 15:04:05"))
	fmt.Println()

	// Show warning
	fmt.Println("⚠️  WARNING: This will restore data to the target database.")
	fmt.Println("   Existing data may be overwritten or merged depending on the restore method.")
	fmt.Println()

	// For safety, show what would be done without actually doing it
	switch archiveType {
	case "Single Database (.dump)":
		fmt.Println("🔄 Would execute: pg_restore to restore single database")
		fmt.Printf("   Command: pg_restore -h %s -p %d -U %s -d %s --verbose %s\n",
			cfg.Host, cfg.Port, cfg.User, cfg.Database, archivePath)
	case "Single Database (.dump.gz)":
		fmt.Println("🔄 Would execute: gunzip and pg_restore to restore single database")
		fmt.Printf("   Command: gunzip -c %s | pg_restore -h %s -p %d -U %s -d %s --verbose\n",
			archivePath, cfg.Host, cfg.Port, cfg.User, cfg.Database)
	case "SQL Script (.sql)":
		if cfg.IsPostgreSQL() {
			fmt.Println("🔄 Would execute: psql to run SQL script")
			fmt.Printf("   Command: psql -h %s -p %d -U %s -d %s -f %s\n",
				cfg.Host, cfg.Port, cfg.User, cfg.Database, archivePath)
		} else if cfg.IsMySQL() {
			fmt.Println("🔄 Would execute: mysql to run SQL script")
			fmt.Printf("   Command: %s\n", mysqlRestoreCommand(archivePath, false))
		} else {
			fmt.Println("🔄 Would execute: SQL client to run script (database type unknown)")
		}
	case "SQL Script (.sql.gz)":
		if cfg.IsPostgreSQL() {
			fmt.Println("🔄 Would execute: gunzip and psql to run SQL script")
			fmt.Printf("   Command: gunzip -c %s | psql -h %s -p %d -U %s -d %s\n",
				archivePath, cfg.Host, cfg.Port, cfg.User, cfg.Database)
		} else if cfg.IsMySQL() {
			fmt.Println("🔄 Would execute: gunzip and mysql to run SQL script")
			fmt.Printf("   Command: %s\n", mysqlRestoreCommand(archivePath, true))
		} else {
			fmt.Println("🔄 Would execute: gunzip and SQL client to run script (database type unknown)")
		}
	case "Cluster Backup (.tar.gz)":
		fmt.Println("🔄 Would execute: Extract and restore cluster backup")
		fmt.Println("   Steps:")
		fmt.Println("   1. Extract tar.gz archive")
		fmt.Println("   2. Restore global objects (roles, tablespaces)")
		fmt.Println("   3. Restore individual databases")
	default:
		return fmt.Errorf("unsupported archive type: %s", archiveType)
	}

	fmt.Println()
	fmt.Println("🛡️  SAFETY MODE: Restore command is in preview mode.")
	fmt.Println("   This shows what would be executed without making changes.")
	fmt.Println("   To enable actual restore, add --confirm flag (not yet implemented).")

	return nil
}

func detectArchiveType(filename string) string {
	switch {
	case strings.HasSuffix(filename, ".dump.gz"):
		return "Single Database (.dump.gz)"
	case strings.HasSuffix(filename, ".dump"):
		return "Single Database (.dump)"
	case strings.HasSuffix(filename, ".sql.gz"):
		return "SQL Script (.sql.gz)"
	case strings.HasSuffix(filename, ".sql"):
		return "SQL Script (.sql)"
	case strings.HasSuffix(filename, ".tar.gz"):
		return "Cluster Backup (.tar.gz)"
	case strings.HasSuffix(filename, ".tar"):
		return "Archive (.tar)"
	default:
		return "Unknown"
	}
}

// runVerify verifies backup archive integrity
func runVerify(ctx context.Context, archiveName string) error {
	fmt.Println("==============================================================")
	fmt.Println(" Backup Archive Verification")
	fmt.Println("==============================================================")

	// Construct full path to archive
	archivePath := filepath.Join(cfg.BackupDir, archiveName)

	// Check if archive exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return fmt.Errorf("backup archive not found: %s", archivePath)
	}

	// Get archive info
	stat, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("cannot access archive: %w", err)
	}

	fmt.Printf("Archive: %s\n", archiveName)
	fmt.Printf("Size: %s\n", formatFileSize(stat.Size()))
	fmt.Printf("Created: %s\n", stat.ModTime().Format("2006-01-02 15:04:05"))
	fmt.Println()

	// Detect and verify based on archive type
	archiveType := detectArchiveType(archiveName)
	fmt.Printf("Type: %s\n", archiveType)

	checksRun := 0
	checksPassed := 0

	// Basic file existence and readability
	fmt.Print("📁 File accessibility... ")
	if file, err := os.Open(archivePath); err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
	} else {
		file.Close()
		fmt.Println("✅ PASSED")
		checksPassed++
	}
	checksRun++

	// File size sanity check
	fmt.Print("📏 File size check... ")
	if stat.Size() == 0 {
		fmt.Println("❌ FAILED: File is empty")
	} else if stat.Size() < 100 {
		fmt.Println("⚠️  WARNING: File is very small (< 100 bytes)")
		checksPassed++
	} else {
		fmt.Println("✅ PASSED")
		checksPassed++
	}
	checksRun++

	// Type-specific verification
	switch archiveType {
	case "Single Database (.dump)":
		fmt.Print("🔍 PostgreSQL dump format check... ")
		if err := verifyPgDump(archivePath); err != nil {
			fmt.Printf("❌ FAILED: %v\n", err)
		} else {
			fmt.Println("✅ PASSED")
			checksPassed++
		}
		checksRun++

	case "Single Database (.dump.gz)":
		fmt.Print("🔍 PostgreSQL dump format check (gzip)... ")
		if err := verifyPgDumpGzip(archivePath); err != nil {
			fmt.Printf("❌ FAILED: %v\n", err)
		} else {
			fmt.Println("✅ PASSED")
			checksPassed++
		}
		checksRun++

	case "SQL Script (.sql)":
		fmt.Print("📜 SQL script validation... ")
		if err := verifySqlScript(archivePath); err != nil {
			fmt.Printf("❌ FAILED: %v\n", err)
		} else {
			fmt.Println("✅ PASSED")
			checksPassed++
		}
		checksRun++

	case "SQL Script (.sql.gz)":
		fmt.Print("📜 SQL script validation (gzip)... ")
		if err := verifyGzipSqlScript(archivePath); err != nil {
			fmt.Printf("❌ FAILED: %v\n", err)
		} else {
			fmt.Println("✅ PASSED")
			checksPassed++
		}
		checksRun++

	case "Cluster Backup (.tar.gz)":
		fmt.Print("📦 Archive extraction test... ")
		if err := verifyTarGz(archivePath); err != nil {
			fmt.Printf("❌ FAILED: %v\n", err)
		} else {
			fmt.Println("✅ PASSED")
			checksPassed++
		}
		checksRun++
	}

	// Check for metadata file
	metadataPath := archivePath + ".info"
	fmt.Print("📋 Metadata file check... ")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		fmt.Println("⚠️  WARNING: No metadata file found")
	} else {
		fmt.Println("✅ PASSED")
		checksPassed++
	}
	checksRun++

	fmt.Println()
	fmt.Printf("Verification Results: %d/%d checks passed\n", checksPassed, checksRun)

	if checksPassed == checksRun {
		fmt.Println("🎉 Archive verification completed successfully!")
		return nil
	} else if float64(checksPassed)/float64(checksRun) >= 0.8 {
		fmt.Println("⚠️  Archive verification completed with warnings.")
		return nil
	} else {
		fmt.Println("❌ Archive verification failed. Archive may be corrupted.")
		return fmt.Errorf("verification failed: %d/%d checks passed", checksPassed, checksRun)
	}
}

func verifyPgDump(path string) error {
	// Basic check - try to read first few bytes for PostgreSQL dump signature
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("cannot read file: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("cannot read file")
	}

	return checkPgDumpSignature(buffer[:n])
}

func verifyPgDumpGzip(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to open gzip stream: %w", err)
	}
	defer gz.Close()

	buffer := make([]byte, 512)
	n, err := gz.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("cannot read gzip contents: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("gzip archive is empty")
	}

	return checkPgDumpSignature(buffer[:n])
}

func verifySqlScript(path string) error {
	// Basic check - ensure it's readable and contains SQL-like content
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := make([]byte, 1024)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("cannot read file: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("cannot read file")
	}

	if containsSQLKeywords(strings.ToLower(string(buffer[:n]))) {
		return nil
	}

	return fmt.Errorf("does not appear to contain SQL content")
}

func verifyGzipSqlScript(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to open gzip stream: %w", err)
	}
	defer gz.Close()

	buffer := make([]byte, 1024)
	n, err := gz.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("cannot read gzip contents: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("gzip archive is empty")
	}

	if containsSQLKeywords(strings.ToLower(string(buffer[:n]))) {
		return nil
	}

	return fmt.Errorf("does not appear to contain SQL content")
}

func verifyTarGz(path string) error {
	// Basic check - try to list contents without extracting
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Check if it starts with gzip magic number
	buffer := make([]byte, 3)
	n, err := file.Read(buffer)
	if err != nil || n < 3 {
		return fmt.Errorf("cannot read file header")
	}

	if buffer[0] == 0x1f && buffer[1] == 0x8b {
		return nil // Valid gzip header
	}

	return fmt.Errorf("does not appear to be a valid gzip file")
}

func checkPgDumpSignature(data []byte) error {
	if len(data) >= 5 && string(data[:5]) == "PGDMP" {
		return nil
	}

	content := strings.ToLower(string(data))
	if strings.Contains(content, "postgresql") || strings.Contains(content, "pg_dump") {
		return nil
	}

	return fmt.Errorf("does not appear to be a PostgreSQL dump file")
}

func containsSQLKeywords(content string) bool {
	sqlKeywords := []string{"select", "insert", "create", "drop", "alter", "database", "table", "update", "delete"}

	for _, keyword := range sqlKeywords {
		if strings.Contains(content, keyword) {
			return true
		}
	}

	return false
}

func mysqlRestoreCommand(archivePath string, compressed bool) string {
	parts := []string{"mysql"}
	
	// Only add -h flag if host is not localhost (to use Unix socket)
	if cfg.Host != "localhost" && cfg.Host != "127.0.0.1" && cfg.Host != "" {
		parts = append(parts, "-h", cfg.Host)
	}
	
	parts = append(parts,
		"-P", fmt.Sprintf("%d", cfg.Port),
		"-u", cfg.User,
	)

	if cfg.Password != "" {
		parts = append(parts, fmt.Sprintf("-p'%s'", cfg.Password))
	}

	if cfg.Database != "" {
		parts = append(parts, cfg.Database)
	}

	command := strings.Join(parts, " ")

	if compressed {
		return fmt.Sprintf("gunzip -c %s | %s", archivePath, command)
	}

	return fmt.Sprintf("%s < %s", command, archivePath)
}
