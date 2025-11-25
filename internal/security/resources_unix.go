// +build !windows

package security

import (
	"runtime"
	"syscall"
)

// checkPlatformLimits checks resource limits on Unix-like systems
func (rc *ResourceChecker) checkPlatformLimits() (*ResourceLimits, error) {
	limits := &ResourceLimits{
		Available: true,
		Platform:  runtime.GOOS,
	}

	// Check max open files (RLIMIT_NOFILE)
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
		limits.MaxOpenFiles = uint64(rLimit.Cur)
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
			limits.MaxProcesses = uint64(rLimit.Cur)
			rc.log.Debug("Resource limit: max processes", "limit", rLimit.Cur)
		}
	}

	return limits, nil
}
