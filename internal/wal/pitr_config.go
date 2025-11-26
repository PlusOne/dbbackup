package wal

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
)

// PITRManager manages Point-in-Time Recovery configuration
type PITRManager struct {
	cfg *config.Config
	log logger.Logger
}

// PITRConfig holds PITR settings
type PITRConfig struct {
	Enabled           bool
	ArchiveMode       string // "on", "off", "always"
	ArchiveCommand    string
	ArchiveDir        string
	WALLevel          string // "minimal", "replica", "logical"
	MaxWALSenders     int
	WALKeepSize       string // e.g., "1GB"
	RestoreCommand    string
}

// RecoveryTarget specifies the point-in-time to recover to
type RecoveryTarget struct {
	TargetTime        *time.Time // Recover to specific timestamp
	TargetXID         string     // Recover to transaction ID
	TargetName        string     // Recover to named restore point
	TargetLSN         string     // Recover to Log Sequence Number
	TargetImmediate   bool       // Recover as soon as consistent state is reached
	TargetInclusive   bool       // Include target transaction
	RecoveryEndAction string     // "pause", "promote", "shutdown"
}

// NewPITRManager creates a new PITR manager
func NewPITRManager(cfg *config.Config, log logger.Logger) *PITRManager {
	return &PITRManager{
		cfg: cfg,
		log: log,
	}
}

// EnablePITR configures PostgreSQL for PITR by modifying postgresql.conf
func (pm *PITRManager) EnablePITR(ctx context.Context, archiveDir string) error {
	pm.log.Info("Enabling PITR (Point-in-Time Recovery)", "archive_dir", archiveDir)

	// Ensure archive directory exists
	if err := os.MkdirAll(archiveDir, 0700); err != nil {
		return fmt.Errorf("failed to create WAL archive directory: %w", err)
	}

	// Find postgresql.conf location
	confPath, err := pm.findPostgreSQLConf(ctx)
	if err != nil {
		return fmt.Errorf("failed to locate postgresql.conf: %w", err)
	}

	pm.log.Info("Found PostgreSQL configuration", "path", confPath)

	// Backup original configuration
	backupPath := confPath + ".backup." + time.Now().Format("20060102_150405")
	if err := pm.backupFile(confPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup postgresql.conf: %w", err)
	}
	pm.log.Info("Created configuration backup", "backup", backupPath)

	// Get absolute path to dbbackup binary
	dbbackupPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get dbbackup executable path: %w", err)
	}

	// Build archive command that calls dbbackup
	archiveCommand := fmt.Sprintf("%s wal archive %%p %%f --archive-dir %s", dbbackupPath, archiveDir)

	// Settings to enable PITR
	settings := map[string]string{
		"wal_level":        "replica", // Required for PITR
		"archive_mode":     "on",
		"archive_command":  archiveCommand,
		"max_wal_senders":  "3",
		"wal_keep_size":    "1GB", // Keep at least 1GB of WAL
	}

	// Update postgresql.conf
	if err := pm.updatePostgreSQLConf(confPath, settings); err != nil {
		return fmt.Errorf("failed to update postgresql.conf: %w", err)
	}

	pm.log.Info("✅ PITR configuration updated successfully")
	pm.log.Warn("⚠️  PostgreSQL restart required for changes to take effect")
	pm.log.Info("To restart PostgreSQL:")
	pm.log.Info("  sudo systemctl restart postgresql")
	pm.log.Info("  OR: sudo pg_ctlcluster <version> <cluster> restart")

	return nil
}

// DisablePITR disables PITR by setting archive_mode = off
func (pm *PITRManager) DisablePITR(ctx context.Context) error {
	pm.log.Info("Disabling PITR")

	confPath, err := pm.findPostgreSQLConf(ctx)
	if err != nil {
		return fmt.Errorf("failed to locate postgresql.conf: %w", err)
	}

	// Backup configuration
	backupPath := confPath + ".backup." + time.Now().Format("20060102_150405")
	if err := pm.backupFile(confPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup postgresql.conf: %w", err)
	}

	settings := map[string]string{
		"archive_mode":    "off",
		"archive_command": "", // Clear command
	}

	if err := pm.updatePostgreSQLConf(confPath, settings); err != nil {
		return fmt.Errorf("failed to update postgresql.conf: %w", err)
	}

	pm.log.Info("✅ PITR disabled successfully")
	pm.log.Warn("⚠️  PostgreSQL restart required")

	return nil
}

// GetCurrentPITRConfig reads current PITR settings from PostgreSQL
func (pm *PITRManager) GetCurrentPITRConfig(ctx context.Context) (*PITRConfig, error) {
	confPath, err := pm.findPostgreSQLConf(ctx)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(confPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgresql.conf: %w", err)
	}
	defer file.Close()

	config := &PITRConfig{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key = value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "'\"")

		switch key {
		case "wal_level":
			config.WALLevel = value
		case "archive_mode":
			config.ArchiveMode = value
			config.Enabled = (value == "on" || value == "always")
		case "archive_command":
			config.ArchiveCommand = value
		case "max_wal_senders":
			fmt.Sscanf(value, "%d", &config.MaxWALSenders)
		case "wal_keep_size":
			config.WALKeepSize = value
		case "restore_command":
			config.RestoreCommand = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading postgresql.conf: %w", err)
	}

	return config, nil
}

// CreateRecoveryConf creates recovery configuration for PITR restore
// PostgreSQL 12+: Creates recovery.signal and modifies postgresql.conf
// PostgreSQL <12: Creates recovery.conf
func (pm *PITRManager) CreateRecoveryConf(ctx context.Context, dataDir string, target RecoveryTarget, walArchiveDir string) error {
	pm.log.Info("Creating recovery configuration", "data_dir", dataDir)

	// Detect PostgreSQL version to determine recovery file format
	version, err := pm.getPostgreSQLVersion(ctx)
	if err != nil {
		pm.log.Warn("Could not detect PostgreSQL version, assuming >= 12", "error", err)
		version = 12 // Default to newer format
	}

	if version >= 12 {
		return pm.createRecoverySignal(ctx, dataDir, target, walArchiveDir)
	} else {
		return pm.createLegacyRecoveryConf(dataDir, target, walArchiveDir)
	}
}

// createRecoverySignal creates recovery.signal for PostgreSQL 12+
func (pm *PITRManager) createRecoverySignal(ctx context.Context, dataDir string, target RecoveryTarget, walArchiveDir string) error {
	// Create recovery.signal file (empty file that triggers recovery mode)
	signalPath := filepath.Join(dataDir, "recovery.signal")
	if err := os.WriteFile(signalPath, []byte{}, 0600); err != nil {
		return fmt.Errorf("failed to create recovery.signal: %w", err)
	}
	pm.log.Info("Created recovery.signal", "path", signalPath)

	// Recovery settings go in postgresql.auto.conf (PostgreSQL 12+)
	autoConfPath := filepath.Join(dataDir, "postgresql.auto.conf")
	
	// Build recovery settings
	var settings []string
	settings = append(settings, fmt.Sprintf("restore_command = 'cp %s/%%f %%p'", walArchiveDir))
	
	if target.TargetTime != nil {
		settings = append(settings, fmt.Sprintf("recovery_target_time = '%s'", target.TargetTime.Format("2006-01-02 15:04:05")))
	} else if target.TargetXID != "" {
		settings = append(settings, fmt.Sprintf("recovery_target_xid = '%s'", target.TargetXID))
	} else if target.TargetName != "" {
		settings = append(settings, fmt.Sprintf("recovery_target_name = '%s'", target.TargetName))
	} else if target.TargetLSN != "" {
		settings = append(settings, fmt.Sprintf("recovery_target_lsn = '%s'", target.TargetLSN))
	} else if target.TargetImmediate {
		settings = append(settings, "recovery_target = 'immediate'")
	}

	if target.RecoveryEndAction != "" {
		settings = append(settings, fmt.Sprintf("recovery_target_action = '%s'", target.RecoveryEndAction))
	}

	// Append to postgresql.auto.conf
	f, err := os.OpenFile(autoConfPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open postgresql.auto.conf: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString("\n# PITR Recovery Configuration (added by dbbackup)\n"); err != nil {
		return err
	}
	for _, setting := range settings {
		if _, err := f.WriteString(setting + "\n"); err != nil {
			return err
		}
	}

	pm.log.Info("Recovery configuration added to postgresql.auto.conf", "path", autoConfPath)
	return nil
}

// createLegacyRecoveryConf creates recovery.conf for PostgreSQL < 12
func (pm *PITRManager) createLegacyRecoveryConf(dataDir string, target RecoveryTarget, walArchiveDir string) error {
	recoveryConfPath := filepath.Join(dataDir, "recovery.conf")
	
	var content strings.Builder
	content.WriteString("# Recovery Configuration (created by dbbackup)\n")
	content.WriteString(fmt.Sprintf("restore_command = 'cp %s/%%f %%p'\n", walArchiveDir))
	
	if target.TargetTime != nil {
		content.WriteString(fmt.Sprintf("recovery_target_time = '%s'\n", target.TargetTime.Format("2006-01-02 15:04:05")))
	}
	// Add other target types...

	if err := os.WriteFile(recoveryConfPath, []byte(content.String()), 0600); err != nil {
		return fmt.Errorf("failed to create recovery.conf: %w", err)
	}

	pm.log.Info("Created recovery.conf", "path", recoveryConfPath)
	return nil
}

// Helper functions

func (pm *PITRManager) findPostgreSQLConf(ctx context.Context) (string, error) {
	// Try common locations
	commonPaths := []string{
		"/var/lib/postgresql/data/postgresql.conf",
		"/etc/postgresql/*/main/postgresql.conf",
		"/usr/local/pgsql/data/postgresql.conf",
	}

	for _, pattern := range commonPaths {
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	// Try to get from PostgreSQL directly
	cmd := exec.CommandContext(ctx, "psql", "-U", pm.cfg.User, "-t", "-c", "SHOW config_file")
	output, err := cmd.Output()
	if err == nil {
		path := strings.TrimSpace(string(output))
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("could not locate postgresql.conf. Please specify --pg-conf-path")
}

func (pm *PITRManager) backupFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644)
}

func (pm *PITRManager) updatePostgreSQLConf(confPath string, settings map[string]string) error {
	file, err := os.Open(confPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	existingKeys := make(map[string]bool)
	scanner := bufio.NewScanner(file)

	// Read existing configuration and track which keys are already present
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)

		// Check if this line sets one of our keys
		for key := range settings {
			if matched, _ := regexp.MatchString(fmt.Sprintf(`^\s*%s\s*=`, key), line); matched {
				existingKeys[key] = true
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Append missing settings
	for key, value := range settings {
		if !existingKeys[key] {
			if value == "" {
				lines = append(lines, fmt.Sprintf("# %s = ''  # Disabled by dbbackup", key))
			} else {
				lines = append(lines, fmt.Sprintf("%s = '%s'  # Added by dbbackup", key, value))
			}
		}
	}

	// Write updated configuration
	output := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(confPath, []byte(output), 0644)
}

func (pm *PITRManager) getPostgreSQLVersion(ctx context.Context) (int, error) {
	cmd := exec.CommandContext(ctx, "psql", "-U", pm.cfg.User, "-t", "-c", "SHOW server_version")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	versionStr := strings.TrimSpace(string(output))
	var major int
	fmt.Sscanf(versionStr, "%d", &major)
	return major, nil
}
