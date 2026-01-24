package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// HelpOverlay displays keyboard shortcuts in a centered overlay
type HelpOverlay struct {
	visible bool
	width   int
	height  int
}

// NewHelpOverlay creates a new help overlay component
func NewHelpOverlay() HelpOverlay {
	return HelpOverlay{
		visible: false,
	}
}

// Toggle toggles the visibility of the help overlay
func (h *HelpOverlay) Toggle() {
	h.visible = !h.visible
}

// SetVisible sets the visibility of the help overlay
func (h *HelpOverlay) SetVisible(visible bool) {
	h.visible = visible
}

// IsVisible returns whether the help overlay is visible
func (h HelpOverlay) IsVisible() bool {
	return h.visible
}

// SetSize sets the dimensions of the help overlay
func (ho *HelpOverlay) SetSize(w, h int) {
	ho.width = w
	ho.height = h
}

// View renders the help overlay
func (h HelpOverlay) View() string {
	if !h.visible {
		return ""
	}

	// Define styles for the overlay
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2).
		Background(lipgloss.Color("#1F1F23"))

	titleStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Bold(true).
		MarginTop(1)

	keyStyle := HelpKey
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E4E4E7"))

	// Build help content
	var content strings.Builder

	content.WriteString(titleStyle.Render("Keyboard Shortcuts"))
	content.WriteString("\n")

	// Navigation section
	content.WriteString(sectionStyle.Render("NAVIGATION"))
	content.WriteString("\n")
	content.WriteString(formatHelpLine(keyStyle, descStyle, "arrows/hjkl", "Navigate"))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "g/G", "Jump to top/bottom"))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "Tab", "Switch panel"))

	// Actions section
	content.WriteString(sectionStyle.Render("ACTIONS"))
	content.WriteString("\n")
	content.WriteString(formatHelpLine(keyStyle, descStyle, "Enter", "Zoom into directory"))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "Backspace", "Zoom out"))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "r", "Rescan drive"))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "d", "Toggle diff mode"))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "s", "Cycle sort mode"))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "1-9", "Select drive"))

	// Other section
	content.WriteString(sectionStyle.Render("OTHER"))
	content.WriteString("\n")
	content.WriteString(formatHelpLine(keyStyle, descStyle, "?", "Toggle this help"))
	content.WriteString(formatHelpLineNoNewline(keyStyle, descStyle, "q/Esc", "Quit"))

	box := boxStyle.Render(content.String())

	// Center the box in the terminal
	boxWidth := lipgloss.Width(box)
	boxHeight := lipgloss.Height(box)

	// Calculate padding to center
	horizontalPad := (h.width - boxWidth) / 2
	verticalPad := (h.height - boxHeight) / 2

	if horizontalPad < 0 {
		horizontalPad = 0
	}
	if verticalPad < 0 {
		verticalPad = 0
	}

	// Create centered overlay
	centeredBox := lipgloss.NewStyle().
		MarginLeft(horizontalPad).
		MarginTop(verticalPad).
		Render(box)

	return centeredBox
}

// formatHelpLine formats a single help line with key and description
func formatHelpLine(keyStyle, descStyle lipgloss.Style, key, desc string) string {
	return keyStyle.Width(12).Render(key) + descStyle.Render(desc) + "\n"
}

// formatHelpLineNoNewline formats a help line without trailing newline
func formatHelpLineNoNewline(keyStyle, descStyle lipgloss.Style, key, desc string) string {
	return keyStyle.Width(12).Render(key) + descStyle.Render(desc)
}

// HelpBar renders a bottom help bar with key hints
func HelpBar(width int) string {
	keyStyle := HelpKey
	sepStyle := HelpStyle

	hints := []struct {
		key  string
		desc string
	}{
		{"arrows/hjkl", "navigate"},
		{"Enter", "zoom in"},
		{"Backspace", "zoom out"},
		{"Tab", "switch panel"},
		{"?", "help"},
		{"q", "quit"},
	}

	var parts []string
	for _, hint := range hints {
		parts = append(parts, keyStyle.Render(hint.key)+sepStyle.Render(" "+hint.desc))
	}

	bar := strings.Join(parts, sepStyle.Render("  |  "))

	return HelpStyle.Width(width).Render(bar)
}
