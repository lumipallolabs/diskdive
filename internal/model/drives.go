package model

import (
	"fmt"
	"os"
	"runtime"
)

// Drive represents a mounted drive/volume
type Drive struct {
	Letter     string // e.g., "C"
	Path       string // e.g., "C:\\"
	Label      string // volume label
	TotalBytes int64
	FreeBytes  int64
}

// UsedBytes returns bytes used on this drive
func (d Drive) UsedBytes() int64 {
	return d.TotalBytes - d.FreeBytes
}

// UsedPercent returns percentage of drive used
func (d Drive) UsedPercent() float64 {
	if d.TotalBytes == 0 {
		return 0
	}
	return float64(d.UsedBytes()) / float64(d.TotalBytes) * 100
}

// GetDrives returns all available drives on the system
func GetDrives() ([]Drive, error) {
	return getPlatformDrives()
}

func getWindowsDrives() ([]Drive, error) {
	var drives []Drive

	for letter := 'A'; letter <= 'Z'; letter++ {
		path := fmt.Sprintf("%c:\\", letter)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			continue
		}

		drive := Drive{
			Letter: string(letter),
			Path:   path,
		}

		// Get disk space info using syscall (Windows-specific)
		drive.TotalBytes, drive.FreeBytes = getDiskSpace(path)

		drives = append(drives, drive)
	}

	return drives, nil
}

func getUnixMounts() ([]Drive, error) {
	// Placeholder for future Linux support
	// For now, just return root
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/"
	}
	return []Drive{
		{Letter: runtime.GOOS, Path: home, Label: home},
	}, nil
}
