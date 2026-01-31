//go:build darwin

package model

import (
	"os"
	"path/filepath"
	"syscall"
)

// GetDiskSpace returns disk space information for a given path using statfs
func GetDiskSpace(path string) (total, free int64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}

	// Calculate total and free bytes
	total = int64(stat.Blocks) * int64(stat.Bsize)
	free = int64(stat.Bavail) * int64(stat.Bsize)
	return total, free
}

func getPlatformDrives() ([]Drive, error) {
	var drives []Drive

	// Add root filesystem first
	rootDrive := Drive{
		Letter: "Macintosh HD",
		Path:   "/",
		Label:  "Macintosh HD",
	}
	rootDrive.TotalBytes, rootDrive.FreeBytes = GetDiskSpace("/")
	drives = append(drives, rootDrive)

	// Scan /Volumes for mounted drives
	volumesDir := "/Volumes"
	entries, err := os.ReadDir(volumesDir)
	if err != nil {
		// If we can't read /Volumes, just return root
		return drives, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		volumePath := filepath.Join(volumesDir, entry.Name())

		// Get filesystem info to filter out network/pseudo filesystems
		var stat syscall.Statfs_t
		if err := syscall.Statfs(volumePath, &stat); err != nil {
			// Skip volumes we can't access
			continue
		}

		// Filter out non-physical filesystems
		fsType := int8ArrayToString(stat.Fstypename[:])
		if isFilteredFilesystem(fsType) {
			continue
		}

		drive := Drive{
			Letter: entry.Name(),
			Path:   volumePath,
			Label:  entry.Name(),
		}
		drive.TotalBytes, drive.FreeBytes = GetDiskSpace(volumePath)

		// Only add if we got valid disk space info
		if drive.TotalBytes > 0 {
			drives = append(drives, drive)
		}
	}

	return drives, nil
}

// int8ArrayToString converts an int8 array to a string
func int8ArrayToString(arr []int8) string {
	b := make([]byte, 0, len(arr))
	for _, v := range arr {
		if v == 0 {
			break
		}
		b = append(b, byte(v))
	}
	return string(b)
}

// isFilteredFilesystem returns true if the filesystem type should be filtered out
func isFilteredFilesystem(fsType string) bool {
	// Network filesystems
	networkFS := []string{"smbfs", "nfs", "afpfs", "webdav", "cifs"}
	for _, nfs := range networkFS {
		if fsType == nfs {
			return true
		}
	}

	// Pseudo filesystems
	pseudoFS := []string{"devfs", "autofs", "mtmfs", "nullfs"}
	for _, pfs := range pseudoFS {
		if fsType == pfs {
			return true
		}
	}

	return false
}
