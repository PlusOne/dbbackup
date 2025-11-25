// +build netbsd

package restore

// getDiskSpace returns available disk space in bytes (NetBSD fallback)
// NetBSD has different syscall interface, so we use a conservative estimate
func getDiskSpace(path string) (int64, error) {
	// Return a large value to skip disk space check on NetBSD
	// This is a fallback - restore will still work, just won't pre-check space
	return 1 << 40, nil // 1 TB
}
