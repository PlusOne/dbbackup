package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// backupCmd represents the backup command
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create database backups",
	Long: `Create database backups with support for various modes:

Backup Modes:
  cluster    - Full cluster backup (all databases + globals) [PostgreSQL only]
  single     - Single database backup
  sample     - Sample database backup (reduced dataset)

Examples:
  # Full cluster backup (PostgreSQL)
  dbbackup backup cluster --db-type postgres

  # Single database backup
  dbbackup backup single mydb --db-type postgres
  dbbackup backup single mydb --db-type mysql

  # Sample database backup
  dbbackup backup sample mydb --sample-ratio 10 --db-type postgres`,
}

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Create full cluster backup (PostgreSQL only)",
	Long:  `Create a complete backup of the entire PostgreSQL cluster including all databases and global objects (roles, tablespaces, etc.)`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runClusterBackup(cmd.Context())
	},
}

var singleCmd = &cobra.Command{
	Use:   "single [database]",
	Short: "Create single database backup",
	Long:  `Create a backup of a single database with all its data and schema`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dbName := ""
		if len(args) > 0 {
			dbName = args[0]
		} else if cfg.SingleDBName != "" {
			dbName = cfg.SingleDBName
		} else {
			return fmt.Errorf("database name required (provide as argument or set SINGLE_DB_NAME)")
		}
		
		return runSingleBackup(cmd.Context(), dbName)
	},
}

var sampleCmd = &cobra.Command{
	Use:   "sample [database]",
	Short: "Create sample database backup",
	Long: `Create a sample database backup with reduced dataset for testing/development.

Sampling Strategies:
  --sample-ratio N     - Take every Nth record (e.g., 10 = every 10th record)
  --sample-percent N   - Take N% of records (e.g., 20 = 20% of data)  
  --sample-count N     - Take first N records from each table

Warning: Sample backups may break referential integrity due to sampling!`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dbName := ""
		if len(args) > 0 {
			dbName = args[0]
		} else if cfg.SingleDBName != "" {
			dbName = cfg.SingleDBName
		} else {
			return fmt.Errorf("database name required (provide as argument or set SAMPLE_DB_NAME)")
		}
		
		return runSampleBackup(cmd.Context(), dbName)
	},
}

func init() {
	// Add backup subcommands
	backupCmd.AddCommand(clusterCmd)
	backupCmd.AddCommand(singleCmd)
	backupCmd.AddCommand(sampleCmd)
	
	// Sample backup flags - use local variables to avoid cfg access during init
	var sampleStrategy string
	var sampleValue int
	var sampleRatio int
	var samplePercent int
	var sampleCount int
	
	sampleCmd.Flags().StringVar(&sampleStrategy, "sample-strategy", "ratio", "Sampling strategy (ratio|percent|count)")
	sampleCmd.Flags().IntVar(&sampleValue, "sample-value", 10, "Sampling value")
	sampleCmd.Flags().IntVar(&sampleRatio, "sample-ratio", 0, "Take every Nth record")
	sampleCmd.Flags().IntVar(&samplePercent, "sample-percent", 0, "Take N% of records")
	sampleCmd.Flags().IntVar(&sampleCount, "sample-count", 0, "Take first N records")
	
	// Set up pre-run hook to handle convenience flags and update cfg
	sampleCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// Update cfg with flag values
		if cmd.Flags().Changed("sample-ratio") && sampleRatio > 0 {
			cfg.SampleStrategy = "ratio"
			cfg.SampleValue = sampleRatio
		} else if cmd.Flags().Changed("sample-percent") && samplePercent > 0 {
			cfg.SampleStrategy = "percent"
			cfg.SampleValue = samplePercent
		} else if cmd.Flags().Changed("sample-count") && sampleCount > 0 {
			cfg.SampleStrategy = "count"
			cfg.SampleValue = sampleCount
		} else if cmd.Flags().Changed("sample-strategy") {
			cfg.SampleStrategy = sampleStrategy
		}
		if cmd.Flags().Changed("sample-value") {
			cfg.SampleValue = sampleValue
		}
		return nil
	}
	
	// Mark the strategy flags as mutually exclusive
	sampleCmd.MarkFlagsMutuallyExclusive("sample-ratio", "sample-percent", "sample-count")
}