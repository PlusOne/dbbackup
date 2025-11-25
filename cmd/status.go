package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"dbbackup/internal/database"
	"dbbackup/internal/progress"
)

// runStatus displays configuration and tests connectivity
func runStatus(ctx context.Context) error {
	// Update config from environment
	cfg.UpdateFromEnvironment()
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	
	// Display header
	displayHeader()
	
	// Display configuration
	displayConfiguration()
	
	// Test database connection
	return testConnection(ctx)
}

// displayHeader shows the application header
func displayHeader() {
	if cfg.NoColor {
		fmt.Println("==============================================================")
		fmt.Println(" Database Backup & Recovery Tool")
		fmt.Println("==============================================================")
	} else {
		fmt.Println("\033[1;34m==============================================================\033[0m")
		fmt.Println("\033[1;37m Database Backup & Recovery Tool\033[0m")
		fmt.Println("\033[1;34m==============================================================\033[0m")
	}
	
	fmt.Printf("Version: %s (built: %s, commit: %s)\n", cfg.Version, cfg.BuildTime, cfg.GitCommit)
	fmt.Println()
}

// displayConfiguration shows current configuration
func displayConfiguration() {
	fmt.Println("Configuration:")
	fmt.Printf("  Database Type: %s\n", cfg.DatabaseType)
	fmt.Printf("  Host:          %s:%d\n", cfg.Host, cfg.Port)
	fmt.Printf("  User:          %s\n", cfg.User)
	fmt.Printf("  Database:      %s\n", cfg.Database)
	
	if cfg.Password != "" {
		fmt.Printf("  Password:      ****** (set)\n")
	} else {
		fmt.Printf("  Password:      (not set)\n")
	}
	
	fmt.Printf("  SSL Mode:      %s\n", cfg.SSLMode)
	if cfg.Insecure {
		fmt.Printf("  SSL:           disabled\n")
	}
	
	fmt.Printf("  Backup Dir:    %s\n", cfg.BackupDir)
	fmt.Printf("  Compression:   %d\n", cfg.CompressionLevel)
	fmt.Printf("  Jobs:          %d\n", cfg.Jobs)
	fmt.Printf("  Dump Jobs:     %d\n", cfg.DumpJobs)
	fmt.Printf("  Max Cores:     %d\n", cfg.MaxCores)
	fmt.Printf("  Auto Detect:   %v\n", cfg.AutoDetectCores)
	
	// System information
	fmt.Println()
	fmt.Println("System Information:")
	fmt.Printf("  OS:            %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  CPU Cores:     %d\n", runtime.NumCPU())
	fmt.Printf("  Go Version:    %s\n", runtime.Version())
	
	// Check if backup directory exists
	if info, err := os.Stat(cfg.BackupDir); err != nil {
		fmt.Printf("  Backup Dir:    %s (does not exist - will be created)\n", cfg.BackupDir)
	} else if info.IsDir() {
		fmt.Printf("  Backup Dir:    %s (exists, writable)\n", cfg.BackupDir)
	} else {
		fmt.Printf("  Backup Dir:    %s (exists but not a directory!)\n", cfg.BackupDir)
	}
	
	fmt.Println()
}

// testConnection tests database connectivity
func testConnection(ctx context.Context) error {
	// Create progress indicator
	indicator := progress.NewIndicator(true, "spinner")
	
	// Create database instance
	db, err := database.New(cfg, log)
	if err != nil {
		indicator.Fail(fmt.Sprintf("Failed to create database instance: %v", err))
		return err
	}
	defer db.Close()
	
	// Test tool availability
	indicator.Start("Checking required tools...")
	if err := db.ValidateBackupTools(); err != nil {
		indicator.Fail(fmt.Sprintf("Tool validation failed: %v", err))
		return err
	}
	indicator.Complete("Required tools available")
	
	// Test connection
	indicator.Start(fmt.Sprintf("Connecting to %s...", cfg.DatabaseType))
	if err := db.Connect(ctx); err != nil {
		indicator.Fail(fmt.Sprintf("Connection failed: %v", err))
		return err
	}
	indicator.Complete("Connected successfully")
	
	// Test basic operations
	indicator.Start("Testing database operations...")
	
	// Get version
	version, err := db.GetVersion(ctx)
	if err != nil {
		indicator.Fail(fmt.Sprintf("Failed to get database version: %v", err))
		return err
	}
	
	// List databases
	databases, err := db.ListDatabases(ctx)
	if err != nil {
		indicator.Fail(fmt.Sprintf("Failed to list databases: %v", err))
		return err
	}
	
	indicator.Complete("Database operations successful")
	
	// Display results
	fmt.Println("Connection Test Results:")
	fmt.Printf("  Status:        Connected ✅\n")
	fmt.Printf("  Version:       %s\n", version)
	fmt.Printf("  Databases:     %d found\n", len(databases))
	
	if len(databases) > 0 {
		fmt.Printf("  Database List: ")
		if len(databases) <= 5 {
			for i, db := range databases {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(db)
			}
		} else {
			for i := 0; i < 3; i++ {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(databases[i])
			}
			fmt.Printf(", ... (%d more)", len(databases)-3)
		}
		fmt.Println()
	}
	
	fmt.Println()
	fmt.Println("✅ Status check completed successfully!")
	
	return nil
}