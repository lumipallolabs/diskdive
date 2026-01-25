//go:build windows

package ui

import "os/exec"

// openInFileManager opens the given path in Windows Explorer
func openInFileManager(path string) error {
	cmd := exec.Command("explorer.exe", path)
	return cmd.Start()
}
