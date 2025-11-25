package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var cpuCmd = &cobra.Command{
	Use:   "cpu",
	Short: "Show CPU information and optimization settings",
	Long:  `Display detailed CPU information and current parallelism configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCPUInfo(cmd.Context())
	},
}

func runCPUInfo(ctx context.Context) error {
	log.Info("Detecting CPU information...")
	
	// Optimize CPU settings if auto-detect is enabled
	if cfg.AutoDetectCores {
		if err := cfg.OptimizeForCPU(); err != nil {
			log.Warn("CPU optimization failed", "error", err)
		}
	}
	
	// Get CPU information
	cpuInfo, err := cfg.GetCPUInfo()
	if err != nil {
		return fmt.Errorf("failed to detect CPU: %w", err)
	}
	
	fmt.Println("=== CPU Information ===")
	fmt.Print(cpuInfo.FormatCPUInfo())
	
	fmt.Println("\n=== Current Configuration ===")
	fmt.Printf("Auto-detect cores: %t\n", cfg.AutoDetectCores)
	fmt.Printf("CPU workload type: %s\n", cfg.CPUWorkloadType)
	fmt.Printf("Parallel jobs (restore): %d\n", cfg.Jobs)
	fmt.Printf("Dump jobs (backup): %d\n", cfg.DumpJobs)
	fmt.Printf("Maximum cores limit: %d\n", cfg.MaxCores)
	
	// Show optimization recommendations
	fmt.Println("\n=== Optimization Recommendations ===")
	if cpuInfo.PhysicalCores > 1 {
		if cfg.CPUWorkloadType == "balanced" {
			optimal, _ := cfg.CPUDetector.CalculateOptimalJobs("balanced", cfg.MaxCores)
			fmt.Printf("Recommended jobs (balanced): %d\n", optimal)
		}
		if cfg.CPUWorkloadType == "io-intensive" {
			optimal, _ := cfg.CPUDetector.CalculateOptimalJobs("io-intensive", cfg.MaxCores)
			fmt.Printf("Recommended jobs (I/O intensive): %d\n", optimal)
		}
		if cfg.CPUWorkloadType == "cpu-intensive" {
			optimal, _ := cfg.CPUDetector.CalculateOptimalJobs("cpu-intensive", cfg.MaxCores)
			fmt.Printf("Recommended jobs (CPU intensive): %d\n", optimal)
		}
	}
	
	// Show current vs optimal
	if cfg.AutoDetectCores {
		fmt.Println("\n✅ CPU optimization is enabled")
		fmt.Println("Job counts are automatically optimized based on detected hardware")
	} else {
		fmt.Println("\n⚠️  CPU optimization is disabled")
		fmt.Println("Consider enabling --auto-detect-cores for better performance")
	}
	
	return nil
}

func init() {
	rootCmd.AddCommand(cpuCmd)
}