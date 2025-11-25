// go:build !linux
// +build !linux

package security

// checkVirtualMemoryLimit is a no-op on non-Linux systems (RLIMIT_AS not available)
func checkVirtualMemoryLimit(minVirtualMemoryMB uint64) error {
	return nil
}
