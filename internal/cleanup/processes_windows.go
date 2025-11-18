//go:build windows
// +build windows

package cleanup

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"dbbackup/internal/logger"
)

// KillOrphanedProcesses finds and kills any orphaned pg_dump, pg_restore, gzip, or pigz processes (Windows implementation)
func KillOrphanedProcesses(log logger.Logger) error {
	processNames := []string{"pg_dump.exe", "pg_restore.exe", "gzip.exe", "pigz.exe", "gunzip.exe"}
	
	myPID := os.Getpid()
	var killed []string
	var errors []error
	
	for _, procName := range processNames {
		pids, err := findProcessesByNameWindows(procName, myPID)
		if err != nil {
			log.Warn("Failed to search for processes", "process", procName, "error", err)
			continue
		}
		
		for _, pid := range pids {
			if err := killProcessWindows(pid); err != nil {
				errors = append(errors, fmt.Errorf("failed to kill %s (PID %d): %w", procName, pid, err))
			} else {
				killed = append(killed, fmt.Sprintf("%s (PID %d)", procName, pid))
			}
		}
	}
	
	if len(killed) > 0 {
		log.Info("Cleaned up orphaned processes", "count", len(killed), "processes", strings.Join(killed, ", "))
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("some processes could not be killed: %v", errors)
	}
	
	return nil
}

// findProcessesByNameWindows returns PIDs of processes matching the given name (Windows implementation)
func findProcessesByNameWindows(name string, excludePID int) ([]int, error) {
	// Use tasklist command for Windows
	cmd := exec.Command("tasklist", "/FO", "CSV", "/NH", "/FI", fmt.Sprintf("IMAGENAME eq %s", name))
	output, err := cmd.Output()
	if err != nil {
		// No processes found or command failed
		return []int{}, nil
	}
	
	var pids []int
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		// Parse CSV output: "name","pid","session","mem"
		fields := strings.Split(line, ",")
		if len(fields) < 2 {
			continue
		}
		
		// Remove quotes from PID field
		pidStr := strings.Trim(fields[1], `"`)
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		
		// Don't kill our own process
		if pid == excludePID {
			continue
		}
		
		pids = append(pids, pid)
	}
	
	return pids, nil
}

// killProcessWindows kills a process on Windows
func killProcessWindows(pid int) error {
	// Use taskkill command
	cmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
	return cmd.Run()
}

// SetProcessGroup sets up process group for Windows (no-op, Windows doesn't use Unix process groups)
func SetProcessGroup(cmd *exec.Cmd) {
	// Windows doesn't support Unix-style process groups
	// We can set CREATE_NEW_PROCESS_GROUP flag instead
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// KillCommandGroup kills a command on Windows
func KillCommandGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	
	// On Windows, just kill the process directly
	return cmd.Process.Kill()
}