// go:build linux
// +build linux

package security

import "syscall"

// checkVirtualMemoryLimit checks RLIMIT_AS (only available on Linux)
func checkVirtualMemoryLimit(minVirtualMemoryMB uint64) error {
	var vmLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_AS, &vmLimit); err == nil {
		if vmLimit.Cur != syscall.RLIM_INFINITY && vmLimit.Cur < minVirtualMemoryMB*1024*1024 {
			return formatError("virtual memory limit too low: %s (minimum: %d MB)",
				formatBytes(uint64(vmLimit.Cur)), minVirtualMemoryMB)
		}
	}
	return nil
}
