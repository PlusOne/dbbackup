package cmd

import (
	"context"
	"fmt"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
	"github.com/spf13/cobra"
)

var (
	cfg *config.Config
	log logger.Logger
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
		
		// Load local config if not disabled
		if !cfg.NoLoadConfig {
			if localCfg, err := config.LoadLocalConfig(); err != nil {
				log.Warn("Failed to load local config", "error", err)
			} else if localCfg != nil {
				config.ApplyLocalConfig(cfg, localCfg)
				log.Info("Loaded configuration from .dbbackup.conf")
			}
		}
		
		return cfg.SetDatabaseType(cfg.DatabaseType)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute(ctx context.Context, config *config.Config, logger logger.Logger) error {
	cfg = config
	log = logger

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
