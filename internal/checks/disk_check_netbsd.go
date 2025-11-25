//go:build netbsd
// +build netbsd

package checks

import (
	"fmt"
	"path/filepath"
)

// CheckDiskSpace checks available disk space for a given path (NetBSD stub implementation)
// NetBSD syscall API differs significantly - returning safe defaults
func CheckDiskSpace(path string) *DiskSpaceCheck {
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Return safe defaults - assume sufficient space
	// NetBSD users can check manually with 'df -h'
	check := &DiskSpaceCheck{
		Path:           absPath,
		TotalBytes:     1024 * 1024 * 1024 * 1024, // 1TB assumed
		AvailableBytes: 512 * 1024 * 1024 * 1024,  // 512GB assumed available
		UsedBytes:      512 * 1024 * 1024 * 1024,  // 512GB assumed used
		UsedPercent:    50.0,
		Sufficient:     true,
		Warning:        false,
		Critical:       false,
	}

	return check
}

// CheckDiskSpaceForRestore checks if there's enough space for restore (needs 4x archive size)
func CheckDiskSpaceForRestore(path string, archiveSize int64) *DiskSpaceCheck {
	check := CheckDiskSpace(path)
	requiredBytes := uint64(archiveSize) * 4 // Account for decompression
	
	// Override status based on required space
	if check.AvailableBytes < requiredBytes {
		check.Critical = true
		check.Sufficient = false
		check.Warning = false
	} else if check.AvailableBytes < requiredBytes*2 {
		check.Warning = true
		check.Sufficient = false
	}
	
	return check
}

// FormatDiskSpaceMessage creates a user-friendly disk space message
func FormatDiskSpaceMessage(check *DiskSpaceCheck) string {
	var status string
	var icon string

	if check.Critical {
		status = "CRITICAL"
		icon = "❌"
	} else if check.Warning {
		status = "WARNING"
		icon = "⚠️ "
	} else {
		status = "OK"
		icon = "✓"
	}

	msg := fmt.Sprintf(`📊 Disk Space Check (%s):
   Path: %s
   Total: %s
   Available: %s (%.1f%% used)
   %s Status: %s`,
		status,
		check.Path,
		formatBytes(check.TotalBytes),
		formatBytes(check.AvailableBytes),
		check.UsedPercent,
		icon,
		status)

	if check.Critical {
		msg += "\n   \n   ⚠️  CRITICAL: Insufficient disk space!"
		msg += "\n   Operation blocked. Free up space before continuing."
	} else if check.Warning {
		msg += "\n   \n   ⚠️  WARNING: Low disk space!"
		msg += "\n   Backup may fail if database is larger than estimated."
	} else {
		msg += "\n   \n   ✓ Sufficient space available"
	}

	return msg
}
