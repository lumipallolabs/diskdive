package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const helpKeyColumnWidth = 14 // Width for key column in help text (includes padding)

// HelpOverlay displays keyboard shortcuts in a centered overlay
type HelpOverlay struct {
	visible bool
	width   int
	height  int
	version string
}

// NewHelpOverlay creates a new help overlay component
func NewHelpOverlay(version string) HelpOverlay {
	return HelpOverlay{
		visible: false,
		version: version,
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

	// Define styles
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 3)

	sectionStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		MarginTop(1)

	keyStyle := HelpOverlayKey
	descStyle := lipgloss.NewStyle().Foreground(ColorText)
	dimStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	// Build help content
	var content strings.Builder

	// App name and version header
	nameStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)
	versionStyle := lipgloss.NewStyle().
		Foreground(ColorMuted)

	content.WriteString(nameStyle.Render("DiskDive"))
	if h.version != "" {
		content.WriteString(versionStyle.Render(" " + h.version))
	}
	content.WriteString("\n")

	// Navigation section
	content.WriteString(sectionStyle.Render("Navigation"))
	content.WriteString("\n")
	content.WriteString(formatHelpLine(keyStyle, descStyle, "↑↓←→ hjkl", "Navigate", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "Enter", "Open directory", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "Esc / ⌫", "Go back", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "PgUp/PgDn", "Scroll faster", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "g / G", "Top / Bottom", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "Tab", "Switch panel", true))

	// Actions section
	content.WriteString(sectionStyle.Render("Actions"))
	content.WriteString("\n")
	content.WriteString(formatHelpLine(keyStyle, descStyle, "Space", "Preview file", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "e", "Change drive", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "o", "Open in Finder", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "r", "Rescan", true))
	content.WriteString(formatHelpLine(keyStyle, descStyle, "q", "Quit", true))

	// Footer
	content.WriteString("\n")
	content.WriteString(dimStyle.Render("Press any key to close"))

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



// HelpBar renders a bottom help bar with key hints
func HelpBar(width int) string {
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")) // lighter dim description

	// Full hints for wide terminals, abbreviated for narrow
	type hint struct {
		key  string
		desc string
	}

	fullHints := []hint{
		{"↑↓←→", "navigate"},
		{"Enter", "zoom in"},
		{"Esc", "back"},
		{"Tab", "panel"},
		{"Space", "preview"},
		{"e", "drives"},
		{"o", "open"},
		{"?", "help"},
		{"q", "quit"},
	}

	// Compact hints for narrow terminals (arrows more universal than vim keys)
	compactHints := []hint{
		{"↑↓←→", "nav"},
		{"Enter", "in"},
		{"Esc", "out"},
		{"?", "help"},
		{"q", "quit"},
	}

	// Minimal hints for very narrow terminals
	minimalHints := []hint{
		{"?", "help"},
		{"q", "quit"},
	}

	// Choose hint set based on width
	var hints []hint
	if width >= 100 {
		hints = fullHints
	} else if width >= 60 {
		hints = compactHints
	} else {
		hints = minimalHints
	}

	var parts []string
	for _, h := range hints {
		parts = append(parts, HelpKey.Render(h.key)+" "+descStyle.Render(h.desc))
	}

	separator := "   "
	if width < 80 {
		separator = "  "
	}

	bar := strings.Join(parts, separator)

	return HelpStyle.Width(width).MaxHeight(1).Render(bar)
}
