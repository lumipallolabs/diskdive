//go:build !windows && !darwin

package model

import "syscall"

func getPlatformDrives() ([]Drive, error) {
	return getUnixMounts()
}

// GetDiskSpace returns disk space information for a given path using statfs
func GetDiskSpace(path string) (total, free int64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}
	total = int64(stat.Blocks) * int64(stat.Bsize)
	free = int64(stat.Bavail) * int64(stat.Bsize)
	return total, free
}
