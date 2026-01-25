package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/samuli/diskdive/internal/model"
)

const headerProgressBarWidth = 20 // Width of disk usage progress bar

// Header displays drive tabs and stats
type Header struct {
	drives       []model.Drive
	selected     int
	width        int
	scanning     bool
	scanProgress string
	freedSession int64
	freedTotal   int64
}

// NewHeader creates a new header component
func NewHeader(drives []model.Drive) Header {
	return Header{
		drives:   drives,
		selected: 0,
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

// View renders the header
func (h Header) View() string {
	// App name with style
	appName := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C084FC")). // soft violet
		Bold(true).
		Render("DISKDIVE")

	// Drive tabs
	var tabs []string
	for i, d := range h.drives {
		label := fmt.Sprintf("%s:", d.Letter)
		if i == h.selected {
			tabs = append(tabs, DriveTabActive.Render(label))
		} else {
			tabs = append(tabs, DriveTabInactive.Render(label))
		}
	}
	driveTabs := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	// Freed stats (show in middle when either counter > 0)
	var freedStats string
	if h.freedSession > 0 || h.freedTotal > 0 {
		freedLabel := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("Freed: ")
		freedSessionStr := lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Render(FormatSize(h.freedSession) + " session")
		freedSep := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" | ")
		freedTotalStr := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(FormatSize(h.freedTotal) + " total")
		freedStats = freedLabel + freedSessionStr + freedSep + freedTotalStr
	}

	// Stats (only show when not scanning - scanning status shown in center panel)
	var stats, statsCompact string
	if !h.scanning {
		if drive := h.Selected(); drive != nil {
			usedPct := drive.UsedPercent()
			barWidth := headerProgressBarWidth
			filled := int(usedPct / 100 * float64(barWidth))
			bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

			stats = StatsStyle.Render(fmt.Sprintf(
				"Used: %s / %s  [%s] %.0f%%",
				FormatSize(drive.UsedBytes()),
				FormatSize(drive.TotalBytes),
				bar,
				usedPct,
			))
			statsCompact = StatsStyle.Render(fmt.Sprintf(
				"Used: %s / %s",
				FormatSize(drive.UsedBytes()),
				FormatSize(drive.TotalBytes),
			))
		}
	}

	// Layout: app name, tabs on left, freed in middle, disk usage on right
	appNameWidth := lipgloss.Width(appName)
	tabsWidth := lipgloss.Width(driveTabs)
	freedWidth := lipgloss.Width(freedStats)
	statsWidth := lipgloss.Width(stats)

	sep := lipgloss.NewStyle().Foreground(ColorBorder).Render(" │ ")
	sepWidth := lipgloss.Width(sep)

	// Calculate total content width
	totalContent := appNameWidth + sepWidth + tabsWidth + freedWidth + statsWidth + 4 // +4 for min gaps

	// For narrow terminals, progressively hide elements
	if h.width < totalContent {
		// First: switch to compact stats (no progress bar)
		if statsWidth > 0 && statsCompact != "" {
			stats = statsCompact
			statsWidth = lipgloss.Width(stats)
			totalContent = appNameWidth + sepWidth + tabsWidth + freedWidth + statsWidth + 4
		}
	}
	if h.width < totalContent {
		// Then drop freed stats
		if freedWidth > 0 {
			freedStats = ""
			freedWidth = 0
			totalContent = appNameWidth + sepWidth + tabsWidth + statsWidth + 2
		}
	}
	if h.width < totalContent {
		// Finally drop stats entirely
		if statsWidth > 0 {
			stats = ""
			statsWidth = 0
			totalContent = appNameWidth + sepWidth + tabsWidth
		}
	}

	// Calculate gaps to distribute remaining space
	remainingSpace := h.width - totalContent
	if remainingSpace < 2 {
		remainingSpace = 2
	}

	// Split remaining space: more on left side of freed stats to push it toward center
	leftGap := remainingSpace / 2
	rightGap := remainingSpace - leftGap
	if leftGap < 1 {
		leftGap = 1
	}
	if rightGap < 1 {
		rightGap = 1
	}

	line := appName + sep + driveTabs + strings.Repeat(" ", leftGap) + freedStats + strings.Repeat(" ", rightGap) + stats

	return HeaderStyle.MaxHeight(1).Render(line)
}
