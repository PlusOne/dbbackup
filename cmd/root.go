package cmd

import (
	"context"
	"fmt"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
	"dbbackup/internal/security"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	cfg         *config.Config
	log         logger.Logger
	auditLogger *security.AuditLogger
	rateLimiter *security.RateLimiter
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "dbbackup",
	Short: "Multi-database backup and restore tool",
	Long: `A comprehensive database backup and restore solution supporting both PostgreSQL and MySQL.

Features:
- CPU-aware parallel processing
- Multiple backup modes (cluster, single database, sample)
- Interactive UI and CLI modes
- Archive verification and restore
- Progress indicators and timing summaries
- Robust error handling and logging

Database Support:
- PostgreSQL (via pg_dump/pg_restore)
- MySQL (via mysqldump/mysql)

For help with specific commands, use: dbbackup [command] --help`,
	Version: "",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			return nil
		}
		
		// Store which flags were explicitly set by user
		flagsSet := make(map[string]bool)
		cmd.Flags().Visit(func(f *pflag.Flag) {
			flagsSet[f.Name] = true
		})
		
		// Load local config if not disabled
		if !cfg.NoLoadConfig {
			if localCfg, err := config.LoadLocalConfig(); err != nil {
				log.Warn("Failed to load local config", "error", err)
			} else if localCfg != nil {
				// Save current flag values that were explicitly set
				savedBackupDir := cfg.BackupDir
				savedHost := cfg.Host
				savedPort := cfg.Port
				savedUser := cfg.User
				savedDatabase := cfg.Database
				savedCompression := cfg.CompressionLevel
				savedJobs := cfg.Jobs
				savedDumpJobs := cfg.DumpJobs
				savedRetentionDays := cfg.RetentionDays
				savedMinBackups := cfg.MinBackups
				
				// Apply config from file
				config.ApplyLocalConfig(cfg, localCfg)
				log.Info("Loaded configuration from .dbbackup.conf")
				
				// Restore explicitly set flag values (flags have priority)
				if flagsSet["backup-dir"] {
					cfg.BackupDir = savedBackupDir
				}
				if flagsSet["host"] {
					cfg.Host = savedHost
				}
				if flagsSet["port"] {
					cfg.Port = savedPort
				}
				if flagsSet["user"] {
					cfg.User = savedUser
				}
				if flagsSet["database"] {
					cfg.Database = savedDatabase
				}
				if flagsSet["compression"] {
					cfg.CompressionLevel = savedCompression
				}
				if flagsSet["jobs"] {
					cfg.Jobs = savedJobs
				}
				if flagsSet["dump-jobs"] {
					cfg.DumpJobs = savedDumpJobs
				}
				if flagsSet["retention-days"] {
					cfg.RetentionDays = savedRetentionDays
				}
				if flagsSet["min-backups"] {
					cfg.MinBackups = savedMinBackups
				}
			}
		}
		
		return cfg.SetDatabaseType(cfg.DatabaseType)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute(ctx context.Context, config *config.Config, logger logger.Logger) error {
	cfg = config
	log = logger
	
	// Initialize audit logger
	auditLogger = security.NewAuditLogger(logger, true)
	
	// Initialize rate limiter
	rateLimiter = security.NewRateLimiter(config.MaxRetries, logger)

	// Set version info
	rootCmd.Version = fmt.Sprintf("%s (built: %s, commit: %s)",
		cfg.Version, cfg.BuildTime, cfg.GitCommit)

	// Add persistent flags
	rootCmd.PersistentFlags().StringVar(&cfg.Host, "host", cfg.Host, "Database host")
	rootCmd.PersistentFlags().IntVar(&cfg.Port, "port", cfg.Port, "Database port")
	rootCmd.PersistentFlags().StringVar(&cfg.User, "user", cfg.User, "Database user")
	rootCmd.PersistentFlags().StringVar(&cfg.Database, "database", cfg.Database, "Database name")
	rootCmd.PersistentFlags().StringVar(&cfg.Password, "password", cfg.Password, "Database password")
	rootCmd.PersistentFlags().StringVarP(&cfg.DatabaseType, "db-type", "d", cfg.DatabaseType, "Database type (postgres|mysql|mariadb)")
	rootCmd.PersistentFlags().StringVar(&cfg.BackupDir, "backup-dir", cfg.BackupDir, "Backup directory")
	rootCmd.PersistentFlags().BoolVar(&cfg.NoColor, "no-color", cfg.NoColor, "Disable colored output")
	rootCmd.PersistentFlags().BoolVar(&cfg.Debug, "debug", cfg.Debug, "Enable debug logging")
	rootCmd.PersistentFlags().IntVar(&cfg.Jobs, "jobs", cfg.Jobs, "Number of parallel jobs")
	rootCmd.PersistentFlags().IntVar(&cfg.DumpJobs, "dump-jobs", cfg.DumpJobs, "Number of parallel dump jobs")
	rootCmd.PersistentFlags().IntVar(&cfg.MaxCores, "max-cores", cfg.MaxCores, "Maximum CPU cores to use")
	rootCmd.PersistentFlags().BoolVar(&cfg.AutoDetectCores, "auto-detect-cores", cfg.AutoDetectCores, "Auto-detect CPU cores")
	rootCmd.PersistentFlags().StringVar(&cfg.CPUWorkloadType, "cpu-workload", cfg.CPUWorkloadType, "CPU workload type (cpu-intensive|io-intensive|balanced)")
	rootCmd.PersistentFlags().StringVar(&cfg.SSLMode, "ssl-mode", cfg.SSLMode, "SSL mode for connections")
	rootCmd.PersistentFlags().BoolVar(&cfg.Insecure, "insecure", cfg.Insecure, "Disable SSL (shortcut for --ssl-mode=disable)")
	rootCmd.PersistentFlags().IntVar(&cfg.CompressionLevel, "compression", cfg.CompressionLevel, "Compression level (0-9)")
	rootCmd.PersistentFlags().BoolVar(&cfg.NoSaveConfig, "no-save-config", false, "Don't save configuration after successful operations")
	rootCmd.PersistentFlags().BoolVar(&cfg.NoLoadConfig, "no-config", false, "Don't load configuration from .dbbackup.conf")
	
	// Security flags (MEDIUM priority)
	rootCmd.PersistentFlags().IntVar(&cfg.RetentionDays, "retention-days", cfg.RetentionDays, "Backup retention period in days (0=disabled)")
	rootCmd.PersistentFlags().IntVar(&cfg.MinBackups, "min-backups", cfg.MinBackups, "Minimum number of backups to keep")
	rootCmd.PersistentFlags().IntVar(&cfg.MaxRetries, "max-retries", cfg.MaxRetries, "Maximum connection retry attempts")
	rootCmd.PersistentFlags().BoolVar(&cfg.AllowRoot, "allow-root", cfg.AllowRoot, "Allow running as root/Administrator")
	rootCmd.PersistentFlags().BoolVar(&cfg.CheckResources, "check-resources", cfg.CheckResources, "Check system resource limits")

	return rootCmd.ExecuteContext(ctx)
}

func init() {
	// Register subcommands
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(interactiveCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(preflightCmd)
}
