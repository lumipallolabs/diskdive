package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lumipallolabs/diskdive/internal/model"
)

// DriveSelector displays a list of available drives for selection
type DriveSelector struct {
	drives   []model.Drive
	selected int
	visible  bool
	width    int
	height   int
}

// NewDriveSelector creates a new drive selector component
func NewDriveSelector(drives []model.Drive) DriveSelector {
	return DriveSelector{
		drives:   drives,
		selected: 0,
		visible:  false,
	}
}

// SetDrives updates the available drives
func (d *DriveSelector) SetDrives(drives []model.Drive) {
	d.drives = drives
	if d.selected >= len(drives) {
		d.selected = 0
	}
}

// SetSelected sets the currently highlighted drive
func (d *DriveSelector) SetSelected(idx int) {
	if idx >= 0 && idx < len(d.drives) {
		d.selected = idx
	}
}

// Selected returns the index of the currently highlighted drive
func (d DriveSelector) Selected() int {
	return d.selected
}

// SelectedDrive returns the currently highlighted drive
func (d DriveSelector) SelectedDrive() *model.Drive {
	if d.selected >= 0 && d.selected < len(d.drives) {
		return &d.drives[d.selected]
	}
	return nil
}

// Toggle toggles visibility of the selector
func (d *DriveSelector) Toggle() {
	d.visible = !d.visible
}

// SetVisible sets visibility of the selector
func (d *DriveSelector) SetVisible(visible bool) {
	d.visible = visible
}

// IsVisible returns whether the selector is visible
func (d DriveSelector) IsVisible() bool {
	return d.visible
}

// SetSize sets the dimensions for centering
func (d *DriveSelector) SetSize(w, h int) {
	d.width = w
	d.height = h
}

// MoveUp moves selection up
func (d *DriveSelector) MoveUp() {
	if d.selected > 0 {
		d.selected--
	}
}

// MoveDown moves selection down
func (d *DriveSelector) MoveDown() {
	if d.selected < len(d.drives)-1 {
		d.selected++
	}
}

// View renders the drive selector overlay
func (d DriveSelector) View() string {
	if !d.visible || len(d.drives) == 0 {
		return ""
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2).
		Background(lipgloss.Color("#1F1F23"))

	titleStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		MarginBottom(1)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E4E4E7")).
		PaddingLeft(1).
		PaddingRight(1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(ColorPrimary).
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	hintStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		MarginTop(1)

	var content strings.Builder

	content.WriteString(titleStyle.Render("Select Drive"))
	content.WriteString("\n")

	for i, drive := range d.drives {
		usedPct := drive.UsedPercent()
		freeSpace := FormatSize(drive.FreeBytes)
		totalSpace := FormatSize(drive.TotalBytes)

		line := fmt.Sprintf("%s: %s free / %s (%.0f%% used)",
			drive.Letter, freeSpace, totalSpace, usedPct)

		if i == d.selected {
			content.WriteString(selectedStyle.Render(line))
		} else {
			content.WriteString(normalStyle.Render(line))
		}
		content.WriteString("\n")
	}

	content.WriteString(hintStyle.Render("↑/↓ select  Enter confirm  Esc cancel"))

	box := boxStyle.Render(strings.TrimSuffix(content.String(), "\n"))

	return lipgloss.Place(d.width, d.height, lipgloss.Center, lipgloss.Center, box)
}
