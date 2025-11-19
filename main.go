package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"dbbackup/cmd"
	"dbbackup/internal/config"
	"dbbackup/internal/logger"
	"dbbackup/internal/metrics"
)

// Build information (set by ldflags)
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Create context that cancels on interrupt
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Initialize configuration
	cfg := config.New()
	
	// Set version information
	cfg.Version = version
	cfg.BuildTime = buildTime
	cfg.GitCommit = gitCommit
	
	// Optimize CPU settings if auto-detect is enabled
	if cfg.AutoDetectCores {
		if err := cfg.OptimizeForCPU(); err != nil {
			slog.Warn("CPU optimization failed", "error", err)
		}
	}

	// Initialize logger
	log := logger.New(cfg.LogLevel, cfg.LogFormat)

	// Initialize global metrics
	metrics.InitGlobalMetrics(log)
	
	// Show session summary on exit
	defer func() {
		if metrics.GlobalMetrics != nil {
			avgs := metrics.GlobalMetrics.GetAverages()
			if ops, ok := avgs["total_operations"].(int); ok && ops > 0 {
				fmt.Printf("\n📊 Session Summary: %d operations, %.1f%% success rate\n", 
					ops, avgs["success_rate"])
			}
		}
	}()

	// Execute command
	if err := cmd.Execute(ctx, cfg, log); err != nil {
		log.Error("Application failed", "error", err)
		os.Exit(1)
	}
}