//go:build !windows && !darwin

package ui

// openInFileManager is a no-op on unsupported platforms
func openInFileManager(path string) error {
	// Not implemented for this platform
	return nil
}
