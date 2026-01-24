// Package ui implements the terminal user interface for spaceview using Bubbletea.
package ui

// Import TUI dependencies to ensure they are tracked in go.mod.
// These will be properly used in the UI components.
import (
	_ "github.com/charmbracelet/bubbles/key"
	_ "github.com/charmbracelet/bubbletea"
	_ "github.com/charmbracelet/lipgloss"
)
