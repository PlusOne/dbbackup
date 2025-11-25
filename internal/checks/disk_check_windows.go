//go:build windows
// +build windows

package checks

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// CheckDiskSpace checks available disk space for a given path (Windows implementation)
func CheckDiskSpace(path string) *DiskSpaceCheck {
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Get the drive root (e.g., "C:\")
	vol := filepath.VolumeName(absPath)
	if vol == "" {
		// If no volume, try current directory
		vol = "."
	}
	
	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64

	// Call Windows API
	pathPtr, _ := syscall.UTF16PtrFromString(vol)
	ret, _, _ := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)))

	if ret == 0 {
		// API call failed, return error state
		return &DiskSpaceCheck{
			Path:       absPath,
			Critical:   true,
			Sufficient: false,
		}
	}

	// Calculate usage
	usedBytes := totalNumberOfBytes - totalNumberOfFreeBytes
	usedPercent := float64(usedBytes) / float64(totalNumberOfBytes) * 100

	check := &DiskSpaceCheck{
		Path:           absPath,
		TotalBytes:     totalNumberOfBytes,
		AvailableBytes: freeBytesAvailable,
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

