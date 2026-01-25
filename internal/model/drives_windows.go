//go:build windows

package model

import (
	"syscall"
	"unsafe"
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceExW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func getPlatformDrives() ([]Drive, error) {
	return getWindowsDrives()
}

func getDiskSpace(path string) (total, free int64) {
	pathPtr, _ := syscall.UTF16PtrFromString(path)

	var freeBytesAvailable, totalBytes, totalFreeBytes int64

	ret, _, _ := getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)

	if ret == 0 {
		return 0, 0
	}

	return totalBytes, freeBytesAvailable
}
