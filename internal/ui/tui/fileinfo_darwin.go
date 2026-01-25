//go:build darwin

package tui

import (
	"os"
	"syscall"
	"time"
)

// getCreationTime returns the file creation time (birthtime) on macOS
func getCreationTime(info os.FileInfo) time.Time {
	if sys := info.Sys(); sys != nil {
		if stat, ok := sys.(*syscall.Stat_t); ok {
			return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
		}
	}
	return time.Time{}
}
