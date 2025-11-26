package pitr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
)

// RestoreOrchestrator orchestrates Point-in-Time Recovery operations
type RestoreOrchestrator struct {
	log       logger.Logger
	config    *config.Config
	configGen *RecoveryConfigGenerator
}

// NewRestoreOrchestrator creates a new PITR restore orchestrator
func NewRestoreOrchestrator(cfg *config.Config, log logger.Logger) *RestoreOrchestrator {
	return &RestoreOrchestrator{
		log:       log,
		config:    cfg,
		configGen: NewRecoveryConfigGenerator(log),
	}
}

// RestoreOptions holds options for PITR restore
type RestoreOptions struct {
	BaseBackupPath  string          // Path to base backup file (.tar.gz, .sql, or directory)
	WALArchiveDir   string          // Path to WAL archive directory
	Target          *RecoveryTarget // Recovery target
	TargetDataDir   string          // PostgreSQL data directory to restore to
	PostgreSQLBin   string          // Path to PostgreSQL binaries (optional, will auto-detect)
	SkipExtraction  bool            // Skip base backup extraction (data dir already exists)
	AutoStart       bool            // Automatically start PostgreSQL after recovery
	MonitorProgress bool            // Monitor recovery progress
}

// RestorePointInTime performs a Point-in-Time Recovery
func (ro *RestoreOrchestrator) RestorePointInTime(ctx context.Context, opts *RestoreOptions) error {
	ro.log.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	ro.log.Info("  Point-in-Time Recovery (PITR)")
	ro.log.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	ro.log.Info("")
	ro.log.Info("Target:", "summary", opts.Target.Summary())
	ro.log.Info("Base Backup:", "path", opts.BaseBackupPath)
	ro.log.Info("WAL Archive:", "path", opts.WALArchiveDir)
	ro.log.Info("Data Directory:", "path", opts.TargetDataDir)
	ro.log.Info("")

	// Step 1: Validate inputs
	if err := ro.validateInputs(opts); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Step 2: Extract base backup (if needed)
	if !opts.SkipExtraction {
		if err := ro.extractBaseBackup(ctx, opts); err != nil {
			return fmt.Errorf("base backup extraction failed: %w", err)
		}
	} else {
		ro.log.Info("Skipping base backup extraction (--skip-extraction)")
	}

	// Step 3: Detect PostgreSQL version
	pgVersion, err := ro.configGen.DetectPostgreSQLVersion(opts.TargetDataDir)
	if err != nil {
		return fmt.Errorf("failed to detect PostgreSQL version: %w", err)
	}
	ro.log.Info("PostgreSQL version detected", "version", pgVersion)

	// Step 4: Backup existing recovery config (if any)
	if err := ro.configGen.BackupExistingConfig(opts.TargetDataDir); err != nil {
		ro.log.Warn("Failed to backup existing recovery config", "error", err)
	}

	// Step 5: Generate recovery configuration
	recoveryConfig := &RecoveryConfig{
		Target:            opts.Target,
		WALArchiveDir:     opts.WALArchiveDir,
		PostgreSQLVersion: pgVersion,
		DataDir:           opts.TargetDataDir,
	}

	if err := ro.configGen.GenerateRecoveryConfig(recoveryConfig); err != nil {
		return fmt.Errorf("failed to generate recovery configuration: %w", err)
	}

	ro.log.Info("✅ Recovery configuration generated successfully")
	ro.log.Info("")
	ro.log.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	ro.log.Info("  Next Steps:")
	ro.log.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	ro.log.Info("")
	ro.log.Info("1. Start PostgreSQL to begin recovery:")
	ro.log.Info(fmt.Sprintf("   pg_ctl -D %s start", opts.TargetDataDir))
	ro.log.Info("")
	ro.log.Info("2. Monitor recovery progress:")
	ro.log.Info("   tail -f " + filepath.Join(opts.TargetDataDir, "log", "postgresql-*.log"))
	ro.log.Info("   OR query: SELECT * FROM pg_stat_recovery_prefetch;")
	ro.log.Info("")
	ro.log.Info("3. After recovery completes:")
	ro.log.Info(fmt.Sprintf("   - Action: %s", opts.Target.Action))
	if opts.Target.Action == ActionPromote {
		ro.log.Info("   - PostgreSQL will automatically promote to primary")
	} else if opts.Target.Action == ActionPause {
		ro.log.Info("   - PostgreSQL will pause - manually promote with: pg_ctl promote")
	}
	ro.log.Info("")
	ro.log.Info("Recovery configuration ready!")
	ro.log.Info("")

	// Optional: Auto-start PostgreSQL
	if opts.AutoStart {
		if err := ro.startPostgreSQL(ctx, opts); err != nil {
			ro.log.Error("Failed to start PostgreSQL", "error", err)
			return fmt.Errorf("PostgreSQL startup failed: %w", err)
		}

		// Optional: Monitor recovery
		if opts.MonitorProgress {
			if err := ro.monitorRecovery(ctx, opts); err != nil {
				ro.log.Warn("Recovery monitoring encountered an issue", "error", err)
			}
		}
	}

	return nil
}

// validateInputs validates restore options
func (ro *RestoreOrchestrator) validateInputs(opts *RestoreOptions) error {
	ro.log.Info("Validating restore options...")

	// Validate target
	if opts.Target == nil {
		return fmt.Errorf("recovery target not specified")
	}
	if err := opts.Target.Validate(); err != nil {
		return fmt.Errorf("invalid recovery target: %w", err)
	}

	// Validate base backup path
	if !opts.SkipExtraction {
		if opts.BaseBackupPath == "" {
			return fmt.Errorf("base backup path not specified")
		}
		if _, err := os.Stat(opts.BaseBackupPath); err != nil {
			return fmt.Errorf("base backup not found: %w", err)
		}
	}

	// Validate WAL archive directory
	if opts.WALArchiveDir == "" {
		return fmt.Errorf("WAL archive directory not specified")
	}
	if stat, err := os.Stat(opts.WALArchiveDir); err != nil {
		return fmt.Errorf("WAL archive directory not accessible: %w", err)
	} else if !stat.IsDir() {
		return fmt.Errorf("WAL archive path is not a directory: %s", opts.WALArchiveDir)
	}

	// Validate target data directory
	if opts.TargetDataDir == "" {
		return fmt.Errorf("target data directory not specified")
	}

	// If not skipping extraction, target dir should not exist or be empty
	if !opts.SkipExtraction {
		if stat, err := os.Stat(opts.TargetDataDir); err == nil {
			if stat.IsDir() {
				entries, err := os.ReadDir(opts.TargetDataDir)
				if err != nil {
					return fmt.Errorf("failed to read target directory: %w", err)
				}
				if len(entries) > 0 {
					return fmt.Errorf("target data directory is not empty: %s (use --skip-extraction if intentional)", opts.TargetDataDir)
				}
			} else {
				return fmt.Errorf("target path exists but is not a directory: %s", opts.TargetDataDir)
			}
		}
	} else {
		// If skipping extraction, validate the data directory
		if err := ro.configGen.ValidateDataDirectory(opts.TargetDataDir); err != nil {
			return err
		}
	}

	ro.log.Info("✅ Validation passed")
	return nil
}

// extractBaseBackup extracts the base backup to the target directory
func (ro *RestoreOrchestrator) extractBaseBackup(ctx context.Context, opts *RestoreOptions) error {
	ro.log.Info("Extracting base backup...", "source", opts.BaseBackupPath, "dest", opts.TargetDataDir)

	// Create target directory
	if err := os.MkdirAll(opts.TargetDataDir, 0700); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Determine backup format and extract
	backupPath := opts.BaseBackupPath

	// Check if encrypted
	if strings.HasSuffix(backupPath, ".enc") {
		ro.log.Info("Backup is encrypted - decryption not yet implemented in PITR module")
		return fmt.Errorf("encrypted backups not yet supported for PITR restore (use manual decryption)")
	}

	// Check format
	if strings.HasSuffix(backupPath, ".tar.gz") || strings.HasSuffix(backupPath, ".tgz") {
		return ro.extractTarGzBackup(ctx, backupPath, opts.TargetDataDir)
	} else if strings.HasSuffix(backupPath, ".tar") {
		return ro.extractTarBackup(ctx, backupPath, opts.TargetDataDir)
	} else if stat, err := os.Stat(backupPath); err == nil && stat.IsDir() {
		return ro.copyDirectoryBackup(ctx, backupPath, opts.TargetDataDir)
	}

	return fmt.Errorf("unsupported backup format: %s (expected .tar.gz, .tar, or directory)", backupPath)
}

// extractTarGzBackup extracts a .tar.gz backup
func (ro *RestoreOrchestrator) extractTarGzBackup(ctx context.Context, source, dest string) error {
	ro.log.Info("Extracting tar.gz backup...")

	cmd := exec.CommandContext(ctx, "tar", "-xzf", source, "-C", dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tar extraction failed: %w", err)
	}

	ro.log.Info("✅ Base backup extracted successfully")
	return nil
}

// extractTarBackup extracts a .tar backup
func (ro *RestoreOrchestrator) extractTarBackup(ctx context.Context, source, dest string) error {
	ro.log.Info("Extracting tar backup...")

	cmd := exec.CommandContext(ctx, "tar", "-xf", source, "-C", dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tar extraction failed: %w", err)
	}

	ro.log.Info("✅ Base backup extracted successfully")
	return nil
}

// copyDirectoryBackup copies a directory backup
func (ro *RestoreOrchestrator) copyDirectoryBackup(ctx context.Context, source, dest string) error {
	ro.log.Info("Copying directory backup...")

	cmd := exec.CommandContext(ctx, "cp", "-a", source+"/.", dest+"/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("directory copy failed: %w", err)
	}

	ro.log.Info("✅ Base backup copied successfully")
	return nil
}

// startPostgreSQL starts PostgreSQL server
func (ro *RestoreOrchestrator) startPostgreSQL(ctx context.Context, opts *RestoreOptions) error {
	ro.log.Info("Starting PostgreSQL for recovery...")

	pgCtl := "pg_ctl"
	if opts.PostgreSQLBin != "" {
		pgCtl = filepath.Join(opts.PostgreSQLBin, "pg_ctl")
	}

	cmd := exec.CommandContext(ctx, pgCtl, "-D", opts.TargetDataDir, "-l", filepath.Join(opts.TargetDataDir, "logfile"), "start")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		ro.log.Error("PostgreSQL startup failed", "output", string(output))
		return fmt.Errorf("pg_ctl start failed: %w", err)
	}

	ro.log.Info("✅ PostgreSQL started successfully")
	ro.log.Info("PostgreSQL is now performing recovery...")
	return nil
}

// monitorRecovery monitors recovery progress
func (ro *RestoreOrchestrator) monitorRecovery(ctx context.Context, opts *RestoreOptions) error {
	ro.log.Info("Monitoring recovery progress...")
	ro.log.Info("(This is a simplified monitor - check PostgreSQL logs for detailed progress)")

	// Monitor for up to 5 minutes or until context cancelled
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			ro.log.Info("Monitoring cancelled")
			return ctx.Err()
		case <-timeout:
			ro.log.Info("Monitoring timeout reached (5 minutes)")
			ro.log.Info("Recovery may still be in progress - check PostgreSQL logs")
			return nil
		case <-ticker.C:
			// Check if recovery is complete by looking for postmaster.pid
			pidFile := filepath.Join(opts.TargetDataDir, "postmaster.pid")
			if _, err := os.Stat(pidFile); err == nil {
				ro.log.Info("✅ PostgreSQL is running")
				
				// Check if recovery files still exist
				recoverySignal := filepath.Join(opts.TargetDataDir, "recovery.signal")
				recoveryConf := filepath.Join(opts.TargetDataDir, "recovery.conf")
				
				if _, err := os.Stat(recoverySignal); os.IsNotExist(err) {
					if _, err := os.Stat(recoveryConf); os.IsNotExist(err) {
						ro.log.Info("✅ Recovery completed - PostgreSQL promoted to primary")
						return nil
					}
				}
				
				ro.log.Info("Recovery in progress...")
			} else {
				ro.log.Info("PostgreSQL not yet started or crashed")
			}
		}
	}
}

// GetRecoveryStatus checks the current recovery status
func (ro *RestoreOrchestrator) GetRecoveryStatus(dataDir string) (string, error) {
	// Check for recovery signal files
	recoverySignal := filepath.Join(dataDir, "recovery.signal")
	standbySignal := filepath.Join(dataDir, "standby.signal")
	recoveryConf := filepath.Join(dataDir, "recovery.conf")
	postmasterPid := filepath.Join(dataDir, "postmaster.pid")

	// Check if PostgreSQL is running
	_, pgRunning := os.Stat(postmasterPid)

	if _, err := os.Stat(recoverySignal); err == nil {
		if pgRunning == nil {
			return "recovering", nil
		}
		return "recovery_configured", nil
	}

	if _, err := os.Stat(standbySignal); err == nil {
		if pgRunning == nil {
			return "standby", nil
		}
		return "standby_configured", nil
	}

	if _, err := os.Stat(recoveryConf); err == nil {
		if pgRunning == nil {
			return "recovering_legacy", nil
		}
		return "recovery_configured_legacy", nil
	}

	if pgRunning == nil {
		return "primary", nil
	}

	return "not_configured", nil
}
