//go:build !darwin && !windows

package tui

import "os/exec"

// previewInQuickLook opens the file with xdg-open on Linux
func previewInQuickLook(path string) error {
	return exec.Command("xdg-open", path).Start()
}
