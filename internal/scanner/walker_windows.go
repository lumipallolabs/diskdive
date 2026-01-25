//go:build windows

package scanner

import (
	"io/fs"
	"sync"
)

// platformRootInfo holds platform-specific root information
type platformRootInfo struct {
	// Windows doesn't need mount point detection - drives are separate
}

// getPlatformRootInfo returns platform-specific info about the root path
func getPlatformRootInfo(path string) platformRootInfo {
	return platformRootInfo{}
}

// shouldSkipDir returns true if the directory should be skipped
// On Windows, we don't need to check for mount points since drives are separate
func shouldSkipDir(path string, d fs.DirEntry, rootInfo platformRootInfo, seenItems *sync.Map) bool {
	return false
}

// getFileSize returns the file size, or -1 if the file should be skipped
func getFileSize(info fs.FileInfo, seenItems *sync.Map) int64 {
	return info.Size()
}
