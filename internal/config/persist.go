package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const ConfigFileName = ".dbbackup.conf"

// LocalConfig represents a saved configuration in the current directory
type LocalConfig struct {
	// Database settings
	DBType   string
	Host     string
	Port     int
	User     string
	Database string
	SSLMode  string

	// Backup settings
	BackupDir   string
	Compression int
	Jobs        int
	DumpJobs    int

	// Performance settings
	CPUWorkload string
	MaxCores    int
}

// LoadLocalConfig loads configuration from .dbbackup.conf in current directory
func LoadLocalConfig() (*LocalConfig, error) {
	configPath := filepath.Join(".", ConfigFileName)
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No config file, not an error
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &LocalConfig{}
	lines := strings.Split(string(data), "\n")
	currentSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section headers
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			continue
		}

		// Key-value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch currentSection {
		case "database":
			switch key {
			case "type":
				cfg.DBType = value
			case "host":
				cfg.Host = value
			case "port":
				if p, err := strconv.Atoi(value); err == nil {
					cfg.Port = p
				}
			case "user":
				cfg.User = value
			case "database":
				cfg.Database = value
			case "ssl_mode":
				cfg.SSLMode = value
			}
		case "backup":
			switch key {
			case "backup_dir":
				cfg.BackupDir = value
			case "compression":
				if c, err := strconv.Atoi(value); err == nil {
					cfg.Compression = c
				}
			case "jobs":
				if j, err := strconv.Atoi(value); err == nil {
					cfg.Jobs = j
				}
			case "dump_jobs":
				if dj, err := strconv.Atoi(value); err == nil {
					cfg.DumpJobs = dj
				}
			}
		case "performance":
			switch key {
			case "cpu_workload":
				cfg.CPUWorkload = value
			case "max_cores":
				if mc, err := strconv.Atoi(value); err == nil {
					cfg.MaxCores = mc
				}
			}
		}
	}

	return cfg, nil
}

// SaveLocalConfig saves configuration to .dbbackup.conf in current directory
func SaveLocalConfig(cfg *LocalConfig) error {
	var sb strings.Builder
	
	sb.WriteString("# dbbackup configuration\n")
	sb.WriteString("# This file is auto-generated. Edit with care.\n\n")

	// Database section
	sb.WriteString("[database]\n")
	if cfg.DBType != "" {
		sb.WriteString(fmt.Sprintf("type = %s\n", cfg.DBType))
	}
	if cfg.Host != "" {
		sb.WriteString(fmt.Sprintf("host = %s\n", cfg.Host))
	}
	if cfg.Port != 0 {
		sb.WriteString(fmt.Sprintf("port = %d\n", cfg.Port))
	}
	if cfg.User != "" {
		sb.WriteString(fmt.Sprintf("user = %s\n", cfg.User))
	}
	if cfg.Database != "" {
		sb.WriteString(fmt.Sprintf("database = %s\n", cfg.Database))
	}
	if cfg.SSLMode != "" {
		sb.WriteString(fmt.Sprintf("ssl_mode = %s\n", cfg.SSLMode))
	}
	sb.WriteString("\n")

	// Backup section
	sb.WriteString("[backup]\n")
	if cfg.BackupDir != "" {
		sb.WriteString(fmt.Sprintf("backup_dir = %s\n", cfg.BackupDir))
	}
	if cfg.Compression != 0 {
		sb.WriteString(fmt.Sprintf("compression = %d\n", cfg.Compression))
	}
	if cfg.Jobs != 0 {
		sb.WriteString(fmt.Sprintf("jobs = %d\n", cfg.Jobs))
	}
	if cfg.DumpJobs != 0 {
		sb.WriteString(fmt.Sprintf("dump_jobs = %d\n", cfg.DumpJobs))
	}
	sb.WriteString("\n")

	// Performance section
	sb.WriteString("[performance]\n")
	if cfg.CPUWorkload != "" {
		sb.WriteString(fmt.Sprintf("cpu_workload = %s\n", cfg.CPUWorkload))
	}
	if cfg.MaxCores != 0 {
		sb.WriteString(fmt.Sprintf("max_cores = %d\n", cfg.MaxCores))
	}

	configPath := filepath.Join(".", ConfigFileName)
	if err := os.WriteFile(configPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ApplyLocalConfig applies loaded local config to the main config if values are not already set
func ApplyLocalConfig(cfg *Config, local *LocalConfig) {
	if local == nil {
		return
	}

	// Only apply if not already set via flags
	if cfg.DatabaseType == "postgres" && local.DBType != "" {
		cfg.DatabaseType = local.DBType
	}
	if cfg.Host == "localhost" && local.Host != "" {
		cfg.Host = local.Host
	}
	if cfg.Port == 5432 && local.Port != 0 {
		cfg.Port = local.Port
	}
	if cfg.User == "root" && local.User != "" {
		cfg.User = local.User
	}
	if local.Database != "" {
		cfg.Database = local.Database
	}
	if cfg.SSLMode == "prefer" && local.SSLMode != "" {
		cfg.SSLMode = local.SSLMode
	}
	if local.BackupDir != "" {
		cfg.BackupDir = local.BackupDir
	}
	if cfg.CompressionLevel == 6 && local.Compression != 0 {
		cfg.CompressionLevel = local.Compression
	}
	if local.Jobs != 0 {
		cfg.Jobs = local.Jobs
	}
	if local.DumpJobs != 0 {
		cfg.DumpJobs = local.DumpJobs
	}
	if cfg.CPUWorkloadType == "balanced" && local.CPUWorkload != "" {
		cfg.CPUWorkloadType = local.CPUWorkload
	}
	if local.MaxCores != 0 {
		cfg.MaxCores = local.MaxCores
	}
}

// ConfigFromConfig creates a LocalConfig from a Config
func ConfigFromConfig(cfg *Config) *LocalConfig {
	return &LocalConfig{
		DBType:      cfg.DatabaseType,
		Host:        cfg.Host,
		Port:        cfg.Port,
		User:        cfg.User,
		Database:    cfg.Database,
		SSLMode:     cfg.SSLMode,
		BackupDir:   cfg.BackupDir,
		Compression: cfg.CompressionLevel,
		Jobs:        cfg.Jobs,
		DumpJobs:    cfg.DumpJobs,
		CPUWorkload: cfg.CPUWorkloadType,
		MaxCores:    cfg.MaxCores,
	}
}
