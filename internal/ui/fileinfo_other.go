//go:build !darwin

package ui

import (
	"os"
	"time"
)

// getCreationTime returns zero time on platforms that don't support birthtime
// Windows does support creation time but would need different syscall handling
func getCreationTime(info os.FileInfo) time.Time {
	return time.Time{}
}
