package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Colors - cyberpunk/neon palette
var (
	ColorPrimary    = lipgloss.Color("#C084FC") // soft violet
	ColorSecondary  = lipgloss.Color("#BD93F9") // soft purple
	ColorSuccess    = lipgloss.Color("#39FF14") // neon green
	ColorWarning    = lipgloss.Color("#FFB86C") // orange
	ColorDanger     = lipgloss.Color("#FF5555") // red
	ColorMuted      = lipgloss.Color("#4A5568") // darker muted
	ColorBorder     = lipgloss.Color("#4A5568") // border
	ColorBackground = lipgloss.Color("#1F1F23") // dark background
	ColorCyan       = lipgloss.Color("#00FFFF") // neon cyan
	ColorSize       = lipgloss.Color("#39FF14") // neon green for sizes
	ColorDir        = lipgloss.Color("#00FFFF") // cyan for directories
	ColorFile       = lipgloss.Color("#A0A0A0") // dimmer for files
	ColorText       = lipgloss.Color("#E4E4E7") // default text

	// Change colors (warm/cool palette - colorblind friendly)
	ColorGrew       = lipgloss.Color("#FFB86C") // orange - growth
	ColorGrewBg     = lipgloss.Color("#7C2D12") // dark orange bg
	ColorShrunk     = lipgloss.Color("#5EEAD4") // teal - shrinkage
	ColorShrunkBg   = lipgloss.Color("#115E59") // dark teal bg
	ColorNew        = lipgloss.Color("#F1FA8C") // yellow
	ColorUnchanged  = lipgloss.Color("#6272A4") // muted purple-gray
	ColorMixed      = lipgloss.Color("#FF79C6") // pink - folder has both grew and shrunk
	ColorMixedBg    = lipgloss.Color("#4C1D95") // dark purple bg
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
			Foreground(lipgloss.Color("#E4E4E7"))

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
		Foreground(ColorCyan). // bright cyan keys
		Bold(true)

	// Change indicators
	GrewStyle = lipgloss.NewStyle().
			Foreground(ColorGrew)

	ShrunkStyle = lipgloss.NewStyle().
			Foreground(ColorShrunk)

	NewBadge = lipgloss.NewStyle().
			Background(ColorNew).
			Foreground(lipgloss.Color("#000000")).
			Padding(0, 1).
			Bold(true)

	DeletedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")) // muted gray

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
