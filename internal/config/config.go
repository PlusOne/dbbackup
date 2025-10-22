package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	
	"dbbackup/internal/cpu"
)

// Config holds all configuration options
type Config struct {
	// Version information
	Version   string
	BuildTime string
	GitCommit string

	// Database connection
	Host         string
	Port         int
	User         string
	Database     string
	Password     string
	DatabaseType string // "postgres" or "mysql"
	SSLMode      string
	Insecure     bool

	// Backup options
	BackupDir        string
	CompressionLevel int
	Jobs             int
	DumpJobs         int
	MaxCores         int
	AutoDetectCores  bool
	CPUWorkloadType  string // "cpu-intensive", "io-intensive", "balanced"
	
	// CPU detection
	CPUDetector *cpu.Detector
	CPUInfo     *cpu.CPUInfo

	// Sample backup options
	SampleStrategy string // "ratio", "percent", "count"
	SampleValue    int

	// Output options
	NoColor      bool
	Debug        bool
	LogLevel     string
	LogFormat    string
	OutputLength int

	// Single database backup/restore
	SingleDBName  string
	RestoreDBName string
}

// New creates a new configuration with default values
func New() *Config {
	// Get default backup directory
	backupDir := getEnvString("BACKUP_DIR", getDefaultBackupDir())

	// Initialize CPU detector
	cpuDetector := cpu.NewDetector()
	cpuInfo, _ := cpuDetector.DetectCPU()

	return &Config{
		// Database defaults
		Host:         getEnvString("PG_HOST", "localhost"),
		Port:         getEnvInt("PG_PORT", 5432),
		User:         getEnvString("PG_USER", getCurrentUser()),
		Database:     getEnvString("PG_DATABASE", "postgres"),
		Password:     getEnvString("PGPASSWORD", ""),
		DatabaseType: getEnvString("DB_TYPE", "postgres"),
		SSLMode:      getEnvString("PG_SSLMODE", "prefer"),
		Insecure:     getEnvBool("INSECURE", false),

		// Backup defaults
		BackupDir:        backupDir,
		CompressionLevel: getEnvInt("COMPRESS_LEVEL", 6),
		Jobs:             getEnvInt("JOBS", getDefaultJobs(cpuInfo)),
		DumpJobs:         getEnvInt("DUMP_JOBS", getDefaultDumpJobs(cpuInfo)),
		MaxCores:         getEnvInt("MAX_CORES", getDefaultMaxCores(cpuInfo)),
		AutoDetectCores:  getEnvBool("AUTO_DETECT_CORES", true),
		CPUWorkloadType:  getEnvString("CPU_WORKLOAD_TYPE", "balanced"),

		// CPU detection
		CPUDetector: cpuDetector,
		CPUInfo:     cpuInfo,

		// Sample backup defaults
		SampleStrategy: getEnvString("SAMPLE_STRATEGY", "ratio"),
		SampleValue:    getEnvInt("SAMPLE_VALUE", 10),

		// Output defaults
		NoColor:      getEnvBool("NO_COLOR", false),
		Debug:        getEnvBool("DEBUG", false),
		LogLevel:     getEnvString("LOG_LEVEL", "info"),
		LogFormat:    getEnvString("LOG_FORMAT", "text"),
		OutputLength: getEnvInt("OUTPUT_LENGTH", 0),

		// Single database options
		SingleDBName:  getEnvString("SINGLE_DB_NAME", ""),
		RestoreDBName: getEnvString("RESTORE_DB_NAME", ""),
	}
}

// UpdateFromEnvironment updates configuration from environment variables
func (c *Config) UpdateFromEnvironment() {
	if password := os.Getenv("PGPASSWORD"); password != "" {
		c.Password = password
	}
	if password := os.Getenv("MYSQL_PWD"); password != "" && c.DatabaseType == "mysql" {
		c.Password = password
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.DatabaseType != "postgres" && c.DatabaseType != "mysql" {
		return &ConfigError{Field: "database-type", Value: c.DatabaseType, Message: "must be 'postgres' or 'mysql'"}
	}
	
	if c.CompressionLevel < 0 || c.CompressionLevel > 9 {
		return &ConfigError{Field: "compression", Value: string(rune(c.CompressionLevel)), Message: "must be between 0-9"}
	}
	
	if c.Jobs < 1 {
		return &ConfigError{Field: "jobs", Value: string(rune(c.Jobs)), Message: "must be at least 1"}
	}
	
	if c.DumpJobs < 1 {
		return &ConfigError{Field: "dump-jobs", Value: string(rune(c.DumpJobs)), Message: "must be at least 1"}
	}

	return nil
}

// IsPostgreSQL returns true if database type is PostgreSQL
func (c *Config) IsPostgreSQL() bool {
	return c.DatabaseType == "postgres"
}

// IsMySQL returns true if database type is MySQL
func (c *Config) IsMySQL() bool {
	return c.DatabaseType == "mysql"
}

// GetDefaultPort returns the default port for the database type
func (c *Config) GetDefaultPort() int {
	if c.IsMySQL() {
		return 3306
	}
	return 5432
}

// OptimizeForCPU optimizes job settings based on detected CPU
func (c *Config) OptimizeForCPU() error {
	if c.CPUDetector == nil {
		c.CPUDetector = cpu.NewDetector()
	}
	
	if c.CPUInfo == nil {
		info, err := c.CPUDetector.DetectCPU()
		if err != nil {
			return err
		}
		c.CPUInfo = info
	}
	
	if c.AutoDetectCores {
		// Optimize jobs based on workload type
		if jobs, err := c.CPUDetector.CalculateOptimalJobs(c.CPUWorkloadType, c.MaxCores); err == nil {
			c.Jobs = jobs
		}
		
		// Optimize dump jobs (more conservative for database dumps)
		if dumpJobs, err := c.CPUDetector.CalculateOptimalJobs("cpu-intensive", c.MaxCores/2); err == nil {
			c.DumpJobs = dumpJobs
			if c.DumpJobs > 8 {
				c.DumpJobs = 8 // Conservative limit for dumps
			}
		}
	}
	
	return nil
}

// GetCPUInfo returns CPU information, detecting if necessary
func (c *Config) GetCPUInfo() (*cpu.CPUInfo, error) {
	if c.CPUInfo != nil {
		return c.CPUInfo, nil
	}
	
	if c.CPUDetector == nil {
		c.CPUDetector = cpu.NewDetector()
	}
	
	info, err := c.CPUDetector.DetectCPU()
	if err != nil {
		return nil, err
	}
	
	c.CPUInfo = info
	return info, nil
}

// ConfigError represents a configuration validation error
type ConfigError struct {
	Field   string
	Value   string
	Message string
}

func (e *ConfigError) Error() string {
	return "config error in field '" + e.Field + "' with value '" + e.Value + "': " + e.Message
}

// Helper functions
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getCurrentUser() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	return "postgres"
}

func getDefaultBackupDir() string {
	// Try to create a sensible default backup directory
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		return filepath.Join(homeDir, "db_backups")
	}
	
	// Fallback based on OS
	if runtime.GOOS == "windows" {
		return "C:\\db_backups"
	}
	
	// For PostgreSQL user on Linux/Unix
	if getCurrentUser() == "postgres" {
		return "/var/lib/pgsql/pg_backups"
	}
	
	return "/tmp/db_backups"
}

// CPU-related helper functions
func getDefaultJobs(cpuInfo *cpu.CPUInfo) int {
	if cpuInfo == nil {
		return 1
	}
	// Default to logical cores for restore operations
	jobs := cpuInfo.LogicalCores
	if jobs < 1 {
		jobs = 1
	}
	if jobs > 16 {
		jobs = 16 // Safety limit
	}
	return jobs
}

func getDefaultDumpJobs(cpuInfo *cpu.CPUInfo) int {
	if cpuInfo == nil {
		return 1
	}
	// Use physical cores for dump operations (CPU intensive)
	jobs := cpuInfo.PhysicalCores
	if jobs < 1 {
		jobs = 1
	}
	if jobs > 8 {
		jobs = 8 // Conservative limit for dumps
	}
	return jobs
}

func getDefaultMaxCores(cpuInfo *cpu.CPUInfo) int {
	if cpuInfo == nil {
		return 16
	}
	// Set max cores to 2x logical cores, with reasonable upper limit
	maxCores := cpuInfo.LogicalCores * 2
	if maxCores < 4 {
		maxCores = 4
	}
	if maxCores > 64 {
		maxCores = 64
	}
	return maxCores
}