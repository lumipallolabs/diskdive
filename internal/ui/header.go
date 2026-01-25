package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/samuli/diskdive/internal/model"
)

const headerProgressBarWidth = 20 // Width of disk usage progress bar

// Header displays drive info and stats (2 lines)
type Header struct {
	drives       []model.Drive
	selected     int
	width        int
	scanning     bool
	scanProgress string
	freedSession int64
	freedTotal   int64
	version      string
}

// NewHeader creates a new header component
func NewHeader(drives []model.Drive, version string) Header {
	return Header{
		drives:   drives,
		selected: 0,
		version:  version,
	}
}

// SetDrives updates the available drives
func (h *Header) SetDrives(drives []model.Drive) {
	h.drives = drives
}

// SetSelected sets the selected drive index
func (h *Header) SetSelected(idx int) {
	if idx >= 0 && idx < len(h.drives) {
		h.selected = idx
	}
}

// Selected returns the currently selected drive
func (h Header) Selected() *model.Drive {
	if h.selected < 0 || h.selected >= len(h.drives) {
		return nil
	}
	return &h.drives[h.selected]
}

// SetScanning sets the scanning state
func (h *Header) SetScanning(scanning bool, progress string) {
	h.scanning = scanning
	h.scanProgress = progress
}

// SetFreedStats sets the freed space statistics
func (h *Header) SetFreedStats(session, total int64) {
	h.freedSession = session
	h.freedTotal = total
}

// ScanProgress returns the current scan progress text
func (h Header) ScanProgress() string {
	return h.scanProgress
}

// SetWidth sets the header width
func (h *Header) SetWidth(w int) {
	h.width = w
}

// Update handles messages
func (h Header) Update(msg tea.Msg) (Header, tea.Cmd) {
	return h, nil
}

// View renders the header (2 lines + separator)
// Line 1: DISKDIVE 0.1.4                     Used: X / Y [bar] XX%
// Line 2: Drive: Name [space]               Freed: X session | Y total
// Line 3: ─────────────────────────────────────────────────────────
func (h Header) View() string {
	// Styles
	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C084FC")). // soft violet
		Bold(true)
	versionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")) // lighter dim gray
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")) // lighter dim gray
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")) // lighter dim for labels
	barFilledStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C084FC")) // violet for filled
	barEmptyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")) // lighter gray for empty

	// === LINE 1: App name (left) | Used stats (right) ===
	appName := nameStyle.Render("DISKDIVE") + versionStyle.Render(" "+h.version)

	var usedStats string
	if drive := h.Selected(); drive != nil {
		usedPct := drive.UsedPercent()
		barWidth := headerProgressBarWidth

		// Check if we have room for full bar
		usedLabel := labelStyle.Render("Used: ")
		usedValue := StatsStyle.Render(fmt.Sprintf("%s / %s", FormatSize(drive.UsedBytes()), FormatSize(drive.TotalBytes)))
		appWidth := lipgloss.Width(appName)
		fullStatsWidth := 6 + 20 + 4 + barWidth + 5 // "Used: " + sizes + "  " + bar + " XX%"

		if h.width < appWidth+fullStatsWidth+4 {
			// Narrow: no bar
			usedStats = usedLabel + usedValue + StatsStyle.Render(fmt.Sprintf("  %.0f%%", usedPct))
		} else {
			// Full with thicker bar
			filled := int(usedPct / 100 * float64(barWidth))
			if filled > barWidth {
				filled = barWidth
			}
			bar := barFilledStyle.Render(strings.Repeat("▓", filled)) + barEmptyStyle.Render(strings.Repeat("░", barWidth-filled))
			usedStats = usedLabel + usedValue + StatsStyle.Render("  ") + bar + StatsStyle.Render(fmt.Sprintf(" %.0f%%", usedPct))
		}
	}

	// Build line 1
	line1Left := appName
	line1Right := usedStats
	gap1 := h.width - lipgloss.Width(line1Left) - lipgloss.Width(line1Right)
	if gap1 < 2 {
		gap1 = 2
	}
	line1 := line1Left + strings.Repeat(" ", gap1) + line1Right

	// === LINE 2: Drive info (left) | Freed stats (right) ===
	var freedStats string
	if h.freedSession > 0 || h.freedTotal > 0 {
		freedLabel := labelStyle.Render("Freed: ")
		freedSession := lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Render(FormatSize(h.freedSession) + " session")
		freedSep := dimStyle.Render(" | ")
		freedTotal := dimStyle.Render(FormatSize(h.freedTotal) + " total")
		freedStats = freedLabel + freedSession + freedSep + freedTotal
	}

	var driveName string
	if drive := h.Selected(); drive != nil {
		driveLabel := labelStyle.Render("Drive: ")
		driveNameStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)
		driveValue := driveNameStyle.Render(drive.Letter)
		driveName = driveLabel + driveValue

		// Add "space change" hint only if there's room
		hint := dimStyle.Render("  ") + KeyHint.Render("space") + dimStyle.Render(" change")
		hintWidth := lipgloss.Width(hint)
		availableForHint := h.width - lipgloss.Width(driveName) - lipgloss.Width(freedStats) - 4 // 4 = min gap
		if availableForHint >= hintWidth {
			driveName = driveName + hint
		}
	}

	// Build line 2
	line2Left := driveName
	line2Right := freedStats
	gap2 := h.width - lipgloss.Width(line2Left) - lipgloss.Width(line2Right)
	if gap2 < 2 {
		gap2 = 2
	}
	line2 := line2Left + strings.Repeat(" ", gap2) + line2Right

	// === LINE 3: Separator ===
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3F3F46"))
	separator := sepStyle.Render(strings.Repeat("─", h.width))

	return lipgloss.JoinVertical(lipgloss.Left, line1, line2, separator)
}
