package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Colors - cyberpunk/neon palette
var (
	ColorPrimary    = lipgloss.Color("#C084FC") // soft violet
	ColorSuccess    = lipgloss.Color("#39FF14") // neon green
	ColorDanger     = lipgloss.Color("#FF5555") // red
	ColorMuted      = lipgloss.Color("#4A5568") // darker muted
	ColorBorder     = lipgloss.Color("#4A5568") // border
	ColorBackground = lipgloss.Color("#1F1F23") // dark background
	ColorCyan       = lipgloss.Color("#00FFFF") // neon cyan
	ColorDir        = lipgloss.Color("#00FFFF") // cyan for directories
	ColorFile       = lipgloss.Color("#A0A0A0") // dimmer for files
	ColorText       = lipgloss.Color("#E4E4E7") // default text

	// Deletion indicator
	ColorShrunk = lipgloss.Color("#5EEAD4") // teal - freed space
)

// Styles
var (
	// Header
	HeaderStyle = lipgloss.NewStyle().
			Background(ColorBackground).
			Padding(0, 1)

	DriveTabActive = lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	DriveTabInactive = lipgloss.NewStyle().
				Background(lipgloss.Color("#3F3F46")).
				Foreground(lipgloss.Color("#A1A1AA")).
				Padding(0, 1)

	StatsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	// Tree
	TreePanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	TreeItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E4E4E7"))

	TreeItemSelected = lipgloss.NewStyle().
				Background(ColorPrimary).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true)

	TreeItemSelectedUnfocused = lipgloss.NewStyle().
					Background(lipgloss.Color("#4A5568")).
					Foreground(lipgloss.Color("#FFFFFF"))

	TreeSizeBar = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	// Treemap
	TreemapPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder).
				Padding(0, 1)

	TreemapBlock = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder())

	TreemapBlockSelected = lipgloss.NewStyle().
				Border(lipgloss.ThickBorder()).
				BorderForeground(ColorPrimary)

	// Help bar - dimmer with bright key highlights
	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3D4555")). // very dim
			Padding(0, 1)

	HelpKey = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Background(lipgloss.Color("#1E3A4C")). // subtle dark cyan bg
			Padding(0, 1)

	// Inline key hint (for use in text)
	KeyHint = lipgloss.NewStyle().
		Foreground(ColorCyan).
		Background(lipgloss.Color("#1E3A4C")).
		Padding(0, 1)

	// Help overlay key style (no background for cleaner look)
	HelpOverlayKey = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Padding(0, 1)

	// Deletion indicators
	ShrunkStyle = lipgloss.NewStyle().
			Foreground(ColorShrunk)

	DeletedBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#374151")). // dark gray
			Foreground(lipgloss.Color("#9CA3AF")). // light gray
			Padding(0, 1)
)

// FormatSize formats bytes to human readable string
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	// Handle negative values
	negative := bytes < 0
	if negative {
		bytes = -bytes
	}

	var result string
	switch {
	case bytes >= TB:
		result = fmt.Sprintf("%.1fTB", float64(bytes)/TB)
	case bytes >= GB:
		result = fmt.Sprintf("%.1fGB", float64(bytes)/GB)
	case bytes >= MB:
		result = fmt.Sprintf("%.1fMB", float64(bytes)/MB)
	case bytes >= KB:
		result = fmt.Sprintf("%.1fKB", float64(bytes)/KB)
	default:
		result = fmt.Sprintf("%dB", bytes)
	}

	if negative {
		return "-" + result
	}
	return result
}

// FormatTime formats a time for display, using shorter format for current year
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	if t.Year() == time.Now().Year() {
		return t.Format("Jan 2 15:04")
	}
	return t.Format("Jan 2, 2006 15:04")
}
