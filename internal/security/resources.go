package security

import (
	"fmt"
	"runtime"

	"dbbackup/internal/logger"
)

// ResourceChecker checks system resource limits
type ResourceChecker struct {
	log logger.Logger
}

// NewResourceChecker creates a new resource checker
func NewResourceChecker(log logger.Logger) *ResourceChecker {
	return &ResourceChecker{
		log: log,
	}
}

// ResourceLimits holds system resource limit information
type ResourceLimits struct {
	MaxOpenFiles    uint64
	MaxProcesses    uint64
	MaxMemory       uint64
	MaxAddressSpace uint64
	Available       bool
	Platform        string
}

// CheckResourceLimits checks and reports system resource limits
// Platform-specific implementation is in resources_unix.go and resources_windows.go
func (rc *ResourceChecker) CheckResourceLimits() (*ResourceLimits, error) {
	return rc.checkPlatformLimits()
}

// ValidateResourcesForBackup validates resources are sufficient for backup operation
func (rc *ResourceChecker) ValidateResourcesForBackup(estimatedSize int64) error {
	limits, err := rc.CheckResourceLimits()
	if err != nil {
		return fmt.Errorf("failed to check resource limits: %w", err)
	}

	var warnings []string

	// Check file descriptor limit on Unix
	if runtime.GOOS != "windows" && limits.MaxOpenFiles < 1024 {
		warnings = append(warnings,
			fmt.Sprintf("Low file descriptor limit (%d), recommended: 4096+", limits.MaxOpenFiles))
	}

	// Check memory (warn if backup size might exceed available memory)
	estimatedMemory := estimatedSize / 10 // Rough estimate: 10% of backup size
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	availableMemory := memStats.Sys - memStats.Alloc

	if estimatedMemory > int64(availableMemory) {
		warnings = append(warnings,
			fmt.Sprintf("Backup may require more memory than available (estimated: %dMB, available: %dMB)",
				estimatedMemory/1024/1024, availableMemory/1024/1024))
	}

	if len(warnings) > 0 {
		for _, warning := range warnings {
			rc.log.Warn("⚠️  Resource constraint: " + warning)
		}
		rc.log.Info("Continuing backup operation (warnings are informational)")
	}

	return nil
}

// GetResourceRecommendations returns recommendations for resource limits
func (rc *ResourceChecker) GetResourceRecommendations() []string {
	if runtime.GOOS == "windows" {
		return []string{
			"Ensure sufficient disk space (3-4x backup size)",
			"Monitor memory usage during large backups",
			"Close unnecessary applications before backup",
		}
	}

	return []string{
		"Set file descriptor limit: ulimit -n 4096",
		"Set max processes: ulimit -u 4096",
		"Monitor disk space: df -h",
		"Check memory: free -h",
		"For large backups, consider increasing limits in /etc/security/limits.conf",
		"Example limits.conf entry: dbbackup soft nofile 8192",
	}
}
