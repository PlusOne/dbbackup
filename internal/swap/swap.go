package swap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"dbbackup/internal/logger"
)

// Manager handles temporary swap file creation and cleanup
type Manager struct {
	swapPath   string
	sizeGB     int
	isActive   bool
	wasCreated bool
	log        logger.Logger
}

// NewManager creates a new swap file manager
func NewManager(swapPath string, sizeGB int, log logger.Logger) *Manager {
	return &Manager{
		swapPath: swapPath,
		sizeGB:   sizeGB,
		log:      log,
	}
}

// IsSupported checks if swap file management is supported on this platform
func (m *Manager) IsSupported() bool {
	// Only supported on Linux
	return runtime.GOOS == "linux"
}

// NeedsRoot checks if we need root privileges for swap operations
func (m *Manager) NeedsRoot() bool {
	// On Linux, swap operations require root
	return runtime.GOOS == "linux" && os.Geteuid() != 0
}

// GetCurrentSwap returns current swap usage info
func (m *Manager) GetCurrentSwap() (totalMB, usedMB, freeMB int64, err error) {
	// Read /proc/meminfo for swap info
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch fields[0] {
		case "SwapTotal:":
			if val, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				totalMB = val / 1024
			}
		case "SwapFree:":
			if val, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				freeMB = val / 1024
			}
		}
	}
	usedMB = totalMB - freeMB
	return
}

// IsSwapFileActive checks if our specific swap file is currently active
func (m *Manager) IsSwapFileActive() (bool, error) {
	// Read /proc/swaps to see active swap devices
	data, err := os.ReadFile("/proc/swaps")
	if err != nil {
		return false, err
	}

	absPath, _ := filepath.Abs(m.swapPath)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, absPath) || strings.Contains(line, m.swapPath) {
			return true, nil
		}
	}
	return false, nil
}

// Setup creates and enables the swap file if needed
func (m *Manager) Setup() error {
	if !m.IsSupported() {
		return fmt.Errorf("swap file management not supported on %s", runtime.GOOS)
	}

	if m.sizeGB <= 0 {
		return fmt.Errorf("swap size must be > 0 GB")
	}

	if m.NeedsRoot() {
		m.log.Warn("Swap file creation requires root privileges, skipping automatic swap setup")
		return fmt.Errorf("swap file operations require root privileges (current euid: %d)", os.Geteuid())
	}

	m.log.Info("Setting up temporary swap file", "path", m.swapPath, "size_gb", m.sizeGB)

	// Check if swap file already exists and is active
	if active, _ := m.IsSwapFileActive(); active {
		m.log.Info("Swap file already active", "path", m.swapPath)
		m.isActive = true
		return nil
	}

	// Check if file exists but is not active
	if _, err := os.Stat(m.swapPath); err == nil {
		m.log.Warn("Swap file exists but is not active, removing", "path", m.swapPath)
		if err := os.Remove(m.swapPath); err != nil {
			return fmt.Errorf("failed to remove existing swap file: %w", err)
		}
	}

	// Create swap file directory if needed
	swapDir := filepath.Dir(m.swapPath)
	if err := os.MkdirAll(swapDir, 0755); err != nil {
		return fmt.Errorf("failed to create swap directory: %w", err)
	}

	// Calculate size in bytes
	sizeBytes := int64(m.sizeGB) * 1024 * 1024 * 1024

	m.log.Info("Creating swap file", "size_bytes", sizeBytes)

	// Use fallocate for fast file creation
	cmd := exec.Command("fallocate", "-l", fmt.Sprintf("%d", sizeBytes), m.swapPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Fallback to dd if fallocate fails
		m.log.Warn("fallocate failed, using dd (slower)", "error", err, "output", string(output))
		cmd = exec.Command("dd", "if=/dev/zero", "of="+m.swapPath, "bs=1M", fmt.Sprintf("count=%d", m.sizeGB*1024))
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create swap file with dd: %w (output: %s)", err, string(output))
		}
	}
	m.wasCreated = true

	// Set correct permissions (600 for security)
	if err := os.Chmod(m.swapPath, 0600); err != nil {
		m.cleanup()
		return fmt.Errorf("failed to set swap file permissions: %w", err)
	}

	// Format as swap
	m.log.Info("Formatting swap file")
	cmd = exec.Command("mkswap", m.swapPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		m.cleanup()
		return fmt.Errorf("failed to format swap file: %w (output: %s)", err, string(output))
	}

	// Enable swap
	m.log.Info("Enabling swap file")
	cmd = exec.Command("swapon", m.swapPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		m.cleanup()
		return fmt.Errorf("failed to enable swap file: %w (output: %s)", err, string(output))
	}

	m.isActive = true

	// Log current swap status
	if total, used, free, err := m.GetCurrentSwap(); err == nil {
		m.log.Info("Swap status after setup", 
			"total_mb", total, 
			"used_mb", used, 
			"free_mb", free,
			"added_gb", m.sizeGB)
	}

	return nil
}

// cleanup removes the swap file (internal helper)
func (m *Manager) cleanup() {
	if m.wasCreated {
		os.Remove(m.swapPath)
		m.wasCreated = false
	}
}

// Cleanup disables and removes the swap file
func (m *Manager) Cleanup() error {
	if !m.isActive {
		return nil
	}

	m.log.Info("Cleaning up swap file", "path", m.swapPath)

	// Check if still active
	if active, _ := m.IsSwapFileActive(); active {
		// Disable swap
		m.log.Info("Disabling swap file")
		cmd := exec.Command("swapoff", m.swapPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			m.log.Warn("Failed to disable swap file", "error", err, "output", string(output))
			// Continue anyway
		}
	}

	// Remove file if we created it
	if m.wasCreated {
		m.log.Info("Removing swap file", "path", m.swapPath)
		if err := os.Remove(m.swapPath); err != nil {
			m.log.Warn("Failed to remove swap file", "error", err)
			return fmt.Errorf("failed to remove swap file: %w", err)
		}
		m.wasCreated = false
	}

	m.isActive = false
	return nil
}

// IsActive returns whether the swap file is currently active
func (m *Manager) IsActive() bool {
	return m.isActive
}

// WasCreated returns whether this manager created the swap file
func (m *Manager) WasCreated() bool {
	return m.wasCreated
}
