package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Colors
var (
	ColorPrimary   = lipgloss.Color("#7D56F4")
	ColorSecondary = lipgloss.Color("#5A4FCF")
	ColorSuccess   = lipgloss.Color("#73F59F")
	ColorWarning   = lipgloss.Color("#F5A623")
	ColorDanger    = lipgloss.Color("#F56565")
	ColorMuted     = lipgloss.Color("#6B7280")
	ColorBorder    = lipgloss.Color("#3F3F46")

	// Change colors (warm/cool palette - colorblind friendly)
	ColorGrew       = lipgloss.Color("#FDBA74") // light orange - growth
	ColorGrewBg     = lipgloss.Color("#7C2D12") // dark orange bg
	ColorShrunk     = lipgloss.Color("#5EEAD4") // teal - shrinkage (distinct from directory blue)
	ColorShrunkBg   = lipgloss.Color("#115E59") // dark teal bg
	ColorNew        = lipgloss.Color("#FDE047") // yellow
	ColorUnchanged  = lipgloss.Color("#9CA3AF") // gray
	ColorMixed      = lipgloss.Color("#E9D5FF") // light purple - folder has both grew and shrunk
	ColorMixedBg    = lipgloss.Color("#4C1D95") // dark purple bg
)

// Styles
var (
	// Header
	HeaderStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1F1F23")).
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

	// Help bar
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(0, 1)

	HelpKey = lipgloss.NewStyle().
		Foreground(ColorPrimary).
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
)

// FormatSize formats bytes to human readable string
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1fTB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
