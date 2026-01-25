//go:build windows

package tui

import "os/exec"

// openInFileManager reveals the given path in Windows Explorer (opens parent directory with item selected)
func openInFileManager(path string) error {
	// explorer /select,path - opens folder with item selected
	return exec.Command("explorer", "/select,"+path).Start()
}
