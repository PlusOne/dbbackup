package cmd

import (
	"context"
	"fmt"

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
	
	log.Info("Starting cluster backup", 
		"host", cfg.Host, 
		"port", cfg.Port,
		"backup_dir", cfg.BackupDir)
	
	// Audit log: backup start
	user := security.GetCurrentUser()
	auditLogger.LogBackupStart(user, "all_databases", "cluster")
	
	// Create database instance
	db, err := database.New(cfg, log)
	if err != nil {
		auditLogger.LogBackupFailed(user, "all_databases", err)
		return fmt.Errorf("failed to create database instance: %w", err)
	}
	defer db.Close()
	
	// Connect to database
	if err := db.Connect(ctx); err != nil {
		auditLogger.LogBackupFailed(user, "all_databases", err)
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	
	// Create backup engine
	engine := backup.New(cfg, log, db)
	
	// Perform cluster backup
	if err := engine.BackupCluster(ctx); err != nil {
		auditLogger.LogBackupFailed(user, "all_databases", err)
		return err
	}
	
	// Audit log: backup success
	auditLogger.LogBackupComplete(user, "all_databases", cfg.BackupDir, 0)
	
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
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	
	log.Info("Starting single database backup", 
		"database", databaseName,
		"db_type", cfg.DatabaseType,
		"host", cfg.Host, 
		"port", cfg.Port,
		"backup_dir", cfg.BackupDir)
	
	// Audit log: backup start
	user := security.GetCurrentUser()
	auditLogger.LogBackupStart(user, databaseName, "single")
	
	// Create database instance
	db, err := database.New(cfg, log)
	if err != nil {
		auditLogger.LogBackupFailed(user, databaseName, err)
		return fmt.Errorf("failed to create database instance: %w", err)
	}
	defer db.Close()
	
	// Connect to database
	if err := db.Connect(ctx); err != nil {
		auditLogger.LogBackupFailed(user, databaseName, err)
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	
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
	
	// Perform single database backup
	if err := engine.BackupSingle(ctx, databaseName); err != nil {
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

// runSampleBackup performs a sample database backup
func runSampleBackup(ctx context.Context, databaseName string) error {
	// Update config from environment
	cfg.UpdateFromEnvironment()
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
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
	
	// Create database instance
	db, err := database.New(cfg, log)
	if err != nil {
		auditLogger.LogBackupFailed(user, databaseName, err)
		return fmt.Errorf("failed to create database instance: %w", err)
	}
	defer db.Close()
	
	// Connect to database
	if err := db.Connect(ctx); err != nil {
		auditLogger.LogBackupFailed(user, databaseName, err)
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	
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