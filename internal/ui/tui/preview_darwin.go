//go:build darwin

package tui

import "os/exec"

// previewInQuickLook opens the file in macOS Quick Look
func previewInQuickLook(path string) error {
	return exec.Command("qlmanage", "-p", path).Start()
}
