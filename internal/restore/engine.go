package restore

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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

// NewSilent creates a new restore engine with no stdout progress (for TUI mode)
func NewSilent(cfg *config.Config, log logger.Logger, db database.Database) *Engine {
	progressIndicator := progress.NewNullIndicator()
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

	// Check version compatibility for PostgreSQL dumps
	if format == FormatPostgreSQLDump || format == FormatPostgreSQLDumpGz {
		if compatResult, err := e.CheckRestoreVersionCompatibility(ctx, archivePath); err == nil && compatResult != nil {
			e.log.Info(compatResult.Message,
				"source_version", compatResult.SourceVersion.Full,
				"target_version", compatResult.TargetVersion.Full,
				"compatibility", compatResult.Level.String())

			// Block unsupported downgrades
			if !compatResult.Compatible {
				operation.Fail(compatResult.Message)
				return fmt.Errorf("version compatibility error: %s", compatResult.Message)
			}

			// Show warnings for risky upgrades
			if compatResult.Level == CompatibilityLevelRisky || compatResult.Level == CompatibilityLevelWarning {
				for _, warning := range compatResult.Warnings {
					e.log.Warn(warning)
				}
			}
		}
	}

	if e.dryRun {
		e.log.Info("DRY RUN: Would restore single database", "archive", archivePath, "target", targetDB)
		return e.previewRestore(archivePath, targetDB, format)
	}

	// Start progress tracking
	e.progress.Start(fmt.Sprintf("Restoring database '%s' from %s", targetDB, filepath.Base(archivePath)))

	// Create database if requested and it doesn't exist
	if createIfMissing {
		e.log.Info("Checking if target database exists", "database", targetDB)
		if err := e.ensureDatabaseExists(ctx, targetDB); err != nil {
			operation.Fail(fmt.Sprintf("Failed to create database: %v", err))
			return fmt.Errorf("failed to create database '%s': %w", targetDB, err)
		}
	}

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
		SingleTransaction: false, // CRITICAL: Disabled to prevent lock exhaustion with large objects
		Verbose:           true,  // Enable verbose for single database restores (not cluster)
	}

	cmd := e.db.BuildRestoreCommand(targetDB, archivePath, opts)

	if compressed {
		// For compressed dumps, decompress first
		return e.executeRestoreWithDecompression(ctx, archivePath, cmd)
	}

	return e.executeRestoreCommand(ctx, cmd)
}

// restorePostgreSQLDumpWithOwnership restores from PostgreSQL custom dump with ownership control
func (e *Engine) restorePostgreSQLDumpWithOwnership(ctx context.Context, archivePath, targetDB string, compressed bool, preserveOwnership bool) error {
	// Build restore command with ownership control
	opts := database.RestoreOptions{
		Parallel:          1,
		Clean:             false,              // We already dropped the database
		NoOwner:           !preserveOwnership, // Preserve ownership if we're superuser
		NoPrivileges:      !preserveOwnership, // Preserve privileges if we're superuser
		SingleTransaction: false,              // CRITICAL: Disabled to prevent lock exhaustion with large objects
		Verbose:           false,              // CRITICAL: disable verbose to prevent OOM on large restores
	}

	e.log.Info("Restoring database",
		"database", targetDB,
		"preserveOwnership", preserveOwnership,
		"noOwner", opts.NoOwner,
		"noPrivileges", opts.NoPrivileges)

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

	// For localhost, omit -h to use Unix socket (avoids Ident auth issues)
	hostArg := ""
	if e.cfg.Host != "localhost" && e.cfg.Host != "" {
		hostArg = fmt.Sprintf("-h %s -p %d", e.cfg.Host, e.cfg.Port)
	}

	if compressed {
		psqlCmd := fmt.Sprintf("psql -U %s -d %s", e.cfg.User, targetDB)
		if hostArg != "" {
			psqlCmd = fmt.Sprintf("psql %s -U %s -d %s", hostArg, e.cfg.User, targetDB)
		}
		// Set PGPASSWORD in the bash command for password-less auth
		cmd = []string{
			"bash", "-c",
			fmt.Sprintf("PGPASSWORD='%s' gunzip -c %s | %s", e.cfg.Password, archivePath, psqlCmd),
		}
	} else {
		if hostArg != "" {
			cmd = []string{
				"psql",
				"-h", e.cfg.Host,
				"-p", fmt.Sprintf("%d", e.cfg.Port),
				"-U", e.cfg.User,
				"-d", targetDB,
				"-f", archivePath,
			}
		} else {
			cmd = []string{
				"psql",
				"-U", e.cfg.User,
				"-d", targetDB,
				"-f", archivePath,
			}
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

	// Stream stderr to avoid memory issues with large output
	// Don't use CombinedOutput() as it loads everything into memory
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start restore command: %w", err)
	}

	// Read stderr in chunks to log errors without loading all into memory
	buf := make([]byte, 4096)
	var lastError string
	var errorCount int
	const maxErrors = 10 // Limit captured errors to prevent OOM
	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			// Only capture REAL errors, not verbose output
			if strings.Contains(chunk, "ERROR:") || strings.Contains(chunk, "FATAL:") || strings.Contains(chunk, "error:") {
				lastError = strings.TrimSpace(chunk)
				errorCount++
				if errorCount <= maxErrors {
					e.log.Warn("Restore stderr", "output", chunk)
				}
			}
			// Note: --verbose output is discarded to prevent OOM
		}
		if err != nil {
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		e.log.Error("Restore command failed", "error", err, "last_stderr", lastError, "error_count", errorCount)
		if lastError != "" {
			return fmt.Errorf("restore failed: %w (last error: %s, total errors: %d)", err, lastError, errorCount)
		}
		return fmt.Errorf("restore failed: %w", err)
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

	// Stream stderr to avoid memory issues with large output
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start restore command: %w", err)
	}

	// Read stderr in chunks to log errors without loading all into memory
	buf := make([]byte, 4096)
	var lastError string
	var errorCount int
	const maxErrors = 10 // Limit captured errors to prevent OOM
	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			// Only capture REAL errors, not verbose output
			if strings.Contains(chunk, "ERROR:") || strings.Contains(chunk, "FATAL:") || strings.Contains(chunk, "error:") {
				lastError = strings.TrimSpace(chunk)
				errorCount++
				if errorCount <= maxErrors {
					e.log.Warn("Restore stderr", "output", chunk)
				}
			}
			// Note: --verbose output is discarded to prevent OOM
		}
		if err != nil {
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		e.log.Error("Restore with decompression failed", "error", err, "last_stderr", lastError, "error_count", errorCount)
		if lastError != "" {
			return fmt.Errorf("restore failed: %w (last error: %s, total errors: %d)", err, lastError, errorCount)
		}
		return fmt.Errorf("restore failed: %w", err)
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

	// Check if user has superuser privileges (required for ownership restoration)
	e.progress.Update("Checking privileges...")
	isSuperuser, err := e.checkSuperuser(ctx)
	if err != nil {
		e.log.Warn("Could not verify superuser status", "error", err)
		isSuperuser = false // Assume not superuser if check fails
	}

	if !isSuperuser {
		e.log.Warn("Current user is not a superuser - database ownership may not be fully restored")
		e.progress.Update("⚠️  Warning: Non-superuser - ownership restoration limited")
		time.Sleep(2 * time.Second) // Give user time to see warning
	} else {
		e.log.Info("Superuser privileges confirmed - full ownership restoration enabled")
	}

	// Restore global objects FIRST (roles, tablespaces) - CRITICAL for ownership
	globalsFile := filepath.Join(tempDir, "globals.sql")
	if _, err := os.Stat(globalsFile); err == nil {
		e.log.Info("Restoring global objects (roles, tablespaces)")
		e.progress.Update("Restoring global objects (roles, tablespaces)...")
		if err := e.restoreGlobals(ctx, globalsFile); err != nil {
			e.log.Error("Failed to restore global objects", "error", err)
			if isSuperuser {
				// If we're superuser and can't restore globals, this is a problem
				e.progress.Fail("Failed to restore global objects")
				operation.Fail("Global objects restoration failed")
				return fmt.Errorf("failed to restore global objects: %w", err)
			} else {
				e.log.Warn("Continuing without global objects (may cause ownership issues)")
			}
		} else {
			e.log.Info("Successfully restored global objects")
		}
	} else {
		e.log.Warn("No globals.sql file found in backup - roles and tablespaces will not be restored")
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

	var failedDBs []string
	totalDBs := 0

	// Count total databases
	for _, entry := range entries {
		if !entry.IsDir() {
			totalDBs++
		}
	}

	// Create ETA estimator for database restores
	estimator := progress.NewETAEstimator("Restoring cluster", totalDBs)
	e.progress.SetEstimator(estimator)

	// Check for large objects in dump files and adjust parallelism
	hasLargeObjects := e.detectLargeObjectsInDumps(dumpsDir, entries)
	
	// Use worker pool for parallel restore
	parallelism := e.cfg.ClusterParallelism
	if parallelism < 1 {
		parallelism = 1 // Ensure at least sequential
	}
	
	// Automatically reduce parallelism if large objects detected
	if hasLargeObjects && parallelism > 1 {
		e.log.Warn("Large objects detected in dump files - reducing parallelism to avoid lock contention",
			"original_parallelism", parallelism,
			"adjusted_parallelism", 1)
		e.progress.Update("⚠️  Large objects detected - using sequential restore to avoid lock conflicts")
		time.Sleep(2 * time.Second) // Give user time to see warning
		parallelism = 1
	}

	var successCount, failCount int32
	var failedDBsMu sync.Mutex
	var mu sync.Mutex // Protect shared resources (progress, logger)

	// Create semaphore to limit concurrency
	semaphore := make(chan struct{}, parallelism)
	var wg sync.WaitGroup

	dbIndex := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		wg.Add(1)
		semaphore <- struct{}{} // Acquire

		go func(idx int, filename string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release

			// Update estimator progress (thread-safe)
			mu.Lock()
			estimator.UpdateProgress(idx)
			mu.Unlock()

			dumpFile := filepath.Join(dumpsDir, filename)
			dbName := filename
			dbName = strings.TrimSuffix(dbName, ".dump")
			dbName = strings.TrimSuffix(dbName, ".sql.gz")

			dbProgress := 15 + int(float64(idx)/float64(totalDBs)*85.0)

			mu.Lock()
			statusMsg := fmt.Sprintf("Restoring database %s (%d/%d)", dbName, idx+1, totalDBs)
			e.progress.Update(statusMsg)
			e.log.Info("Restoring database", "name", dbName, "file", dumpFile, "progress", dbProgress)
			mu.Unlock()

			// STEP 1: Drop existing database completely (clean slate)
			e.log.Info("Dropping existing database for clean restore", "name", dbName)
			if err := e.dropDatabaseIfExists(ctx, dbName); err != nil {
				e.log.Warn("Could not drop existing database", "name", dbName, "error", err)
			}

			// STEP 2: Create fresh database
			if err := e.ensureDatabaseExists(ctx, dbName); err != nil {
				e.log.Error("Failed to create database", "name", dbName, "error", err)
				failedDBsMu.Lock()
				failedDBs = append(failedDBs, fmt.Sprintf("%s: failed to create database: %v", dbName, err))
				failedDBsMu.Unlock()
				atomic.AddInt32(&failCount, 1)
				return
			}

			// STEP 3: Restore with ownership preservation if superuser
			preserveOwnership := isSuperuser
			isCompressedSQL := strings.HasSuffix(dumpFile, ".sql.gz")

			var restoreErr error
			if isCompressedSQL {
				mu.Lock()
				e.log.Info("Detected compressed SQL format, using psql + gunzip", "file", dumpFile, "database", dbName)
				mu.Unlock()
				restoreErr = e.restorePostgreSQLSQL(ctx, dumpFile, dbName, true)
			} else {
				mu.Lock()
				e.log.Info("Detected custom dump format, using pg_restore", "file", dumpFile, "database", dbName)
				mu.Unlock()
				restoreErr = e.restorePostgreSQLDumpWithOwnership(ctx, dumpFile, dbName, false, preserveOwnership)
			}

			if restoreErr != nil {
				mu.Lock()
				e.log.Error("Failed to restore database", "name", dbName, "file", dumpFile, "error", restoreErr)
				mu.Unlock()
				
				// Check for specific recoverable errors
				errMsg := restoreErr.Error()
				if strings.Contains(errMsg, "max_locks_per_transaction") {
					mu.Lock()
					e.log.Warn("Database restore failed due to insufficient locks - this is a PostgreSQL configuration issue", 
						"database", dbName, 
						"solution", "increase max_locks_per_transaction in postgresql.conf")
					mu.Unlock()
				} else if strings.Contains(errMsg, "total errors:") && strings.Contains(errMsg, "2562426") {
					mu.Lock()
					e.log.Warn("Database has massive error count - likely data corruption or incompatible dump format",
						"database", dbName,
						"errors", "2562426")
					mu.Unlock()
				}
				
				failedDBsMu.Lock()
				// Include more context in the error message
				failedDBs = append(failedDBs, fmt.Sprintf("%s: restore failed: %v", dbName, restoreErr))
				failedDBsMu.Unlock()
				atomic.AddInt32(&failCount, 1)
				return
			}

			atomic.AddInt32(&successCount, 1)
		}(dbIndex, entry.Name())

		dbIndex++
	}

	// Wait for all restores to complete
	wg.Wait()

	successCountFinal := int(atomic.LoadInt32(&successCount))
	failCountFinal := int(atomic.LoadInt32(&failCount))

	if failCountFinal > 0 {
		failedList := strings.Join(failedDBs, "\n  ")
		
		// Log summary
		e.log.Info("Cluster restore completed with failures",
			"succeeded", successCountFinal,
			"failed", failCountFinal,
			"total", totalDBs)
		
		e.progress.Fail(fmt.Sprintf("Cluster restore: %d succeeded, %d failed out of %d total", successCountFinal, failCountFinal, totalDBs))
		operation.Complete(fmt.Sprintf("Partial restore: %d/%d databases succeeded", successCountFinal, totalDBs))
		
		return fmt.Errorf("cluster restore completed with %d failures:\n  %s", failCountFinal, failedList)
	}

	e.progress.Complete(fmt.Sprintf("Cluster restored successfully: %d databases", successCountFinal))
	operation.Complete(fmt.Sprintf("Restored %d databases from cluster archive", successCountFinal))
	return nil
}

// extractArchive extracts a tar.gz archive
func (e *Engine) extractArchive(ctx context.Context, archivePath, destDir string) error {
	cmd := exec.CommandContext(ctx, "tar", "-xzf", archivePath, "-C", destDir)

	// Stream stderr to avoid memory issues - tar can produce lots of output for large archives
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tar: %w", err)
	}

	// Discard stderr output in chunks to prevent memory buildup
	buf := make([]byte, 4096)
	for {
		_, err := stderr.Read(buf)
		if err != nil {
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("tar extraction failed: %w", err)
	}
	return nil
}

// restoreGlobals restores global objects (roles, tablespaces)
func (e *Engine) restoreGlobals(ctx context.Context, globalsFile string) error {
	args := []string{
		"-p", fmt.Sprintf("%d", e.cfg.Port),
		"-U", e.cfg.User,
		"-d", "postgres",
		"-f", globalsFile,
	}

	// Only add -h flag if host is not localhost (to use Unix socket for peer auth)
	if e.cfg.Host != "localhost" && e.cfg.Host != "127.0.0.1" && e.cfg.Host != "" {
		args = append([]string{"-h", e.cfg.Host}, args...)
	}

	cmd := exec.CommandContext(ctx, "psql", args...)

	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", e.cfg.Password))

	// Stream output to avoid memory issues with large globals.sql files
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start psql: %w", err)
	}

	// Read stderr in chunks
	buf := make([]byte, 4096)
	var lastError string
	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			if strings.Contains(chunk, "ERROR") || strings.Contains(chunk, "FATAL") {
				lastError = chunk
				e.log.Warn("Globals restore stderr", "output", chunk)
			}
		}
		if err != nil {
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to restore globals: %w (last error: %s)", err, lastError)
	}

	return nil
}

// checkSuperuser verifies if the current user has superuser privileges
func (e *Engine) checkSuperuser(ctx context.Context) (bool, error) {
	args := []string{
		"-p", fmt.Sprintf("%d", e.cfg.Port),
		"-U", e.cfg.User,
		"-d", "postgres",
		"-tAc", "SELECT usesuper FROM pg_user WHERE usename = current_user",
	}

	// Only add -h flag if host is not localhost (to use Unix socket for peer auth)
	if e.cfg.Host != "localhost" && e.cfg.Host != "127.0.0.1" && e.cfg.Host != "" {
		args = append([]string{"-h", e.cfg.Host}, args...)
	}

	cmd := exec.CommandContext(ctx, "psql", args...)

	// Always set PGPASSWORD (empty string is fine for peer/ident auth)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", e.cfg.Password))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to check superuser status: %w", err)
	}

	isSuperuser := strings.TrimSpace(string(output)) == "t"
	return isSuperuser, nil
}

// terminateConnections kills all active connections to a database
func (e *Engine) terminateConnections(ctx context.Context, dbName string) error {
	query := fmt.Sprintf(`
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = '%s'
		AND pid <> pg_backend_pid()
	`, dbName)

	args := []string{
		"-p", fmt.Sprintf("%d", e.cfg.Port),
		"-U", e.cfg.User,
		"-d", "postgres",
		"-tAc", query,
	}

	// Only add -h flag if host is not localhost (to use Unix socket for peer auth)
	if e.cfg.Host != "localhost" && e.cfg.Host != "127.0.0.1" && e.cfg.Host != "" {
		args = append([]string{"-h", e.cfg.Host}, args...)
	}

	cmd := exec.CommandContext(ctx, "psql", args...)

	// Always set PGPASSWORD (empty string is fine for peer/ident auth)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", e.cfg.Password))

	output, err := cmd.CombinedOutput()
	if err != nil {
		e.log.Warn("Failed to terminate connections", "database", dbName, "error", err, "output", string(output))
		// Don't fail - database might not exist or have no connections
	}

	return nil
}

// dropDatabaseIfExists drops a database completely (clean slate)
func (e *Engine) dropDatabaseIfExists(ctx context.Context, dbName string) error {
	// First terminate all connections
	if err := e.terminateConnections(ctx, dbName); err != nil {
		e.log.Warn("Could not terminate connections", "database", dbName, "error", err)
	}

	// Wait a moment for connections to terminate
	time.Sleep(500 * time.Millisecond)

	// Drop the database
	args := []string{
		"-p", fmt.Sprintf("%d", e.cfg.Port),
		"-U", e.cfg.User,
		"-d", "postgres",
		"-c", fmt.Sprintf("DROP DATABASE IF EXISTS \"%s\"", dbName),
	}

	// Only add -h flag if host is not localhost (to use Unix socket for peer auth)
	if e.cfg.Host != "localhost" && e.cfg.Host != "127.0.0.1" && e.cfg.Host != "" {
		args = append([]string{"-h", e.cfg.Host}, args...)
	}

	cmd := exec.CommandContext(ctx, "psql", args...)

	// Always set PGPASSWORD (empty string is fine for peer/ident auth)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", e.cfg.Password))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to drop database '%s': %w\nOutput: %s", dbName, err, string(output))
	}

	e.log.Info("Dropped existing database", "name", dbName)
	return nil
}

// ensureDatabaseExists checks if a database exists and creates it if not
func (e *Engine) ensureDatabaseExists(ctx context.Context, dbName string) error {
	// Skip creation for postgres and template databases - they should already exist
	if dbName == "postgres" || dbName == "template0" || dbName == "template1" {
		e.log.Info("Skipping create for system database (assume exists)", "name", dbName)
		return nil
	}
	// Build psql command with authentication
	buildPsqlCmd := func(ctx context.Context, database, query string) *exec.Cmd {
		args := []string{
			"-p", fmt.Sprintf("%d", e.cfg.Port),
			"-U", e.cfg.User,
			"-d", database,
			"-tAc", query,
		}

		// Only add -h flag if host is not localhost (to use Unix socket for peer auth)
		if e.cfg.Host != "localhost" && e.cfg.Host != "127.0.0.1" && e.cfg.Host != "" {
			args = append([]string{"-h", e.cfg.Host}, args...)
		}

		cmd := exec.CommandContext(ctx, "psql", args...)

		// Always set PGPASSWORD (empty string is fine for peer/ident auth)
		cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", e.cfg.Password))

		return cmd
	}

	// Check if database exists
	checkCmd := buildPsqlCmd(ctx, "postgres", fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s'", dbName))

	output, err := checkCmd.CombinedOutput()
	if err != nil {
		e.log.Warn("Database existence check failed", "name", dbName, "error", err, "output", string(output))
		// Continue anyway - maybe we can create it
	}

	// If database exists, we're done
	if strings.TrimSpace(string(output)) == "1" {
		e.log.Info("Database already exists", "name", dbName)
		return nil
	}

	// Database doesn't exist, create it
	// IMPORTANT: Use template0 to avoid duplicate definition errors from local additions to template1
	// See PostgreSQL docs: https://www.postgresql.org/docs/current/app-pgrestore.html#APP-PGRESTORE-NOTES
	e.log.Info("Creating database from template0", "name", dbName)

	createArgs := []string{
		"-p", fmt.Sprintf("%d", e.cfg.Port),
		"-U", e.cfg.User,
		"-d", "postgres",
		"-c", fmt.Sprintf("CREATE DATABASE \"%s\" WITH TEMPLATE template0", dbName),
	}

	// Only add -h flag if host is not localhost (to use Unix socket for peer auth)
	if e.cfg.Host != "localhost" && e.cfg.Host != "127.0.0.1" && e.cfg.Host != "" {
		createArgs = append([]string{"-h", e.cfg.Host}, createArgs...)
	}

	createCmd := exec.CommandContext(ctx, "psql", createArgs...)

	// Always set PGPASSWORD (empty string is fine for peer/ident auth)
	createCmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", e.cfg.Password))

	output, err = createCmd.CombinedOutput()
	if err != nil {
		// Log the error and include the psql output in the returned error to aid debugging
		e.log.Warn("Database creation failed", "name", dbName, "error", err, "output", string(output))
		return fmt.Errorf("failed to create database '%s': %w (output: %s)", dbName, err, strings.TrimSpace(string(output)))
	}

	e.log.Info("Successfully created database from template0", "name", dbName)
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

// detectLargeObjectsInDumps checks if any dump files contain large objects
func (e *Engine) detectLargeObjectsInDumps(dumpsDir string, entries []os.DirEntry) bool {
	hasLargeObjects := false
	checkedCount := 0
	maxChecks := 5 // Only check first 5 dumps to avoid slowdown
	
	for _, entry := range entries {
		if entry.IsDir() || checkedCount >= maxChecks {
			continue
		}
		
		dumpFile := filepath.Join(dumpsDir, entry.Name())
		
		// Skip compressed SQL files (can't easily check without decompressing)
		if strings.HasSuffix(dumpFile, ".sql.gz") {
			continue
		}
		
		// Use pg_restore -l to list contents (fast, doesn't restore data)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		cmd := exec.CommandContext(ctx, "pg_restore", "-l", dumpFile)
		output, err := cmd.Output()
		
		if err != nil {
			// If pg_restore -l fails, it might not be custom format - skip
			continue
		}
		
		checkedCount++
		
		// Check if output contains "BLOB" or "LARGE OBJECT" entries
		outputStr := string(output)
		if strings.Contains(outputStr, "BLOB") || 
		   strings.Contains(outputStr, "LARGE OBJECT") ||
		   strings.Contains(outputStr, " BLOBS ") {
			e.log.Info("Large objects detected in dump file", "file", entry.Name())
			hasLargeObjects = true
			// Don't break - log all files with large objects
		}
	}
	
	if hasLargeObjects {
		e.log.Warn("Cluster contains databases with large objects - parallel restore may cause lock contention")
	}
	
	return hasLargeObjects
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
