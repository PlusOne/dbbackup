// +build !windows,!openbsd,!netbsd

package restore

import "syscall"

// getDiskSpace returns available disk space in bytes (Unix/Linux/macOS/FreeBSD)
func getDiskSpace(path string) (int64, error) {
	var statfs syscall.Statfs_t
	if err := syscall.Statfs(path, &statfs); err != nil {
		return 0, err
	}
	return int64(statfs.Bavail) * int64(statfs.Bsize), nil
}
