//go:build !windows

package scanner

import (
	"io/fs"
	"sync"
	"syscall"
)

// platformRootInfo holds platform-specific root information
type platformRootInfo struct {
	dev uint64
}

// getPlatformRootInfo returns platform-specific info about the root path
func getPlatformRootInfo(path string) platformRootInfo {
	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err != nil {
		return platformRootInfo{}
	}
	return platformRootInfo{dev: uint64(stat.Dev)}
}

// shouldSkipDir returns true if the directory should be skipped
func shouldSkipDir(path string, d fs.DirEntry, rootInfo platformRootInfo, seenItems *sync.Map) bool {
	info, err := d.Info()
	if err != nil {
		return false
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}

	// Skip if different filesystem (mount point)
	if uint64(stat.Dev) != rootInfo.dev {
		return true
	}

	// Skip if already seen this inode (firmlinks on macOS)
	inode := stat.Ino
	if _, exists := seenItems.LoadOrStore(inode, true); exists {
		return true
	}

	return false
}

// getFileSize returns the file size, or -1 if the file should be skipped
func getFileSize(info fs.FileInfo, seenItems *sync.Map) int64 {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return info.Size()
	}

	// Check for hard links (nlink > 1)
	if stat.Nlink > 1 {
		inode := stat.Ino
		if _, exists := seenItems.LoadOrStore(inode, true); exists {
			return -1 // Already counted
		}
	}

	// Use actual blocks allocated (handles sparse files)
	// Blocks is in 512-byte units
	return stat.Blocks * 512
}
