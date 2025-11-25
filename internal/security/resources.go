package security

import (
	"fmt"
	"runtime"
	"syscall"

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
func (rc *ResourceChecker) CheckResourceLimits() (*ResourceLimits, error) {
	if runtime.GOOS == "windows" {
		return rc.checkWindowsLimits()
	}
	return rc.checkUnixLimits()
}

// checkUnixLimits checks resource limits on Unix-like systems
func (rc *ResourceChecker) checkUnixLimits() (*ResourceLimits, error) {
	limits := &ResourceLimits{
		Available: true,
		Platform:  runtime.GOOS,
	}

	// Check max open files (RLIMIT_NOFILE)
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
		limits.MaxOpenFiles = rLimit.Cur
		rc.log.Debug("Resource limit: max open files", "limit", rLimit.Cur, "max", rLimit.Max)
		
		if rLimit.Cur < 1024 {
			rc.log.Warn("⚠️  Low file descriptor limit detected",
				"current", rLimit.Cur,
				"recommended", 4096,
				"hint", "Increase with: ulimit -n 4096")
		}
	}

	// Check max processes (RLIMIT_NPROC) - Linux/BSD only
	if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "openbsd" {
		// RLIMIT_NPROC may not be available on all platforms
		const RLIMIT_NPROC = 6 // Linux value
		if err := syscall.Getrlimit(RLIMIT_NPROC, &rLimit); err == nil {
			limits.MaxProcesses = rLimit.Cur
			rc.log.Debug("Resource limit: max processes", "limit", rLimit.Cur)
		}
	}

	// Check max memory (RLIMIT_AS - address space)
	if err := syscall.Getrlimit(syscall.RLIMIT_AS, &rLimit); err == nil {
		limits.MaxAddressSpace = rLimit.Cur
		// Check if unlimited (max value indicates unlimited)
		if rLimit.Cur < ^uint64(0)-1024 {
			rc.log.Debug("Resource limit: max address space", "limit_mb", rLimit.Cur/1024/1024)
		}
	}

	// Check available memory
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	limits.MaxMemory = memStats.Sys

	rc.log.Debug("Memory stats",
		"alloc_mb", memStats.Alloc/1024/1024,
		"sys_mb", memStats.Sys/1024/1024,
		"num_gc", memStats.NumGC)

	return limits, nil
}

// checkWindowsLimits checks resource limits on Windows
func (rc *ResourceChecker) checkWindowsLimits() (*ResourceLimits, error) {
	limits := &ResourceLimits{
		Available:    true,
		Platform:     "windows",
		MaxOpenFiles: 2048, // Windows default
	}

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	limits.MaxMemory = memStats.Sys

	rc.log.Debug("Windows memory stats",
		"alloc_mb", memStats.Alloc/1024/1024,
		"sys_mb", memStats.Sys/1024/1024)

	return limits, nil
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
