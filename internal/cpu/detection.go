package cpu

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"os"
	"os/exec"
	"bufio"
)

// CPUInfo holds information about the system CPU
type CPUInfo struct {
	LogicalCores  int     `json:"logical_cores"`
	PhysicalCores int     `json:"physical_cores"`
	Architecture  string  `json:"architecture"`
	ModelName     string  `json:"model_name"`
	MaxFrequency  float64 `json:"max_frequency_mhz"`
	CacheSize     string  `json:"cache_size"`
	Vendor        string  `json:"vendor"`
	Features      []string `json:"features"`
}

// Detector provides CPU detection functionality
type Detector struct {
	info *CPUInfo
}

// NewDetector creates a new CPU detector
func NewDetector() *Detector {
	return &Detector{}
}

// DetectCPU detects CPU information for the current system
func (d *Detector) DetectCPU() (*CPUInfo, error) {
	if d.info != nil {
		return d.info, nil
	}

	info := &CPUInfo{
		LogicalCores: runtime.NumCPU(),
		Architecture: runtime.GOARCH,
	}

	// Platform-specific detection
	switch runtime.GOOS {
	case "linux":
		if err := d.detectLinux(info); err != nil {
			return info, fmt.Errorf("linux CPU detection failed: %w", err)
		}
	case "darwin":
		if err := d.detectDarwin(info); err != nil {
			return info, fmt.Errorf("darwin CPU detection failed: %w", err)
		}
	case "windows":
		if err := d.detectWindows(info); err != nil {
			return info, fmt.Errorf("windows CPU detection failed: %w", err)
		}
	default:
		// Fallback for unsupported platforms
		info.PhysicalCores = info.LogicalCores
		info.ModelName = "Unknown"
		info.Vendor = "Unknown"
	}

	d.info = info
	return info, nil
}

// detectLinux detects CPU information on Linux systems
func (d *Detector) detectLinux(info *CPUInfo) error {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	physicalCoreCount := make(map[string]bool)
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "model name":
			if info.ModelName == "" {
				info.ModelName = value
			}
		case "vendor_id":
			if info.Vendor == "" {
				info.Vendor = value
			}
		case "cpu MHz":
			if freq, err := strconv.ParseFloat(value, 64); err == nil && info.MaxFrequency < freq {
				info.MaxFrequency = freq
			}
		case "cache size":
			if info.CacheSize == "" {
				info.CacheSize = value
			}
		case "flags", "Features":
			if len(info.Features) == 0 {
				info.Features = strings.Fields(value)
			}
		case "physical id":
			physicalCoreCount[value] = true
		}
	}

	// Calculate physical cores
	if len(physicalCoreCount) > 0 {
		info.PhysicalCores = len(physicalCoreCount)
	} else {
		// Fallback: assume hyperthreading if logical > 1
		info.PhysicalCores = info.LogicalCores
		if info.LogicalCores > 1 {
			info.PhysicalCores = info.LogicalCores / 2
		}
	}

	// Try to get more accurate physical core count from lscpu
	if cmd := exec.Command("lscpu"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			d.parseLscpu(string(output), info)
		}
	}

	return scanner.Err()
}

// parseLscpu parses lscpu output for more accurate CPU information
func (d *Detector) parseLscpu(output string, info *CPUInfo) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Core(s) per socket":
			if cores, err := strconv.Atoi(value); err == nil {
				if sockets := d.getSocketCount(output); sockets > 0 {
					info.PhysicalCores = cores * sockets
				}
			}
		case "Model name":
			info.ModelName = value
		case "Vendor ID":
			info.Vendor = value
		case "CPU max MHz":
			if freq, err := strconv.ParseFloat(value, 64); err == nil {
				info.MaxFrequency = freq
			}
		}
	}
}

// getSocketCount extracts socket count from lscpu output
func (d *Detector) getSocketCount(output string) int {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Socket(s):") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				if sockets, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
					return sockets
				}
			}
		}
	}
	return 1 // Default to 1 socket
}

// detectDarwin detects CPU information on macOS systems
func (d *Detector) detectDarwin(info *CPUInfo) error {
	// Get CPU brand
	if output, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
		info.ModelName = strings.TrimSpace(string(output))
	}

	// Get physical cores
	if output, err := exec.Command("sysctl", "-n", "hw.physicalcpu").Output(); err == nil {
		if cores, err := strconv.Atoi(strings.TrimSpace(string(output))); err == nil {
			info.PhysicalCores = cores
		}
	}

	// Get max frequency
	if output, err := exec.Command("sysctl", "-n", "hw.cpufrequency_max").Output(); err == nil {
		if freq, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64); err == nil {
			info.MaxFrequency = freq / 1000000 // Convert Hz to MHz
		}
	}

	// Get vendor
	if output, err := exec.Command("sysctl", "-n", "machdep.cpu.vendor").Output(); err == nil {
		info.Vendor = strings.TrimSpace(string(output))
	}

	// Get cache size
	if output, err := exec.Command("sysctl", "-n", "hw.l3cachesize").Output(); err == nil {
		if cache, err := strconv.Atoi(strings.TrimSpace(string(output))); err == nil {
			info.CacheSize = fmt.Sprintf("%d KB", cache/1024)
		}
	}

	return nil
}

// detectWindows detects CPU information on Windows systems
func (d *Detector) detectWindows(info *CPUInfo) error {
	// Use wmic to get CPU information
	cmd := exec.Command("wmic", "cpu", "get", "Name,NumberOfCores,NumberOfLogicalProcessors,MaxClockSpeed", "/format:list")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Name":
			if value != "" {
				info.ModelName = value
			}
		case "NumberOfCores":
			if cores, err := strconv.Atoi(value); err == nil {
				info.PhysicalCores = cores
			}
		case "MaxClockSpeed":
			if freq, err := strconv.ParseFloat(value, 64); err == nil {
				info.MaxFrequency = freq
			}
		}
	}

	// Get vendor information
	cmd = exec.Command("wmic", "cpu", "get", "Manufacturer", "/format:list")
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Manufacturer=") {
				info.Vendor = strings.TrimSpace(strings.SplitN(line, "=", 2)[1])
				break
			}
		}
	}

	return nil
}

// CalculateOptimalJobs calculates optimal job count based on CPU info and workload type
func (d *Detector) CalculateOptimalJobs(workloadType string, maxJobs int) (int, error) {
	info, err := d.DetectCPU()
	if err != nil {
		return 1, err
	}

	var optimal int

	switch workloadType {
	case "cpu-intensive":
		// For CPU-intensive tasks, use physical cores
		optimal = info.PhysicalCores
	case "io-intensive":
		// For I/O intensive tasks, can use more jobs than cores
		optimal = info.LogicalCores * 2
	case "balanced":
		// Balanced workload, use logical cores
		optimal = info.LogicalCores
	default:
		optimal = info.LogicalCores
	}

	// Apply safety limits
	if optimal < 1 {
		optimal = 1
	}
	if maxJobs > 0 && optimal > maxJobs {
		optimal = maxJobs
	}

	return optimal, nil
}

// GetCPUInfo returns the detected CPU information
func (d *Detector) GetCPUInfo() *CPUInfo {
	return d.info
}

// FormatCPUInfo returns a formatted string representation of CPU info
func (info *CPUInfo) FormatCPUInfo() string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("Architecture: %s\n", info.Architecture))
	sb.WriteString(fmt.Sprintf("Logical Cores: %d\n", info.LogicalCores))
	sb.WriteString(fmt.Sprintf("Physical Cores: %d\n", info.PhysicalCores))
	
	if info.ModelName != "" {
		sb.WriteString(fmt.Sprintf("Model: %s\n", info.ModelName))
	}
	if info.Vendor != "" {
		sb.WriteString(fmt.Sprintf("Vendor: %s\n", info.Vendor))
	}
	if info.MaxFrequency > 0 {
		sb.WriteString(fmt.Sprintf("Max Frequency: %.2f MHz\n", info.MaxFrequency))
	}
	if info.CacheSize != "" {
		sb.WriteString(fmt.Sprintf("Cache Size: %s\n", info.CacheSize))
	}
	
	return sb.String()
}