// +build windows

package security

import (
	"runtime"
)

// checkPlatformLimits returns resource limits for Windows
func (rc *ResourceChecker) checkPlatformLimits() (*ResourceLimits, error) {
	limits := &ResourceLimits{
		Available: false, // Windows doesn't use Unix-style rlimits
		Platform:  runtime.GOOS,
	}

	// Windows doesn't have the same resource limit concept
	// Set reasonable defaults
	limits.MaxOpenFiles = 8192 // Windows default is typically much higher
	limits.MaxProcesses = 0    // Not applicable
	limits.MaxAddressSpace = 0 // Not applicable

	rc.log.Debug("Resource limits not available on Windows", "platform", "windows")

	return limits, nil
}


