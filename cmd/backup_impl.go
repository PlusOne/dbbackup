package cmd

import (
	"context"
	"fmt"

	"dbbackup/internal/backup"
	"dbbackup/internal/database"
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
	
	// Create database instance
	db, err := database.New(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create database instance: %w", err)
	}
	defer db.Close()
	
	// Connect to database
	if err := db.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	
	// Create backup engine
	engine := backup.New(cfg, log, db)
	
	// Perform cluster backup
	return engine.BackupCluster(ctx)
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
	
	// Create database instance
	db, err := database.New(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create database instance: %w", err)
	}
	defer db.Close()
	
	// Connect to database
	if err := db.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	
	// Verify database exists
	exists, err := db.DatabaseExists(ctx, databaseName)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("database '%s' does not exist", databaseName)
	}
	
	// Create backup engine
	engine := backup.New(cfg, log, db)
	
	// Perform single database backup
	return engine.BackupSingle(ctx, databaseName)
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
	
	// Create database instance
	db, err := database.New(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create database instance: %w", err)
	}
	defer db.Close()
	
	// Connect to database
	if err := db.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	
	// Verify database exists
	exists, err := db.DatabaseExists(ctx, databaseName)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("database '%s' does not exist", databaseName)
	}
	
	// Create backup engine
	engine := backup.New(cfg, log, db)
	
	// Perform sample database backup
	return engine.BackupSample(ctx, databaseName)
}