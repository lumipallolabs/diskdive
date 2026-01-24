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

	// Stats (only show when not scanning - scanning status shown in center panel)
	var stats string
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
		}
	}

	// Layout: tabs on left, stats on right
	tabsWidth := lipgloss.Width(driveTabs)
	statsWidth := lipgloss.Width(stats)
	gap := h.width - tabsWidth - statsWidth - 2 // -2 for HeaderStyle padding
	if gap < 1 {
		gap = 1
	}

	line := driveTabs + strings.Repeat(" ", gap) + stats

	return HeaderStyle.Render(line)
}
