// +build openbsd

package restore

import "syscall"

// getDiskSpace returns available disk space in bytes (OpenBSD)
func getDiskSpace(path string) (int64, error) {
	var statfs syscall.Statfs_t
	if err := syscall.Statfs(path, &statfs); err != nil {
		return 0, err
	}
	// OpenBSD uses F_bavail and F_bsize
	return int64(statfs.F_bavail) * int64(statfs.F_bsize), nil
}
