package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const helpKeyColumnWidth = 12 // Width for key column in help text

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
		Padding(1, 2)

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
	content.WriteString(formatHelpLine(keyStyle, descStyle, "arrows/hjkl", "Navigate", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "PgUp/PgDn", "Scroll faster", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "g/G", "Jump to top/bottom", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "Tab", "Switch panel", true))

	// Actions section
	content.WriteString(sectionStyle.Render("ACTIONS"))
	content.WriteString("\n")
	content.WriteString(formatHelpLine(keyStyle, descStyle, "Enter", "Zoom into directory", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "Esc/⌫", "Go back / Close overlay", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "Space", "Select drive", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "o", "Open in Explorer", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "r", "Rescan drive", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "d", "Toggle diff mode", true))

	// Other section
	content.WriteString(sectionStyle.Render("OTHER"))
	content.WriteString("\n")
	content.WriteString(formatHelpLine(keyStyle, descStyle, "?", "Toggle this help", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "q", "Quit", true))

	// Diff colors section
	content.WriteString(sectionStyle.Render("DIFF COLORS"))
	content.WriteString("\n")
	content.WriteString(formatColorLine(ColorNew, "New item", true))
	content.WriteString(formatColorLine(ColorGrew, "Size increased", true))
	content.WriteString(formatColorLine(ColorShrunk, "Size decreased", true))
	content.WriteString(formatColorLine(ColorMixed, "Mixed changes", false))

	box := boxStyle.Render(content.String())

	// Center the box using lipgloss.Place
	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, box)
}

// formatHelpLine formats a single help line with key and description
func formatHelpLine(keyStyle, descStyle lipgloss.Style, key, desc string, newline bool) string {
	line := keyStyle.Width(helpKeyColumnWidth).Render(key) + descStyle.Render(desc)
	if newline {
		return line + "\n"
	}
	return line
}

// formatColorLine formats a color indicator line
func formatColorLine(color lipgloss.Color, desc string, newline bool) string {
	colorStyle := lipgloss.NewStyle().Foreground(color)
	descStyle := lipgloss.NewStyle().Foreground(ColorText)
	line := colorStyle.Width(helpKeyColumnWidth).Render("████") + descStyle.Render(desc)
	if newline {
		return line + "\n"
	}
	return line
}

// HelpBar renders a bottom help bar with key hints
func HelpBar(width int) string {
	keyStyle := HelpKey
	sepStyle := HelpStyle

	hints := []struct {
		key  string
		desc string
	}{
		{"↑↓←→/hjkl", "navigate"},
		{"Enter", "zoom in"},
		{"Esc", "back"},
		{"Tab", "panel"},
		{"Space", "drives"},
		{"o", "open"},
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
