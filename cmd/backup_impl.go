package cmd

import (
	"context"
	"fmt"
	"os"

	"dbbackup/internal/backup"
	"dbbackup/internal/config"
	"dbbackup/internal/database"
	"dbbackup/internal/security"
)

// runClusterBackup performs a full cluster backup
func runClusterBackup(ctx context.Context) error {
	if !cfg.IsPostgreSQL() {
		return fmt.Errorf("cluster backup is only supported for PostgreSQL")
	}
	
	// Update config from environment
	cfg.UpdateFromEnvironment()
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	
	// Check privileges
	privChecker := security.NewPrivilegeChecker(log)
	if err := privChecker.CheckAndWarn(cfg.AllowRoot); err != nil {
		return err
	}
	
	// Check resource limits
	if cfg.CheckResources {
		resChecker := security.NewResourceChecker(log)
		if _, err := resChecker.CheckResourceLimits(); err != nil {
			log.Warn("Failed to check resource limits", "error", err)
		}
	}
	
	log.Info("Starting cluster backup", 
		"host", cfg.Host, 
		"port", cfg.Port,
		"backup_dir", cfg.BackupDir)
	
	// Audit log: backup start
	user := security.GetCurrentUser()
	auditLogger.LogBackupStart(user, "all_databases", "cluster")
	
	// Rate limit connection attempts
	host := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	if err := rateLimiter.CheckAndWait(host); err != nil {
		auditLogger.LogBackupFailed(user, "all_databases", err)
		return fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Create database instance
	db, err := database.New(cfg, log)
	if err != nil {
		auditLogger.LogBackupFailed(user, "all_databases", err)
		return fmt.Errorf("failed to create database instance: %w", err)
	}
	defer db.Close()
	
	// Connect to database
	if err := db.Connect(ctx); err != nil {
		rateLimiter.RecordFailure(host)
		auditLogger.LogBackupFailed(user, "all_databases", err)
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	rateLimiter.RecordSuccess(host)
	
	// Create backup engine
	engine := backup.New(cfg, log, db)
	
	// Perform cluster backup
	if err := engine.BackupCluster(ctx); err != nil {
		auditLogger.LogBackupFailed(user, "all_databases", err)
		return err
	}
	
	// Audit log: backup success
	auditLogger.LogBackupComplete(user, "all_databases", cfg.BackupDir, 0)
	
	// Cleanup old backups if retention policy is enabled
	if cfg.RetentionDays > 0 {
		retentionPolicy := security.NewRetentionPolicy(cfg.RetentionDays, cfg.MinBackups, log)
		if deleted, freed, err := retentionPolicy.CleanupOldBackups(cfg.BackupDir); err != nil {
			log.Warn("Failed to cleanup old backups", "error", err)
		} else if deleted > 0 {
			log.Info("Cleaned up old backups", "deleted", deleted, "freed_mb", freed/1024/1024)
		}
	}
	
	// Save configuration for future use (unless disabled)
	if !cfg.NoSaveConfig {
		localCfg := config.ConfigFromConfig(cfg)
		if err := config.SaveLocalConfig(localCfg); err != nil {
			log.Warn("Failed to save configuration", "error", err)
		} else {
			log.Info("Configuration saved to .dbbackup.conf")
			auditLogger.LogConfigChange(user, "config_file", "", ".dbbackup.conf")
		}
	}
	
	return nil
}

// runSingleBackup performs a single database backup
func runSingleBackup(ctx context.Context, databaseName string) error {
	// Update config from environment
	cfg.UpdateFromEnvironment()
	
	// Get backup type and base backup from environment variables (set by PreRunE)
	// For now, incremental is just scaffolding - actual implementation comes next
	backupType := "full"  // TODO: Read from flag via global var in cmd/backup.go
	baseBackup := ""      // TODO: Read from flag via global var in cmd/backup.go
	
	// Validate backup type
	if backupType != "full" && backupType != "incremental" {
		return fmt.Errorf("invalid backup type: %s (must be 'full' or 'incremental')", backupType)
	}
	
	// Validate incremental backup requirements
	if backupType == "incremental" {
		if !cfg.IsPostgreSQL() {
			return fmt.Errorf("incremental backups are currently only supported for PostgreSQL")
		}
		if baseBackup == "" {
			return fmt.Errorf("--base-backup is required for incremental backups")
		}
		// Verify base backup exists
		if _, err := os.Stat(baseBackup); os.IsNotExist(err) {
			return fmt.Errorf("base backup not found: %s", baseBackup)
		}
	}
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	
	// Check privileges
	privChecker := security.NewPrivilegeChecker(log)
	if err := privChecker.CheckAndWarn(cfg.AllowRoot); err != nil {
		return err
	}
	
	log.Info("Starting single database backup", 
		"database", databaseName,
		"db_type", cfg.DatabaseType,
		"backup_type", backupType,
		"host", cfg.Host, 
		"port", cfg.Port,
		"backup_dir", cfg.BackupDir)
	
	if backupType == "incremental" {
		log.Info("Incremental backup", "base_backup", baseBackup)
	}
	
	// Audit log: backup start
	user := security.GetCurrentUser()
	auditLogger.LogBackupStart(user, databaseName, "single")
	
	// Rate limit connection attempts
	host := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	if err := rateLimiter.CheckAndWait(host); err != nil {
		auditLogger.LogBackupFailed(user, databaseName, err)
		return fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Create database instance
	db, err := database.New(cfg, log)
	if err != nil {
		auditLogger.LogBackupFailed(user, databaseName, err)
		return fmt.Errorf("failed to create database instance: %w", err)
	}
	defer db.Close()
	
	// Connect to database
	if err := db.Connect(ctx); err != nil {
		rateLimiter.RecordFailure(host)
		auditLogger.LogBackupFailed(user, databaseName, err)
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	rateLimiter.RecordSuccess(host)
	
	// Verify database exists
	exists, err := db.DatabaseExists(ctx, databaseName)
	if err != nil {
		auditLogger.LogBackupFailed(user, databaseName, err)
		return fmt.Errorf("failed to check if database exists: %w", err)
	}
	if !exists {
		err := fmt.Errorf("database '%s' does not exist", databaseName)
		auditLogger.LogBackupFailed(user, databaseName, err)
		return err
	}
	
	// Create backup engine
	engine := backup.New(cfg, log, db)
	
	// Perform backup based on type
	var backupErr error
	if backupType == "incremental" {
		// Incremental backup - NOT IMPLEMENTED YET
		log.Warn("Incremental backup is not fully implemented yet - creating full backup instead")
		log.Warn("Full incremental support coming in v2.2.1")
		backupErr = engine.BackupSingle(ctx, databaseName)
	} else {
		// Full backup
		backupErr = engine.BackupSingle(ctx, databaseName)
	}
	
	if backupErr != nil {
		auditLogger.LogBackupFailed(user, databaseName, backupErr)
		return backupErr
	}
	
	// Audit log: backup success
	auditLogger.LogBackupComplete(user, databaseName, cfg.BackupDir, 0)
	
	// Cleanup old backups if retention policy is enabled
	if cfg.RetentionDays > 0 {
		retentionPolicy := security.NewRetentionPolicy(cfg.RetentionDays, cfg.MinBackups, log)
		if deleted, freed, err := retentionPolicy.CleanupOldBackups(cfg.BackupDir); err != nil {
			log.Warn("Failed to cleanup old backups", "error", err)
		} else if deleted > 0 {
			log.Info("Cleaned up old backups", "deleted", deleted, "freed_mb", freed/1024/1024)
		}
	}
	
	// Save configuration for future use (unless disabled)
	if !cfg.NoSaveConfig {
		localCfg := config.ConfigFromConfig(cfg)
		if err := config.SaveLocalConfig(localCfg); err != nil {
			log.Warn("Failed to save configuration", "error", err)
		} else {
			log.Info("Configuration saved to .dbbackup.conf")
			auditLogger.LogConfigChange(user, "config_file", "", ".dbbackup.conf")
		}
	}
	
	return nil
}

// runSampleBackup performs a sample database backup
func runSampleBackup(ctx context.Context, databaseName string) error {
	// Update config from environment
	cfg.UpdateFromEnvironment()
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	
	// Check privileges
	privChecker := security.NewPrivilegeChecker(log)
	if err := privChecker.CheckAndWarn(cfg.AllowRoot); err != nil {
		return err
	}
	
	// Validate sample parameters
	if cfg.SampleValue <= 0 {
		return fmt.Errorf("sample value must be greater than 0")
	}
	
	switch cfg.SampleStrategy {
	case "percent":
		if cfg.SampleValue > 100 {
			return fmt.Errorf("percentage cannot exceed 100")
		}
	case "ratio":
		if cfg.SampleValue < 2 {
			return fmt.Errorf("ratio must be at least 2")
		}
	case "count":
		// Any positive count is valid
	default:
		return fmt.Errorf("invalid sampling strategy: %s (must be ratio, percent, or count)", cfg.SampleStrategy)
	}
	
	log.Info("Starting sample database backup", 
		"database", databaseName,
		"db_type", cfg.DatabaseType,
		"strategy", cfg.SampleStrategy,
		"value", cfg.SampleValue,
		"host", cfg.Host, 
		"port", cfg.Port,
		"backup_dir", cfg.BackupDir)
	
	// Audit log: backup start
	user := security.GetCurrentUser()
	auditLogger.LogBackupStart(user, databaseName, "sample")
	
	// Rate limit connection attempts
	host := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	if err := rateLimiter.CheckAndWait(host); err != nil {
		auditLogger.LogBackupFailed(user, databaseName, err)
		return fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Create database instance
	db, err := database.New(cfg, log)
	if err != nil {
		auditLogger.LogBackupFailed(user, databaseName, err)
		return fmt.Errorf("failed to create database instance: %w", err)
	}
	defer db.Close()
	
	// Connect to database
	if err := db.Connect(ctx); err != nil {
		rateLimiter.RecordFailure(host)
		auditLogger.LogBackupFailed(user, databaseName, err)
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	rateLimiter.RecordSuccess(host)
	
	// Verify database exists
	exists, err := db.DatabaseExists(ctx, databaseName)
	if err != nil {
		auditLogger.LogBackupFailed(user, databaseName, err)
		return fmt.Errorf("failed to check if database exists: %w", err)
	}
	if !exists {
		err := fmt.Errorf("database '%s' does not exist", databaseName)
		auditLogger.LogBackupFailed(user, databaseName, err)
		return err
	}
	
	// Create backup engine
	engine := backup.New(cfg, log, db)
	
	// Perform sample backup
	if err := engine.BackupSample(ctx, databaseName); err != nil {
		auditLogger.LogBackupFailed(user, databaseName, err)
		return err
	}
	
	// Audit log: backup success
	auditLogger.LogBackupComplete(user, databaseName, cfg.BackupDir, 0)
	
	// Save configuration for future use (unless disabled)
	if !cfg.NoSaveConfig {
		localCfg := config.ConfigFromConfig(cfg)
		if err := config.SaveLocalConfig(localCfg); err != nil {
			log.Warn("Failed to save configuration", "error", err)
		} else {
			log.Info("Configuration saved to .dbbackup.conf")
			auditLogger.LogConfigChange(user, "config_file", "", ".dbbackup.conf")
		}
	}
	
	return nil
}