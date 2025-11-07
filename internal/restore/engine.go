package restore

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dbbackup/internal/config"
	"dbbackup/internal/database"
	"dbbackup/internal/logger"
	"dbbackup/internal/progress"
)

// Engine handles database restore operations
type Engine struct {
	cfg              *config.Config
	log              logger.Logger
	db               database.Database
	progress         progress.Indicator
	detailedReporter *progress.DetailedReporter
	dryRun           bool
}

// New creates a new restore engine
func New(cfg *config.Config, log logger.Logger, db database.Database) *Engine {
	progressIndicator := progress.NewIndicator(true, "line")
	detailedReporter := progress.NewDetailedReporter(progressIndicator, &loggerAdapter{logger: log})

	return &Engine{
		cfg:              cfg,
		log:              log,
		db:               db,
		progress:         progressIndicator,
		detailedReporter: detailedReporter,
		dryRun:           false,
	}
}

// NewWithProgress creates a restore engine with custom progress indicator
func NewWithProgress(cfg *config.Config, log logger.Logger, db database.Database, progressIndicator progress.Indicator, dryRun bool) *Engine {
	if progressIndicator == nil {
		progressIndicator = progress.NewNullIndicator()
	}

	detailedReporter := progress.NewDetailedReporter(progressIndicator, &loggerAdapter{logger: log})

	return &Engine{
		cfg:              cfg,
		log:              log,
		db:               db,
		progress:         progressIndicator,
		detailedReporter: detailedReporter,
		dryRun:           dryRun,
	}
}

// loggerAdapter adapts our logger to the progress.Logger interface
type loggerAdapter struct {
	logger logger.Logger
}

func (la *loggerAdapter) Info(msg string, args ...any) {
	la.logger.Info(msg, args...)
}

func (la *loggerAdapter) Warn(msg string, args ...any) {
	la.logger.Warn(msg, args...)
}

func (la *loggerAdapter) Error(msg string, args ...any) {
	la.logger.Error(msg, args...)
}

func (la *loggerAdapter) Debug(msg string, args ...any) {
	la.logger.Debug(msg, args...)
}

// RestoreSingle restores a single database from an archive
func (e *Engine) RestoreSingle(ctx context.Context, archivePath, targetDB string, cleanFirst, createIfMissing bool) error {
	operation := e.log.StartOperation("Single Database Restore")

	// Validate archive exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		operation.Fail("Archive not found")
		return fmt.Errorf("archive not found: %s", archivePath)
	}

	// Detect archive format
	format := DetectArchiveFormat(archivePath)
	e.log.Info("Detected archive format", "format", format, "path", archivePath)

	if e.dryRun {
		e.log.Info("DRY RUN: Would restore single database", "archive", archivePath, "target", targetDB)
		return e.previewRestore(archivePath, targetDB, format)
	}

	// Start progress tracking
	e.progress.Start(fmt.Sprintf("Restoring database '%s' from %s", targetDB, filepath.Base(archivePath)))

	// Handle different archive formats
	var err error
	switch format {
	case FormatPostgreSQLDump, FormatPostgreSQLDumpGz:
		err = e.restorePostgreSQLDump(ctx, archivePath, targetDB, format == FormatPostgreSQLDumpGz, cleanFirst)
	case FormatPostgreSQLSQL, FormatPostgreSQLSQLGz:
		err = e.restorePostgreSQLSQL(ctx, archivePath, targetDB, format == FormatPostgreSQLSQLGz)
	case FormatMySQLSQL, FormatMySQLSQLGz:
		err = e.restoreMySQLSQL(ctx, archivePath, targetDB, format == FormatMySQLSQLGz)
	default:
		operation.Fail("Unsupported archive format")
		return fmt.Errorf("unsupported archive format: %s", format)
	}

	if err != nil {
		e.progress.Fail(fmt.Sprintf("Restore failed: %v", err))
		operation.Fail(fmt.Sprintf("Restore failed: %v", err))
		return err
	}

	e.progress.Complete(fmt.Sprintf("Database '%s' restored successfully", targetDB))
	operation.Complete(fmt.Sprintf("Restored database '%s' from %s", targetDB, filepath.Base(archivePath)))
	return nil
}

// restorePostgreSQLDump restores from PostgreSQL custom dump format
func (e *Engine) restorePostgreSQLDump(ctx context.Context, archivePath, targetDB string, compressed bool, cleanFirst bool) error {
	// Build restore command
	opts := database.RestoreOptions{
		Parallel:          1,
		Clean:             cleanFirst,
		NoOwner:           true,
		NoPrivileges:      true,
		SingleTransaction: true,
	}
	
	cmd := e.db.BuildRestoreCommand(targetDB, archivePath, opts)

	if compressed {
		// For compressed dumps, decompress first
		return e.executeRestoreWithDecompression(ctx, archivePath, cmd)
	}

	return e.executeRestoreCommand(ctx, cmd)
}

// restorePostgreSQLSQL restores from PostgreSQL SQL script
func (e *Engine) restorePostgreSQLSQL(ctx context.Context, archivePath, targetDB string, compressed bool) error {
	// Use psql for SQL scripts
	var cmd []string
	if compressed {
		cmd = []string{
			"bash", "-c",
			fmt.Sprintf("gunzip -c %s | psql -h %s -p %d -U %s -d %s",
				archivePath, e.cfg.Host, e.cfg.Port, e.cfg.User, targetDB),
		}
	} else {
		cmd = []string{
			"psql",
			"-h", e.cfg.Host,
			"-p", fmt.Sprintf("%d", e.cfg.Port),
			"-U", e.cfg.User,
			"-d", targetDB,
			"-f", archivePath,
		}
	}

	return e.executeRestoreCommand(ctx, cmd)
}

// restoreMySQLSQL restores from MySQL SQL script
func (e *Engine) restoreMySQLSQL(ctx context.Context, archivePath, targetDB string, compressed bool) error {
	options := database.RestoreOptions{}

	cmd := e.db.BuildRestoreCommand(targetDB, archivePath, options)

	if compressed {
		// For compressed SQL, decompress on the fly
		cmd = []string{
			"bash", "-c",
			fmt.Sprintf("gunzip -c %s | %s", archivePath, strings.Join(cmd, " ")),
		}
	}

	return e.executeRestoreCommand(ctx, cmd)
}

// executeRestoreCommand executes a restore command
func (e *Engine) executeRestoreCommand(ctx context.Context, cmdArgs []string) error {
	e.log.Info("Executing restore command", "command", strings.Join(cmdArgs, " "))

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)

	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PGPASSWORD=%s", e.cfg.Password),
		fmt.Sprintf("MYSQL_PWD=%s", e.cfg.Password),
	)

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		e.log.Error("Restore command failed", "error", err, "output", string(output))
		return fmt.Errorf("restore failed: %w\nOutput: %s", err, string(output))
	}

	e.log.Info("Restore command completed successfully")
	return nil
}

// executeRestoreWithDecompression handles decompression during restore
func (e *Engine) executeRestoreWithDecompression(ctx context.Context, archivePath string, restoreCmd []string) error {
	// Check if pigz is available for faster decompression
	decompressCmd := "gunzip"
	if _, err := exec.LookPath("pigz"); err == nil {
		decompressCmd = "pigz"
		e.log.Info("Using pigz for parallel decompression")
	}

	// Build pipeline: decompress | restore
	pipeline := fmt.Sprintf("%s -dc %s | %s", decompressCmd, archivePath, strings.Join(restoreCmd, " "))
	cmd := exec.CommandContext(ctx, "bash", "-c", pipeline)

	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PGPASSWORD=%s", e.cfg.Password),
		fmt.Sprintf("MYSQL_PWD=%s", e.cfg.Password),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		e.log.Error("Restore with decompression failed", "error", err, "output", string(output))
		return fmt.Errorf("restore failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// previewRestore shows what would be done without executing
func (e *Engine) previewRestore(archivePath, targetDB string, format ArchiveFormat) error {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println(" RESTORE PREVIEW (DRY RUN)")
	fmt.Println(strings.Repeat("=", 60))

	stat, _ := os.Stat(archivePath)
	fmt.Printf("\nArchive: %s\n", filepath.Base(archivePath))
	fmt.Printf("Format: %s\n", format)
	if stat != nil {
		fmt.Printf("Size: %s\n", FormatBytes(stat.Size()))
		fmt.Printf("Modified: %s\n", stat.ModTime().Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("Target Database: %s\n", targetDB)
	fmt.Printf("Target Host: %s:%d\n", e.cfg.Host, e.cfg.Port)

	fmt.Println("\nOperations that would be performed:")
	switch format {
	case FormatPostgreSQLDump:
		fmt.Printf("  1. Execute: pg_restore -d %s %s\n", targetDB, archivePath)
	case FormatPostgreSQLDumpGz:
		fmt.Printf("  1. Decompress: %s\n", archivePath)
		fmt.Printf("  2. Execute: pg_restore -d %s\n", targetDB)
	case FormatPostgreSQLSQL, FormatPostgreSQLSQLGz:
		fmt.Printf("  1. Execute: psql -d %s -f %s\n", targetDB, archivePath)
	case FormatMySQLSQL, FormatMySQLSQLGz:
		fmt.Printf("  1. Execute: mysql %s < %s\n", targetDB, archivePath)
	}

	fmt.Println("\n⚠️  WARNING: This will restore data to the target database.")
	fmt.Println("   Existing data may be overwritten or merged.")
	fmt.Println("\nTo execute this restore, add the --confirm flag.")
	fmt.Println(strings.Repeat("=", 60) + "\n")

	return nil
}

// RestoreCluster restores a full cluster from a tar.gz archive
func (e *Engine) RestoreCluster(ctx context.Context, archivePath string) error {
	operation := e.log.StartOperation("Cluster Restore")

	// Validate archive
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		operation.Fail("Archive not found")
		return fmt.Errorf("archive not found: %s", archivePath)
	}

	format := DetectArchiveFormat(archivePath)
	if format != FormatClusterTarGz {
		operation.Fail("Invalid cluster archive format")
		return fmt.Errorf("not a cluster archive: %s (detected format: %s)", archivePath, format)
	}

	if e.dryRun {
		e.log.Info("DRY RUN: Would restore cluster", "archive", archivePath)
		return e.previewClusterRestore(archivePath)
	}

	e.progress.Start(fmt.Sprintf("Restoring cluster from %s", filepath.Base(archivePath)))

	// Create temporary extraction directory
	tempDir := filepath.Join(e.cfg.BackupDir, fmt.Sprintf(".restore_%d", time.Now().Unix()))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		operation.Fail("Failed to create temporary directory")
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract archive
	e.log.Info("Extracting cluster archive", "archive", archivePath, "tempDir", tempDir)
	if err := e.extractArchive(ctx, archivePath, tempDir); err != nil {
		operation.Fail("Archive extraction failed")
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	// Restore global objects (roles, tablespaces)
	globalsFile := filepath.Join(tempDir, "globals.sql")
	if _, err := os.Stat(globalsFile); err == nil {
		e.log.Info("Restoring global objects")
		e.progress.Update("Restoring global objects (roles, tablespaces)...")
		if err := e.restoreGlobals(ctx, globalsFile); err != nil {
			e.log.Warn("Failed to restore global objects", "error", err)
			// Continue anyway - global objects might already exist
		}
	}

	// Restore individual databases
	dumpsDir := filepath.Join(tempDir, "dumps")
	if _, err := os.Stat(dumpsDir); err != nil {
		operation.Fail("No database dumps found in archive")
		return fmt.Errorf("no database dumps found in archive")
	}

	entries, err := os.ReadDir(dumpsDir)
	if err != nil {
		operation.Fail("Failed to read dumps directory")
		return fmt.Errorf("failed to read dumps directory: %w", err)
	}

	successCount := 0
	failCount := 0

	for i, entry := range entries {
		if entry.IsDir() {
			continue
		}

		dumpFile := filepath.Join(dumpsDir, entry.Name())
		dbName := strings.TrimSuffix(entry.Name(), ".dump")

		e.progress.Update(fmt.Sprintf("[%d/%d] Restoring database: %s", i+1, len(entries), dbName))
		e.log.Info("Restoring database", "name", dbName, "file", dumpFile)

		// Create database first if it doesn't exist
		if err := e.ensureDatabaseExists(ctx, dbName); err != nil {
			e.log.Warn("Could not ensure database exists", "name", dbName, "error", err)
			// Continue anyway - pg_restore can create it
		}

		if err := e.restorePostgreSQLDump(ctx, dumpFile, dbName, false, false); err != nil {
			e.log.Error("Failed to restore database", "name", dbName, "error", err)
			failCount++
			continue
		}

		successCount++
	}

	if failCount > 0 {
		e.progress.Fail(fmt.Sprintf("Cluster restore completed with errors: %d succeeded, %d failed", successCount, failCount))
		operation.Complete(fmt.Sprintf("Partial restore: %d succeeded, %d failed", successCount, failCount))
		return fmt.Errorf("cluster restore completed with %d failures", failCount)
	}

	e.progress.Complete(fmt.Sprintf("Cluster restored successfully: %d databases", successCount))
	operation.Complete(fmt.Sprintf("Restored %d databases from cluster archive", successCount))
	return nil
}

// extractArchive extracts a tar.gz archive
func (e *Engine) extractArchive(ctx context.Context, archivePath, destDir string) error {
	cmd := exec.CommandContext(ctx, "tar", "-xzf", archivePath, "-C", destDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar extraction failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// restoreGlobals restores global objects (roles, tablespaces)
func (e *Engine) restoreGlobals(ctx context.Context, globalsFile string) error {
	cmd := exec.CommandContext(ctx,
		"psql",
		"-h", e.cfg.Host,
		"-p", fmt.Sprintf("%d", e.cfg.Port),
		"-U", e.cfg.User,
		"-d", "postgres",
		"-f", globalsFile,
	)

	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", e.cfg.Password))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restore globals: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// ensureDatabaseExists checks if a database exists and creates it if not
func (e *Engine) ensureDatabaseExists(ctx context.Context, dbName string) error {
	// Check if database exists using psql
	checkCmd := exec.CommandContext(ctx, "psql",
		"-h", e.cfg.Host,
		"-p", fmt.Sprintf("%d", e.cfg.Port),
		"-U", e.cfg.User,
		"-d", "postgres", // Connect to default postgres database
		"-tAc",
		fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s'", dbName),
	)
	checkCmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", e.cfg.Password))

	output, err := checkCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check database existence: %w", err)
	}

	// If database exists, we're done
	if strings.TrimSpace(string(output)) == "1" {
		e.log.Info(fmt.Sprintf("Database '%s' already exists", dbName))
		return nil
	}

	// Database doesn't exist, create it
	e.log.Info(fmt.Sprintf("Creating database '%s'", dbName))
	createCmd := exec.CommandContext(ctx, "psql",
		"-h", e.cfg.Host,
		"-p", fmt.Sprintf("%d", e.cfg.Port),
		"-U", e.cfg.User,
		"-d", "postgres",
		"-c", fmt.Sprintf("CREATE DATABASE \"%s\"", dbName),
	)
	createCmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", e.cfg.Password))

	output, err = createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create database '%s': %w\nOutput: %s", dbName, err, string(output))
	}

	e.log.Info(fmt.Sprintf("Successfully created database '%s'", dbName))
	return nil
}

// previewClusterRestore shows cluster restore preview
func (e *Engine) previewClusterRestore(archivePath string) error {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println(" CLUSTER RESTORE PREVIEW (DRY RUN)")
	fmt.Println(strings.Repeat("=", 60))

	stat, _ := os.Stat(archivePath)
	fmt.Printf("\nArchive: %s\n", filepath.Base(archivePath))
	if stat != nil {
		fmt.Printf("Size: %s\n", FormatBytes(stat.Size()))
		fmt.Printf("Modified: %s\n", stat.ModTime().Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("Target Host: %s:%d\n", e.cfg.Host, e.cfg.Port)

	fmt.Println("\nOperations that would be performed:")
	fmt.Println("  1. Extract cluster archive to temporary directory")
	fmt.Println("  2. Restore global objects (roles, tablespaces)")
	fmt.Println("  3. Restore all databases found in archive")
	fmt.Println("  4. Cleanup temporary files")

	fmt.Println("\n⚠️  WARNING: This will restore multiple databases.")
	fmt.Println("   Existing databases may be overwritten or merged.")
	fmt.Println("\nTo execute this restore, add the --confirm flag.")
	fmt.Println(strings.Repeat("=", 60) + "\n")

	return nil
}

// FormatBytes formats bytes to human readable format
func FormatBytes(bytes int64) string {
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
