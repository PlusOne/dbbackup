//go:build openbsd || netbsd
// +build openbsd netbsd

package checks

import (
	"fmt"
	"path/filepath"
	"syscall"
)

// CheckDiskSpace checks available disk space for a given path (OpenBSD/NetBSD implementation)
func CheckDiskSpace(path string) *DiskSpaceCheck {
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Get filesystem stats
	var stat syscall.Statfs_t
	if err := syscall.Statfs(absPath, &stat); err != nil {
		// Return error state
		return &DiskSpaceCheck{
			Path:       absPath,
			Critical:   true,
			Sufficient: false,
		}
	}

	// Calculate space (OpenBSD/NetBSD use different field names)
	totalBytes := uint64(stat.F_blocks) * uint64(stat.F_bsize)
	availableBytes := uint64(stat.F_bavail) * uint64(stat.F_bsize)
	usedBytes := totalBytes - availableBytes
	usedPercent := float64(usedBytes) / float64(totalBytes) * 100

	check := &DiskSpaceCheck{
		Path:           absPath,
		TotalBytes:     totalBytes,
		AvailableBytes: availableBytes,
		UsedBytes:      usedBytes,
		UsedPercent:    usedPercent,
	}

	// Determine status thresholds
	check.Critical = usedPercent >= 95
	check.Warning = usedPercent >= 80 && !check.Critical
	check.Sufficient = !check.Critical && !check.Warning

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