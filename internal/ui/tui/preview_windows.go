//go:build windows

package tui

import "os/exec"

// previewInQuickLook opens the file with Windows default viewer
func previewInQuickLook(path string) error {
	return exec.Command("cmd", "/c", "start", "", path).Start()
}
