//go:build darwin

package tui

import "os/exec"

// openInFileManager reveals the given path in Finder (opens parent directory with item selected)
func openInFileManager(path string) error {
	cmd := exec.Command("open", "-R", path)
	return cmd.Start()
}
