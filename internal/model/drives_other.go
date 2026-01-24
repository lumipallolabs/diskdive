//go:build !windows

package model

func getDiskSpace(path string) (total, free int64) {
	// Placeholder for Unix implementation
	return 0, 0
}
