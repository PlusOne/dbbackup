package checks

import (
	"fmt"
	"path/filepath"
	"syscall"
)

// DiskSpaceCheck represents disk space information
type DiskSpaceCheck struct {
	Path           string
	TotalBytes     uint64
	AvailableBytes uint64
	UsedBytes      uint64
	UsedPercent    float64
	Sufficient     bool
	Warning        bool
	Critical       bool
}

// CheckDiskSpace checks available disk space for a given path
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

	// Calculate space
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	availableBytes := stat.Bavail * uint64(stat.Bsize)
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

// EstimateBackupSize estimates backup size based on database size
func EstimateBackupSize(databaseSize uint64, compressionLevel int) uint64 {
	// Typical compression ratios:
	// Level 0 (no compression): 1.0x
	// Level 1-3 (fast): 0.4-0.6x
	// Level 4-6 (balanced): 0.3-0.4x
	// Level 7-9 (best): 0.2-0.3x

	var compressionRatio float64
	if compressionLevel == 0 {
		compressionRatio = 1.0
	} else if compressionLevel <= 3 {
		compressionRatio = 0.5
	} else if compressionLevel <= 6 {
		compressionRatio = 0.35
	} else {
		compressionRatio = 0.25
	}

	estimated := uint64(float64(databaseSize) * compressionRatio)

	// Add 10% buffer for metadata, indexes, etc.
	return uint64(float64(estimated) * 1.1)
}



// formatBytes formats bytes to human-readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
