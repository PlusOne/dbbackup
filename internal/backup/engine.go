package backup

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dbbackup/internal/config"
	"dbbackup/internal/database"
	"dbbackup/internal/logger"
	"dbbackup/internal/progress"
	"dbbackup/internal/swap"
)

// Engine handles backup operations
type Engine struct {
	cfg              *config.Config
	log              logger.Logger
	db               database.Database
	progress         progress.Indicator
	detailedReporter *progress.DetailedReporter
	silent           bool // Silent mode for TUI
}

// New creates a new backup engine
func New(cfg *config.Config, log logger.Logger, db database.Database) *Engine {
	progressIndicator := progress.NewIndicator(true, "line") // Use line-by-line indicator
	detailedReporter := progress.NewDetailedReporter(progressIndicator, &loggerAdapter{logger: log})
	
	return &Engine{
		cfg:              cfg,
		log:              log,
		db:               db,
		progress:         progressIndicator,
		detailedReporter: detailedReporter,
		silent:           false,
	}
}

// NewWithProgress creates a new backup engine with a custom progress indicator
func NewWithProgress(cfg *config.Config, log logger.Logger, db database.Database, progressIndicator progress.Indicator) *Engine {
	detailedReporter := progress.NewDetailedReporter(progressIndicator, &loggerAdapter{logger: log})
	
	return &Engine{
		cfg:              cfg,
		log:              log,
		db:               db,
		progress:         progressIndicator,
		detailedReporter: detailedReporter,
		silent:           false,
	}
}

// NewSilent creates a new backup engine in silent mode (for TUI)
func NewSilent(cfg *config.Config, log logger.Logger, db database.Database, progressIndicator progress.Indicator) *Engine {
	// If no indicator provided, use null indicator (no output)
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
		silent:           true, // Silent mode enabled
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

// printf prints to stdout only if not in silent mode
func (e *Engine) printf(format string, args ...interface{}) {
	if !e.silent {
		fmt.Printf(format, args...)
	}
}

// generateOperationID creates a unique operation ID
func generateOperationID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// BackupSingle performs a single database backup with detailed progress tracking
func (e *Engine) BackupSingle(ctx context.Context, databaseName string) error {
	// Start detailed operation tracking
	operationID := generateOperationID()
	tracker := e.detailedReporter.StartOperation(operationID, databaseName, "backup")
	
	// Add operation details
	tracker.SetDetails("database", databaseName)
	tracker.SetDetails("type", "single")
	tracker.SetDetails("compression", strconv.Itoa(e.cfg.CompressionLevel))
	tracker.SetDetails("format", "custom")
	
	// Start preparing backup directory
	prepStep := tracker.AddStep("prepare", "Preparing backup directory")
	if err := os.MkdirAll(e.cfg.BackupDir, 0755); err != nil {
		prepStep.Fail(fmt.Errorf("failed to create backup directory: %w", err))
		tracker.Fail(fmt.Errorf("failed to create backup directory: %w", err))
		return fmt.Errorf("failed to create backup directory: %w", err)
	}
	prepStep.Complete("Backup directory prepared")
	tracker.UpdateProgress(10, "Backup directory prepared")
	
	// Generate timestamp and filename
	timestamp := time.Now().Format("20060102_150405")
	var outputFile string
	
	if e.cfg.IsPostgreSQL() {
		outputFile = filepath.Join(e.cfg.BackupDir, fmt.Sprintf("db_%s_%s.dump", databaseName, timestamp))
	} else {
		outputFile = filepath.Join(e.cfg.BackupDir, fmt.Sprintf("db_%s_%s.sql.gz", databaseName, timestamp))
	}
	
	tracker.SetDetails("output_file", outputFile)
	tracker.UpdateProgress(20, "Generated backup filename")
	
	// Build backup command
	cmdStep := tracker.AddStep("command", "Building backup command")
	options := database.BackupOptions{
		Compression:  e.cfg.CompressionLevel,
		Parallel:     e.cfg.DumpJobs,
		Format:       "custom",
		Blobs:        true,
		NoOwner:      false,
		NoPrivileges: false,
	}
	
	cmd := e.db.BuildBackupCommand(databaseName, outputFile, options)
	cmdStep.Complete("Backup command prepared")
	tracker.UpdateProgress(30, "Backup command prepared")
	
	// Execute backup command with progress monitoring
	execStep := tracker.AddStep("execute", "Executing database backup")
	tracker.UpdateProgress(40, "Starting database backup...")
	
	if err := e.executeCommandWithProgress(ctx, cmd, outputFile, tracker); err != nil {
		execStep.Fail(fmt.Errorf("backup execution failed: %w", err))
		tracker.Fail(fmt.Errorf("backup failed: %w", err))
		return fmt.Errorf("backup failed: %w", err)
	}
	execStep.Complete("Database backup completed")
	tracker.UpdateProgress(80, "Database backup completed")
	
	// Verify backup file
	verifyStep := tracker.AddStep("verify", "Verifying backup file")
	if info, err := os.Stat(outputFile); err != nil {
		verifyStep.Fail(fmt.Errorf("backup file not created: %w", err))
		tracker.Fail(fmt.Errorf("backup file not created: %w", err))
		return fmt.Errorf("backup file not created: %w", err)
	} else {
		size := formatBytes(info.Size())
		tracker.SetDetails("file_size", size)
		tracker.SetByteProgress(info.Size(), info.Size())
		verifyStep.Complete(fmt.Sprintf("Backup file verified: %s", size))
		tracker.UpdateProgress(90, fmt.Sprintf("Backup verified: %s", size))
	}
	
	// Create metadata file
	metaStep := tracker.AddStep("metadata", "Creating metadata file")
	if err := e.createMetadata(outputFile, databaseName, "single", ""); err != nil {
		e.log.Warn("Failed to create metadata file", "error", err)
		metaStep.Fail(fmt.Errorf("metadata creation failed: %w", err))
	} else {
		metaStep.Complete("Metadata file created")
	}
	
	// Complete operation
	tracker.UpdateProgress(100, "Backup operation completed successfully")
	tracker.Complete(fmt.Sprintf("Single database backup completed: %s", filepath.Base(outputFile)))
	
	return nil
}

// BackupSample performs a sample database backup
func (e *Engine) BackupSample(ctx context.Context, databaseName string) error {
	operation := e.log.StartOperation("Sample Database Backup")
	
	// Ensure backup directory exists
	if err := os.MkdirAll(e.cfg.BackupDir, 0755); err != nil {
		operation.Fail("Failed to create backup directory")
		return fmt.Errorf("failed to create backup directory: %w", err)
	}
	
	// Generate timestamp and filename
	timestamp := time.Now().Format("20060102_150405")
	outputFile := filepath.Join(e.cfg.BackupDir, 
		fmt.Sprintf("sample_%s_%s%d_%s.sql", databaseName, e.cfg.SampleStrategy, e.cfg.SampleValue, timestamp))
	
	operation.Update("Starting sample database backup")
	e.progress.Start(fmt.Sprintf("Creating sample backup of '%s' (%s=%d)", databaseName, e.cfg.SampleStrategy, e.cfg.SampleValue))
	
	// For sample backups, we need to get the schema first, then sample data
	if err := e.createSampleBackup(ctx, databaseName, outputFile); err != nil {
		e.progress.Fail(fmt.Sprintf("Sample backup failed: %v", err))
		operation.Fail("Sample backup failed")
		return fmt.Errorf("sample backup failed: %w", err)
	}
	
	// Check output file
	if info, err := os.Stat(outputFile); err != nil {
		e.progress.Fail("Sample backup file not created")
		operation.Fail("Sample backup file not found")
		return fmt.Errorf("sample backup file not created: %w", err)
	} else {
		size := formatBytes(info.Size())
		e.progress.Complete(fmt.Sprintf("Sample backup completed: %s (%s)", filepath.Base(outputFile), size))
		operation.Complete(fmt.Sprintf("Sample backup created: %s (%s)", outputFile, size))
	}
	
	// Create metadata file
	if err := e.createMetadata(outputFile, databaseName, "sample", e.cfg.SampleStrategy); err != nil {
		e.log.Warn("Failed to create metadata file", "error", err)
	}
	
	return nil
}

// BackupCluster performs a full cluster backup (PostgreSQL only)
func (e *Engine) BackupCluster(ctx context.Context) error {
	if !e.cfg.IsPostgreSQL() {
		return fmt.Errorf("cluster backup is only supported for PostgreSQL")
	}
	
	operation := e.log.StartOperation("Cluster Backup")
	
	// Setup swap file if configured
	var swapMgr *swap.Manager
	if e.cfg.AutoSwap && e.cfg.SwapFileSizeGB > 0 {
		swapMgr = swap.NewManager(e.cfg.SwapFilePath, e.cfg.SwapFileSizeGB, e.log)
		
		if swapMgr.IsSupported() {
			e.log.Info("Setting up temporary swap file for large backup", 
				"path", e.cfg.SwapFilePath, 
				"size_gb", e.cfg.SwapFileSizeGB)
			
			if err := swapMgr.Setup(); err != nil {
				e.log.Warn("Failed to setup swap file (continuing without it)", "error", err)
			} else {
				// Ensure cleanup on exit
				defer func() {
					if err := swapMgr.Cleanup(); err != nil {
						e.log.Warn("Failed to cleanup swap file", "error", err)
					}
				}()
			}
		} else {
			e.log.Warn("Swap file management not supported on this platform", "os", swapMgr)
		}
	}
	
	// Use appropriate progress indicator based on silent mode
	var quietProgress progress.Indicator
	if e.silent {
		// In silent mode (TUI), use null indicator - no stdout output at all
		quietProgress = progress.NewNullIndicator()
	} else {
		// In CLI mode, use quiet line-by-line output
		quietProgress = progress.NewQuietLineByLine()
		quietProgress.Start("Starting cluster backup (all databases)")
	}
	
	// Ensure backup directory exists
	if err := os.MkdirAll(e.cfg.BackupDir, 0755); err != nil {
		operation.Fail("Failed to create backup directory")
		quietProgress.Fail("Failed to create backup directory")
		return fmt.Errorf("failed to create backup directory: %w", err)
	}
	
	// Generate timestamp and filename
	timestamp := time.Now().Format("20060102_150405")
	outputFile := filepath.Join(e.cfg.BackupDir, fmt.Sprintf("cluster_%s.tar.gz", timestamp))
	tempDir := filepath.Join(e.cfg.BackupDir, fmt.Sprintf(".cluster_%s", timestamp))
	
	operation.Update("Starting cluster backup")
	
	// Create temporary directory
	if err := os.MkdirAll(filepath.Join(tempDir, "dumps"), 0755); err != nil {
		operation.Fail("Failed to create temporary directory")
		quietProgress.Fail("Failed to create temporary directory")
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Backup globals
	e.printf("   Backing up global objects...\n")
	if err := e.backupGlobals(ctx, tempDir); err != nil {
		quietProgress.Fail(fmt.Sprintf("Failed to backup globals: %v", err))
		operation.Fail("Global backup failed")
		return fmt.Errorf("failed to backup globals: %w", err)
	}
	
	// Get list of databases
	e.printf("   Getting database list...\n")
	databases, err := e.db.ListDatabases(ctx)
	if err != nil {
		quietProgress.Fail(fmt.Sprintf("Failed to list databases: %v", err))
		operation.Fail("Database listing failed")
		return fmt.Errorf("failed to list databases: %w", err)
	}
	
	// Create ETA estimator for database backups
	estimator := progress.NewETAEstimator("Backing up cluster", len(databases))
	quietProgress.SetEstimator(estimator)
	
	// Backup each database
	e.printf("   Backing up %d databases...\n", len(databases))
	successCount := 0
	failCount := 0
	
	for i, dbName := range databases {
		// Update estimator progress
		estimator.UpdateProgress(i)
		
		e.printf("   [%d/%d] Backing up database: %s\n", i+1, len(databases), dbName)
		quietProgress.Update(fmt.Sprintf("Backing up database %d/%d: %s", i+1, len(databases), dbName))
		
		// Check database size and warn if very large
		if size, err := e.db.GetDatabaseSize(ctx, dbName); err == nil {
			sizeStr := formatBytes(size)
			e.printf("       Database size: %s\n", sizeStr)
			if size > 10*1024*1024*1024 { // > 10GB
				e.printf("       ⚠️  Large database detected - this may take a while\n")
			}
		}
		
		dumpFile := filepath.Join(tempDir, "dumps", dbName+".dump")
		
		// For cluster backups, use settings optimized for large databases:
		// - Lower compression (faster, less memory)
		// - Use parallel dumps if configured
		// - Smart format selection based on size
		
		compressionLevel := e.cfg.CompressionLevel
		if compressionLevel > 6 {
			compressionLevel = 6 // Cap at 6 for cluster backups to reduce memory
		}
		
		// Determine optimal format based on database size
		format := "custom"
		parallel := e.cfg.DumpJobs
		
		// For large databases (>5GB), use plain format with external compression
		// This avoids pg_dump's custom format memory overhead
		if size, err := e.db.GetDatabaseSize(ctx, dbName); err == nil {
			if size > 5*1024*1024*1024 { // > 5GB
				format = "plain"        // Plain SQL format
				compressionLevel = 0    // Disable pg_dump compression
				parallel = 0            // Plain format doesn't support parallel
				e.printf("       Using plain format + external compression (optimal for large DBs)\n")
			}
		}
		
		options := database.BackupOptions{
			Compression:  compressionLevel,
			Parallel:     parallel,
			Format:       format,
			Blobs:        true,
			NoOwner:      false,
			NoPrivileges: false,
		}
		
		cmd := e.db.BuildBackupCommand(dbName, dumpFile, options)
		
		// Use a context with timeout for each database to prevent hangs
		// Use longer timeout for huge databases (2 hours per database)
		dbCtx, cancel := context.WithTimeout(ctx, 2*time.Hour)
		defer cancel() // Ensure cancel is called even if executeCommand panics
		err := e.executeCommand(dbCtx, cmd, dumpFile)
		cancel() // Also call immediately for early cleanup
		
		if err != nil {
			e.log.Warn("Failed to backup database", "database", dbName, "error", err)
			e.printf("   ⚠️  WARNING: Failed to backup %s: %v\n", dbName, err)
			failCount++
			// Continue with other databases
		} else {
			// If streaming compression was used the compressed file may have a different name
			// (e.g. .sql.gz). Prefer compressed file size when present, fall back to dumpFile.
			compressedCandidate := strings.TrimSuffix(dumpFile, ".dump") + ".sql.gz"
			if info, err := os.Stat(compressedCandidate); err == nil {
				e.printf("   ✅ Completed %s (%s)\n", dbName, formatBytes(info.Size()))
			} else if info, err := os.Stat(dumpFile); err == nil {
				e.printf("   ✅ Completed %s (%s)\n", dbName, formatBytes(info.Size()))
			}
			successCount++
		}
	}
	
	e.printf("   Backup summary: %d succeeded, %d failed\n", successCount, failCount)
	
	// Create archive
	e.printf("   Creating compressed archive...\n")
	if err := e.createArchive(ctx, tempDir, outputFile); err != nil {
		quietProgress.Fail(fmt.Sprintf("Failed to create archive: %v", err))
		operation.Fail("Archive creation failed")
		return fmt.Errorf("failed to create archive: %w", err)
	}
	
	// Check output file
	if info, err := os.Stat(outputFile); err != nil {
		quietProgress.Fail("Cluster backup archive not created")
		operation.Fail("Cluster backup archive not found")
		return fmt.Errorf("cluster backup archive not created: %w", err)
	} else {
		size := formatBytes(info.Size())
		quietProgress.Complete(fmt.Sprintf("Cluster backup completed: %s (%s)", filepath.Base(outputFile), size))
		operation.Complete(fmt.Sprintf("Cluster backup created: %s (%s)", outputFile, size))
	}
	
	// Create metadata file
	if err := e.createMetadata(outputFile, "cluster", "cluster", ""); err != nil {
		e.log.Warn("Failed to create metadata file", "error", err)
	}
	
	return nil
}

// executeCommandWithProgress executes a backup command with real-time progress monitoring
func (e *Engine) executeCommandWithProgress(ctx context.Context, cmdArgs []string, outputFile string, tracker *progress.OperationTracker) error {
	if len(cmdArgs) == 0 {
		return fmt.Errorf("empty command")
	}
	
	e.log.Debug("Executing backup command with progress", "cmd", cmdArgs[0], "args", cmdArgs[1:])
	
	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	
	// Set environment variables for database tools
	cmd.Env = os.Environ()
	if e.cfg.Password != "" {
		if e.cfg.IsPostgreSQL() {
			cmd.Env = append(cmd.Env, "PGPASSWORD="+e.cfg.Password)
		} else if e.cfg.IsMySQL() {
			cmd.Env = append(cmd.Env, "MYSQL_PWD="+e.cfg.Password)
		}
	}
	
	// For MySQL, handle compression and redirection differently
	if e.cfg.IsMySQL() && e.cfg.CompressionLevel > 0 {
		return e.executeMySQLWithProgressAndCompression(ctx, cmdArgs, outputFile, tracker)
	}
	
	// Get stderr pipe for progress monitoring
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}
	
	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}
	
	// Monitor progress via stderr
	go e.monitorCommandProgress(stderr, tracker)
	
	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("backup command failed: %w", err)
	}
	
	return nil
}

// monitorCommandProgress monitors command output for progress information
func (e *Engine) monitorCommandProgress(stderr io.ReadCloser, tracker *progress.OperationTracker) {
	defer stderr.Close()
	
	scanner := bufio.NewScanner(stderr)
	progressBase := 40 // Start from 40% since command preparation is done
	progressIncrement := 0
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		e.log.Debug("Command output", "line", line)
		
		// Increment progress gradually based on output
		if progressBase < 75 {
			progressIncrement++
			if progressIncrement%5 == 0 { // Update every 5 lines
				progressBase += 2
				tracker.UpdateProgress(progressBase, "Processing data...")
			}
		}
		
		// Look for specific progress indicators
		if strings.Contains(line, "COPY") {
			tracker.UpdateProgress(progressBase+5, "Copying table data...")
		} else if strings.Contains(line, "completed") {
			tracker.UpdateProgress(75, "Backup nearly complete...")
		} else if strings.Contains(line, "done") {
			tracker.UpdateProgress(78, "Finalizing backup...")
		}
	}
}

// executeMySQLWithProgressAndCompression handles MySQL backup with compression and progress
func (e *Engine) executeMySQLWithProgressAndCompression(ctx context.Context, cmdArgs []string, outputFile string, tracker *progress.OperationTracker) error {
	// Create mysqldump command
	dumpCmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	dumpCmd.Env = os.Environ()
	if e.cfg.Password != "" {
		dumpCmd.Env = append(dumpCmd.Env, "MYSQL_PWD="+e.cfg.Password)
	}
	
	// Create gzip command
	gzipCmd := exec.CommandContext(ctx, "gzip", fmt.Sprintf("-%d", e.cfg.CompressionLevel))
	
	// Create output file
	outFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()
	
	// Set up pipeline: mysqldump | gzip > outputfile
	pipe, err := dumpCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	
	gzipCmd.Stdin = pipe
	gzipCmd.Stdout = outFile
	
	// Get stderr for progress monitoring
	stderr, err := dumpCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}
	
	// Start monitoring progress
	go e.monitorCommandProgress(stderr, tracker)
	
	// Start both commands
	if err := gzipCmd.Start(); err != nil {
		return fmt.Errorf("failed to start gzip: %w", err)
	}
	
	if err := dumpCmd.Start(); err != nil {
		return fmt.Errorf("failed to start mysqldump: %w", err)
	}
	
	// Wait for mysqldump to complete
	if err := dumpCmd.Wait(); err != nil {
		return fmt.Errorf("mysqldump failed: %w", err)
	}
	
	// Close pipe and wait for gzip
	pipe.Close()
	if err := gzipCmd.Wait(); err != nil {
		return fmt.Errorf("gzip failed: %w", err)
	}
	
	return nil
}

// executeMySQLWithCompression handles MySQL backup with compression
func (e *Engine) executeMySQLWithCompression(ctx context.Context, cmdArgs []string, outputFile string) error {
	// Create mysqldump command
	dumpCmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	dumpCmd.Env = os.Environ()
	if e.cfg.Password != "" {
		dumpCmd.Env = append(dumpCmd.Env, "MYSQL_PWD="+e.cfg.Password)
	}
	
	// Create gzip command
	gzipCmd := exec.CommandContext(ctx, "gzip", fmt.Sprintf("-%d", e.cfg.CompressionLevel))
	
	// Create output file
	outFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()
	
	// Set up pipeline: mysqldump | gzip > outputfile
	stdin, err := dumpCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	gzipCmd.Stdin = stdin
	gzipCmd.Stdout = outFile
	
	// Start both commands
	if err := gzipCmd.Start(); err != nil {
		return fmt.Errorf("failed to start gzip: %w", err)
	}
	
	if err := dumpCmd.Run(); err != nil {
		return fmt.Errorf("mysqldump failed: %w", err)
	}
	
	if err := gzipCmd.Wait(); err != nil {
		return fmt.Errorf("gzip failed: %w", err)
	}
	
	return nil
}

// createSampleBackup creates a sample backup with reduced dataset
func (e *Engine) createSampleBackup(ctx context.Context, databaseName, outputFile string) error {
	// This is a simplified implementation
	// A full implementation would:
	// 1. Export schema
	// 2. Get list of tables
	// 3. For each table, run sampling query
	// 4. Combine into single SQL file
	
	// For now, we'll use a simple approach with schema-only backup first
	// Then add sample data
	
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create sample backup file: %w", err)
	}
	defer file.Close()
	
	// Write header
	fmt.Fprintf(file, "-- Sample Database Backup\n")
	fmt.Fprintf(file, "-- Database: %s\n", databaseName)
	fmt.Fprintf(file, "-- Strategy: %s = %d\n", e.cfg.SampleStrategy, e.cfg.SampleValue)
	fmt.Fprintf(file, "-- Created: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "-- WARNING: This backup may have referential integrity issues!\n\n")
	
	// For PostgreSQL, we can use pg_dump --schema-only first
	if e.cfg.IsPostgreSQL() {
		// Get schema
		schemaCmd := e.db.BuildBackupCommand(databaseName, "/dev/stdout", database.BackupOptions{
			SchemaOnly: true,
			Format:     "plain",
		})
		
		cmd := exec.CommandContext(ctx, schemaCmd[0], schemaCmd[1:]...)
		cmd.Env = os.Environ()
		if e.cfg.Password != "" {
			cmd.Env = append(cmd.Env, "PGPASSWORD="+e.cfg.Password)
		}
		cmd.Stdout = file
		
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to export schema: %w", err)
		}
		
		fmt.Fprintf(file, "\n-- Sample data follows\n\n")
		
		// Get tables and sample data
		tables, err := e.db.ListTables(ctx, databaseName)
		if err != nil {
			return fmt.Errorf("failed to list tables: %w", err)
		}
		
		strategy := database.SampleStrategy{
			Type:  e.cfg.SampleStrategy,
			Value: e.cfg.SampleValue,
		}
		
		for _, table := range tables {
			fmt.Fprintf(file, "-- Data for table: %s\n", table)
			sampleQuery := e.db.BuildSampleQuery(databaseName, table, strategy)
			fmt.Fprintf(file, "\\copy (%s) TO STDOUT\n\n", sampleQuery)
		}
	}
	
	return nil
}

// backupGlobals creates a backup of global PostgreSQL objects
func (e *Engine) backupGlobals(ctx context.Context, tempDir string) error {
	globalsFile := filepath.Join(tempDir, "globals.sql")
	
	cmd := exec.CommandContext(ctx, "pg_dumpall", "--globals-only")
	if e.cfg.Host != "localhost" {
		cmd.Args = append(cmd.Args, "-h", e.cfg.Host, "-p", fmt.Sprintf("%d", e.cfg.Port))
	}
	cmd.Args = append(cmd.Args, "-U", e.cfg.User)
	
	cmd.Env = os.Environ()
	if e.cfg.Password != "" {
		cmd.Env = append(cmd.Env, "PGPASSWORD="+e.cfg.Password)
	}
	
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("pg_dumpall failed: %w", err)
	}
	
	return os.WriteFile(globalsFile, output, 0644)
}

// createArchive creates a compressed tar archive
func (e *Engine) createArchive(ctx context.Context, sourceDir, outputFile string) error {
	// Use pigz for faster parallel compression if available, otherwise use standard gzip
	compressCmd := "tar"
	compressArgs := []string{"-czf", outputFile, "-C", sourceDir, "."}
	
	// Check if pigz is available for faster parallel compression
	if _, err := exec.LookPath("pigz"); err == nil {
		// Use pigz with number of cores for parallel compression
		compressArgs = []string{"-cf", "-", "-C", sourceDir, "."}
		cmd := exec.CommandContext(ctx, "tar", compressArgs...)
		
		// Create output file
		outFile, err := os.Create(outputFile)
		if err != nil {
			// Fallback to regular tar
			goto regularTar
		}
		defer outFile.Close()
		
		// Pipe to pigz for parallel compression
		pigzCmd := exec.CommandContext(ctx, "pigz", "-p", strconv.Itoa(e.cfg.Jobs))
		
		tarOut, err := cmd.StdoutPipe()
		if err != nil {
			outFile.Close()
			// Fallback to regular tar
			goto regularTar
		}
		pigzCmd.Stdin = tarOut
		pigzCmd.Stdout = outFile
		
		// Start both commands
		if err := pigzCmd.Start(); err != nil {
			outFile.Close()
			goto regularTar
		}
		if err := cmd.Start(); err != nil {
			pigzCmd.Process.Kill()
			outFile.Close()
			goto regularTar
		}
		
		// Wait for tar to finish
		if err := cmd.Wait(); err != nil {
			pigzCmd.Process.Kill()
			return fmt.Errorf("tar failed: %w", err)
		}
		
		// Wait for pigz to finish
		if err := pigzCmd.Wait(); err != nil {
			return fmt.Errorf("pigz compression failed: %w", err)
		}
		return nil
	}

regularTar:
	// Standard tar with gzip (fallback)
	cmd := exec.CommandContext(ctx, compressCmd, compressArgs...)
	
	// Stream stderr to avoid memory issues
	// Use io.Copy to ensure goroutine completes when pipe closes
	stderr, err := cmd.StderrPipe()
	if err == nil {
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				if line != "" {
					e.log.Debug("Archive creation", "output", line)
				}
			}
			// Scanner will exit when stderr pipe closes after cmd.Wait()
		}()
	}
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tar failed: %w", err)
	}
	// cmd.Run() calls Wait() which closes stderr pipe, terminating the goroutine
	return nil
}

// createMetadata creates a metadata file for the backup
func (e *Engine) createMetadata(backupFile, database, backupType, strategy string) error {
	metaFile := backupFile + ".info"
	
	content := fmt.Sprintf(`{
  "type": "%s",
  "database": "%s",
  "timestamp": "%s",
  "host": "%s",
  "port": %d,
  "user": "%s",
  "db_type": "%s",
  "compression": %d`,
		backupType, database, time.Now().Format("20060102_150405"),
		e.cfg.Host, e.cfg.Port, e.cfg.User, e.cfg.DatabaseType, e.cfg.CompressionLevel)
	
	if strategy != "" {
		content += fmt.Sprintf(`,
  "sample_strategy": "%s",
  "sample_value": %d`, e.cfg.SampleStrategy, e.cfg.SampleValue)
	}
	
	if info, err := os.Stat(backupFile); err == nil {
		content += fmt.Sprintf(`,
  "size_bytes": %d`, info.Size())
	}
	
	content += "\n}"
	
	return os.WriteFile(metaFile, []byte(content), 0644)
}

// executeCommand executes a backup command (optimized for huge databases)
func (e *Engine) executeCommand(ctx context.Context, cmdArgs []string, outputFile string) error {
	if len(cmdArgs) == 0 {
		return fmt.Errorf("empty command")
	}
	
	e.log.Debug("Executing backup command", "cmd", cmdArgs[0], "args", cmdArgs[1:])
	
	// Check if pg_dump will write to stdout (which means we need to handle piping to compressor).
	// BuildBackupCommand omits --file when format==plain AND compression==0, causing pg_dump
	// to write to stdout. In that case we must pipe to external compressor.
	usesStdout := false
	isPlainFormat := false
	hasFileFlag := false

	for _, arg := range cmdArgs {
		if strings.HasPrefix(arg, "--format=") && strings.Contains(arg, "plain") {
			isPlainFormat = true
		}
		if arg == "-Fp" {
			isPlainFormat = true
		}
		if arg == "--file" || strings.HasPrefix(arg, "--file=") {
			hasFileFlag = true
		}
	}

	// If plain format and no --file specified, pg_dump writes to stdout
	if isPlainFormat && !hasFileFlag {
		usesStdout = true
	}
	
	e.log.Debug("Backup command analysis", 
		"plain_format", isPlainFormat, 
		"has_file_flag", hasFileFlag, 
		"uses_stdout", usesStdout,
		"output_file", outputFile)
	
	// For MySQL, handle compression differently
	if e.cfg.IsMySQL() && e.cfg.CompressionLevel > 0 {
		return e.executeMySQLWithCompression(ctx, cmdArgs, outputFile)
	}
	
	// For plain format writing to stdout, use streaming compression
	if usesStdout {
		e.log.Debug("Using streaming compression for large database")
		return e.executeWithStreamingCompression(ctx, cmdArgs, outputFile)
	}
	
	// For custom format, pg_dump handles everything (writes directly to file)
	// NO GO BUFFERING - pg_dump writes directly to disk
	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	
	// Set environment variables for database tools
	cmd.Env = os.Environ()
	if e.cfg.Password != "" {
		if e.cfg.IsPostgreSQL() {
			cmd.Env = append(cmd.Env, "PGPASSWORD="+e.cfg.Password)
		} else if e.cfg.IsMySQL() {
			cmd.Env = append(cmd.Env, "MYSQL_PWD="+e.cfg.Password)
		}
	}
	
	// Stream stderr to avoid memory issues with large databases
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start backup command: %w", err)
	}
	
	// Stream stderr output (don't buffer it all in memory)
	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 1MB max line size
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				e.log.Debug("Backup output", "line", line)
			}
		}
	}()
	
	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		e.log.Error("Backup command failed", "error", err, "database", filepath.Base(outputFile))
		return fmt.Errorf("backup command failed: %w", err)
	}
	
	return nil
}

// executeWithStreamingCompression handles plain format dumps with external compression
// Uses: pg_dump | pigz > file.sql.gz (zero-copy streaming)
func (e *Engine) executeWithStreamingCompression(ctx context.Context, cmdArgs []string, outputFile string) error {
	e.log.Debug("Using streaming compression for large database")
	
	// Derive compressed output filename. If the output was named *.dump we replace that
	// with *.sql.gz; otherwise append .gz to the provided output file so we don't
	// accidentally create unwanted double extensions.
	var compressedFile string
	lowerOut := strings.ToLower(outputFile)
	if strings.HasSuffix(lowerOut, ".dump") {
		compressedFile = strings.TrimSuffix(outputFile, ".dump") + ".sql.gz"
	} else if strings.HasSuffix(lowerOut, ".sql") {
		compressedFile = outputFile + ".gz"
	} else {
		compressedFile = outputFile + ".gz"
	}
	
	// Create pg_dump command
	dumpCmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	dumpCmd.Env = os.Environ()
	if e.cfg.Password != "" && e.cfg.IsPostgreSQL() {
		dumpCmd.Env = append(dumpCmd.Env, "PGPASSWORD="+e.cfg.Password)
	}
	
	// Check for pigz (parallel gzip)
	compressor := "gzip"
	compressorArgs := []string{"-c"}
	
	if _, err := exec.LookPath("pigz"); err == nil {
		compressor = "pigz"
		compressorArgs = []string{"-p", strconv.Itoa(e.cfg.Jobs), "-c"}
		e.log.Debug("Using pigz for parallel compression", "threads", e.cfg.Jobs)
	}
	
	// Create compression command
	compressCmd := exec.CommandContext(ctx, compressor, compressorArgs...)
	
	// Create output file
	outFile, err := os.Create(compressedFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()
	
	// Set up pipeline: pg_dump | pigz > file.sql.gz
	dumpStdout, err := dumpCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create dump stdout pipe: %w", err)
	}
	
	compressCmd.Stdin = dumpStdout
	compressCmd.Stdout = outFile
	
	// Capture stderr from both commands
	dumpStderr, err := dumpCmd.StderrPipe()
	if err != nil {
		e.log.Warn("Failed to capture dump stderr", "error", err)
	}
	compressStderr, err := compressCmd.StderrPipe()
	if err != nil {
		e.log.Warn("Failed to capture compress stderr", "error", err)
	}
	
	// Stream stderr output
	if dumpStderr != nil {
		go func() {
			scanner := bufio.NewScanner(dumpStderr)
			for scanner.Scan() {
				line := scanner.Text()
				if line != "" {
					e.log.Debug("pg_dump", "output", line)
				}
			}
		}()
	}
	
	if compressStderr != nil {
		go func() {
			scanner := bufio.NewScanner(compressStderr)
			for scanner.Scan() {
				line := scanner.Text()
				if line != "" {
					e.log.Debug("compression", "output", line)
				}
			}
		}()
	}
	
	// Start compression first
	if err := compressCmd.Start(); err != nil {
		return fmt.Errorf("failed to start compressor: %w", err)
	}
	
	// Then start pg_dump
	if err := dumpCmd.Start(); err != nil {
		return fmt.Errorf("failed to start pg_dump: %w", err)
	}
	
	// Wait for pg_dump to complete
	if err := dumpCmd.Wait(); err != nil {
		return fmt.Errorf("pg_dump failed: %w", err)
	}
	
	// Close stdout pipe to signal compressor we're done
	dumpStdout.Close()
	
	// Wait for compression to complete
	if err := compressCmd.Wait(); err != nil {
		return fmt.Errorf("compression failed: %w", err)
	}
	
	e.log.Debug("Streaming compression completed", "output", compressedFile)
	return nil
}

// formatBytes formats byte count in human-readable format
func formatBytes(bytes int64) string {
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