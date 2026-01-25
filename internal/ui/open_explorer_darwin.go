//go:build darwin

package ui

import "os/exec"

// openInFileManager opens the given path in Finder
func openInFileManager(path string) error {
	cmd := exec.Command("open", path)
	return cmd.Start()
}
