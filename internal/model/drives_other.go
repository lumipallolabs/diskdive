//go:build !windows && !darwin

package model

func getPlatformDrives() ([]Drive, error) {
	return getUnixMounts()
}

func getDiskSpace(path string) (total, free int64) {
	// Placeholder for Unix implementation
	return 0, 0
}
