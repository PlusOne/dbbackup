//go:build !windows
// +build !windows

package cleanup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"dbbackup/internal/logger"
)

// ProcessManager tracks and manages process lifecycle safely
type ProcessManager struct {
	mu        sync.RWMutex
	processes map[int]*os.Process
	ctx       context.Context
	cancel    context.CancelFunc
	log       logger.Logger
}

// NewProcessManager creates a new process manager
func NewProcessManager(log logger.Logger) *ProcessManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ProcessManager{
		processes: make(map[int]*os.Process),
		ctx:       ctx,
		cancel:    cancel,
		log:       log,
	}
}

// Track adds a process to be managed
func (pm *ProcessManager) Track(proc *os.Process) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.processes[proc.Pid] = proc
	
	// Auto-cleanup when process exits
	go func() {
		proc.Wait()
		pm.mu.Lock()
		delete(pm.processes, proc.Pid)
		pm.mu.Unlock()
	}()
}

// KillAll kills all tracked processes
func (pm *ProcessManager) KillAll() error {
	pm.mu.RLock()
	procs := make([]*os.Process, 0, len(pm.processes))
	for _, proc := range pm.processes {
		procs = append(procs, proc)
	}
	pm.mu.RUnlock()
	
	var errors []error
	for _, proc := range procs {
		if err := proc.Kill(); err != nil {
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to kill %d processes: %v", len(errors), errors)
	}
	return nil
}

// Close cleans up the process manager
func (pm *ProcessManager) Close() error {
	pm.cancel()
	return pm.KillAll()
}

// KillOrphanedProcesses finds and kills any orphaned pg_dump, pg_restore, gzip, or pigz processes
func KillOrphanedProcesses(log logger.Logger) error {
	processNames := []string{"pg_dump", "pg_restore", "gzip", "pigz", "gunzip"}
	
	myPID := os.Getpid()
	var killed []string
	var errors []error
	
	for _, procName := range processNames {
		pids, err := findProcessesByName(procName, myPID)
		if err != nil {
			log.Warn("Failed to search for processes", "process", procName, "error", err)
			continue
		}
		
		for _, pid := range pids {
			if err := killProcessGroup(pid); err != nil {
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

// findProcessesByName returns PIDs of processes matching the given name
func findProcessesByName(name string, excludePID int) ([]int, error) {
	// Use pgrep for efficient process searching
	cmd := exec.Command("pgrep", "-x", name)
	output, err := cmd.Output()
	if err != nil {
		// Exit code 1 means no processes found (not an error)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []int{}, nil
		}
		return nil, err
	}
	
	var pids []int
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		pid, err := strconv.Atoi(line)
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

// killProcessGroup kills a process and its entire process group
func killProcessGroup(pid int) error {
	// First try to get the process group ID
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		// Process might already be gone
		return nil
	}
	
	// Kill the entire process group (negative PID kills the group)
	// This catches pipelines like "pg_dump | gzip"
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		// If SIGTERM fails, try SIGKILL
		syscall.Kill(-pgid, syscall.SIGKILL)
	}
	
	// Also kill the specific PID in case it's not in a group
	syscall.Kill(pid, syscall.SIGTERM)
	
	return nil
}

// SetProcessGroup sets the current process to be a process group leader
// This should be called when starting external commands to ensure clean termination
func SetProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0, // Create new process group
	}
}

// KillCommandGroup kills a command and its entire process group
func KillCommandGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	
	pid := cmd.Process.Pid
	
	// Get the process group ID
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		// Process might already be gone
		return nil
	}
	
	// Kill the entire process group
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		// If SIGTERM fails, use SIGKILL
		syscall.Kill(-pgid, syscall.SIGKILL)
	}
	
	return nil
}
