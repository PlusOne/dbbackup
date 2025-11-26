package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dbbackup/internal/backup"
	"dbbackup/internal/config"
	"dbbackup/internal/database"
	"dbbackup/internal/security"
)

// runClusterBackup performs a full cluster backup
func runClusterBackup(ctx context.Context) error {
	if !cfg.IsPostgreSQL() {
		return fmt.Errorf("cluster backup requires PostgreSQL (detected: %s). Use 'backup single' for individual database backups", cfg.DisplayDatabaseType())
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
		return fmt.Errorf("rate limit exceeded for %s. Too many connection attempts. Wait 60s or check credentials: %w", host, err)
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
		return fmt.Errorf("failed to connect to %s@%s:%d. Check: 1) Database is running 2) Credentials are correct 3) pg_hba.conf allows connection: %w", cfg.User, cfg.Host, cfg.Port, err)
	}
	rateLimiter.RecordSuccess(host)
	
	// Create backup engine
	engine := backup.New(cfg, log, db)
	
	// Perform cluster backup
	if err := engine.BackupCluster(ctx); err != nil {
		auditLogger.LogBackupFailed(user, "all_databases", err)
		return err
	}
	
	// Apply encryption if requested
	if isEncryptionEnabled() {
		if err := encryptLatestClusterBackup(); err != nil {
			log.Error("Failed to encrypt backup", "error", err)
			return fmt.Errorf("backup completed successfully but encryption failed. Unencrypted backup remains in %s: %w", cfg.BackupDir, err)
		}
		log.Info("Cluster backup encrypted successfully")
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
	
	// Get backup type and base backup from command line flags (set via global vars in PreRunE)
	// These are populated by cobra flag binding in cmd/backup.go
	backupType := "full"  // Default to full backup if not specified
	baseBackup := ""      // Base backup path for incremental backups
	
	// Validate backup type
	if backupType != "full" && backupType != "incremental" {
		return fmt.Errorf("invalid backup type: %s (must be 'full' or 'incremental')", backupType)
	}
	
	// Validate incremental backup requirements
	if backupType == "incremental" {
		if !cfg.IsPostgreSQL() && !cfg.IsMySQL() {
			return fmt.Errorf("incremental backups require PostgreSQL or MySQL/MariaDB (detected: %s). Use --backup-type=full for other databases", cfg.DisplayDatabaseType())
		}
		if baseBackup == "" {
			return fmt.Errorf("incremental backup requires --base-backup flag pointing to initial full backup archive")
		}
		// Verify base backup exists
		if _, err := os.Stat(baseBackup); os.IsNotExist(err) {
			return fmt.Errorf("base backup file not found at %s. Ensure path is correct and file exists", baseBackup)
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
		// Incremental backup - supported for PostgreSQL and MySQL
		log.Info("Creating incremental backup", "base_backup", baseBackup)
		
		// Create appropriate incremental engine based on database type
		var incrEngine interface {
			FindChangedFiles(context.Context, *backup.IncrementalBackupConfig) ([]backup.ChangedFile, error)
			CreateIncrementalBackup(context.Context, *backup.IncrementalBackupConfig, []backup.ChangedFile) error
		}
		
		if cfg.IsPostgreSQL() {
			incrEngine = backup.NewPostgresIncrementalEngine(log)
		} else {
			incrEngine = backup.NewMySQLIncrementalEngine(log)
		}
		
		// Configure incremental backup
		incrConfig := &backup.IncrementalBackupConfig{
			BaseBackupPath:   baseBackup,
			DataDirectory:    cfg.BackupDir, // Note: This should be the actual data directory
			CompressionLevel: cfg.CompressionLevel,
		}
		
		// Find changed files
		changedFiles, err := incrEngine.FindChangedFiles(ctx, incrConfig)
		if err != nil {
			return fmt.Errorf("failed to find changed files: %w", err)
		}
		
		// Create incremental backup
		if err := incrEngine.CreateIncrementalBackup(ctx, incrConfig, changedFiles); err != nil {
			return fmt.Errorf("failed to create incremental backup: %w", err)
		}
		
		log.Info("Incremental backup completed", "changed_files", len(changedFiles))
	} else {
		// Full backup
		backupErr = engine.BackupSingle(ctx, databaseName)
	}
	
	if backupErr != nil {
		auditLogger.LogBackupFailed(user, databaseName, backupErr)
		return backupErr
	}
	
	// Apply encryption if requested
	if isEncryptionEnabled() {
		if err := encryptLatestBackup(databaseName); err != nil {
			log.Error("Failed to encrypt backup", "error", err)
			return fmt.Errorf("backup succeeded but encryption failed: %w", err)
		}
		log.Info("Backup encrypted successfully")
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
	
	// Apply encryption if requested
	if isEncryptionEnabled() {
		if err := encryptLatestBackup(databaseName); err != nil {
			log.Error("Failed to encrypt backup", "error", err)
			return fmt.Errorf("backup succeeded but encryption failed: %w", err)
		}
		log.Info("Sample backup encrypted successfully")
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
// encryptLatestBackup finds and encrypts the most recent backup for a database
func encryptLatestBackup(databaseName string) error {
	// Load encryption key
	key, err := loadEncryptionKey(encryptionKeyFile, encryptionKeyEnv)
	if err != nil {
		return err
	}

	// Find most recent backup file for this database
	backupPath, err := findLatestBackup(cfg.BackupDir, databaseName)
	if err != nil {
		return err
	}

	// Encrypt the backup
	return backup.EncryptBackupFile(backupPath, key, log)
}

// encryptLatestClusterBackup finds and encrypts the most recent cluster backup
func encryptLatestClusterBackup() error {
	// Load encryption key
	key, err := loadEncryptionKey(encryptionKeyFile, encryptionKeyEnv)
	if err != nil {
		return err
	}

	// Find most recent cluster backup
	backupPath, err := findLatestClusterBackup(cfg.BackupDir)
	if err != nil {
		return err
	}

	// Encrypt the backup
	return backup.EncryptBackupFile(backupPath, key, log)
}

// findLatestBackup finds the most recently created backup file for a database
func findLatestBackup(backupDir, databaseName string) (string, error) {
entries, err := os.ReadDir(backupDir)
if err != nil {
return "", fmt.Errorf("failed to read backup directory: %w", err)
}

var latestPath string
var latestTime time.Time

prefix := "db_" + databaseName + "_"
for _, entry := range entries {
if entry.IsDir() {
continue
}

name := entry.Name()
// Skip metadata files and already encrypted files
if strings.HasSuffix(name, ".meta.json") || strings.HasSuffix(name, ".encrypted") {
continue
}

// Match database backup files
if strings.HasPrefix(name, prefix) && (strings.HasSuffix(name, ".dump") || 
strings.HasSuffix(name, ".dump.gz") || strings.HasSuffix(name, ".sql.gz")) {
info, err := entry.Info()
if err != nil {
continue
}

if info.ModTime().After(latestTime) {
latestTime = info.ModTime()
latestPath = filepath.Join(backupDir, name)
}
}
}

if latestPath == "" {
return "", fmt.Errorf("no backup found for database: %s", databaseName)
}

return latestPath, nil
}

// findLatestClusterBackup finds the most recently created cluster backup
func findLatestClusterBackup(backupDir string) (string, error) {
entries, err := os.ReadDir(backupDir)
if err != nil {
return "", fmt.Errorf("failed to read backup directory: %w", err)
}

var latestPath string
var latestTime time.Time

for _, entry := range entries {
if entry.IsDir() {
continue
}

name := entry.Name()
// Skip metadata files and already encrypted files
if strings.HasSuffix(name, ".meta.json") || strings.HasSuffix(name, ".encrypted") {
continue
}

// Match cluster backup files
if strings.HasPrefix(name, "cluster_") && strings.HasSuffix(name, ".tar.gz") {
info, err := entry.Info()
if err != nil {
continue
}

if info.ModTime().After(latestTime) {
latestTime = info.ModTime()
latestPath = filepath.Join(backupDir, name)
}
}
}

if latestPath == "" {
return "", fmt.Errorf("no cluster backup found")
}

return latestPath, nil
}
