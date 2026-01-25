package ui

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/samuli/diskdive/internal/cache"
	"github.com/samuli/diskdive/internal/model"
	"github.com/samuli/diskdive/internal/scanner"
)

var debugLog *log.Logger

func init() {
	f, err := os.Create("debug.log")
	if err != nil {
		return
	}
	debugLog = log.New(f, "", log.Lmicroseconds)
}

func logTiming(name string, start time.Time) {
	if debugLog != nil {
		debugLog.Printf("%s: %v", name, time.Since(start))
	}
}

// Panel identifies which panel is active
type Panel int

const (
	PanelTree Panel = iota
	PanelTreemap
)

// SortMode defines how nodes are sorted
type SortMode int

const (
	SortBySize SortMode = iota
	SortByChange
	SortByName
)

// String returns a human-readable name for the sort mode
func (s SortMode) String() string {
	switch s {
	case SortBySize:
		return "size"
	case SortByChange:
		return "change"
	case SortByName:
		return "name"
	default:
		return "unknown"
	}
}

// scanStartMsg triggers the actual scan start (after UI has rendered)
type scanStartMsg struct{}

// scanCompleteMsg is sent when a scan finishes
type scanCompleteMsg struct {
	root *model.Node
	err  error
}

// scanProgressMsg is sent during scanning
type scanProgressMsg struct {
	progress scanner.Progress
}

// focusDebounceMsg triggers a debounced treemap focus update
type focusDebounceMsg struct {
	version int
	node    *model.Node
}

// App is the main application model
type App struct {
	// Components
	header        Header
	tree          TreePanel
	treemap       TreemapPanel
	help          HelpOverlay
	driveSelector DriveSelector

	// State
	keys    KeyMap
	cache   *cache.Cache
	scanner scanner.Scanner

	// Data
	drives   []model.Drive
	root     *model.Node
	prevRoot *model.Node

	// UI state
	activePanel  Panel
	sortMode     SortMode
	showDiff     bool
	scanning     bool
	err          error
	focusVersion int // incremented on each selection, used for debouncing

	// Dimensions
	width  int
	height int
}

// NewApp creates a new application instance
func NewApp() App {
	drives, _ := model.GetDrives()

	app := App{
		header:        NewHeader(drives),
		tree:          NewTreePanel(),
		treemap:       NewTreemapPanel(),
		help:          NewHelpOverlay(),
		driveSelector: NewDriveSelector(drives),
		keys:          DefaultKeyMap(),
		cache:         cache.New(cache.DefaultDir()),
		scanner:       scanner.NewWalker(8),
		drives:        drives,
		activePanel:   PanelTree,
		sortMode:      SortBySize,
		scanning:      len(drives) > 0, // Will start scanning on init
	}

	app.tree.SetFocused(true)
	app.treemap.SetFocused(false)

	// Set initial scanning status if we have drives
	if len(drives) > 0 {
		app.header.SetScanning(true, "")
	}

	return app
}

// Init implements tea.Model
func (a App) Init() tea.Cmd {
	// Start scanning first drive if available
	// We send scanStartMsg first to allow the UI to render the scanning state
	if len(a.drives) > 0 {
		return func() tea.Msg {
			return scanStartMsg{}
		}
	}
	return nil
}

// startScan starts scanning a path and returns a command
func (a App) startScan(path string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		root, err := a.scanner.Scan(ctx, path)
		return scanCompleteMsg{root: root, err: err}
	}
}

// Update implements tea.Model
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	start := time.Now()
	defer logTiming("Update", start)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.updateLayout()
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)

	case scanStartMsg:
		// Now actually start the scan
		if len(a.drives) > 0 {
			return a, a.startScan(a.drives[0].Path)
		}
		return a, nil

	case scanCompleteMsg:
		a.scanning = false
		if msg.err != nil {
			a.err = msg.err
			a.header.SetScanning(false, "")
			return a, nil
		}

		// Load previous scan for diff
		if drive := a.header.Selected(); drive != nil {
			prev, _ := a.cache.LoadLatest(drive.Letter)
			a.prevRoot = prev

			// Apply diff
			cache.ApplyDiff(msg.root, prev)

			// Save current scan
			_ = a.cache.Save(drive.Letter, msg.root)
		}

		a.root = msg.root
		a.tree.SetRoot(msg.root)
		a.treemap.SetRoot(msg.root)
		a.tree.SetShowDiff(a.showDiff)
		a.treemap.SetShowDiff(a.showDiff)
		a.header.SetScanning(false, "")
		a.err = nil

		// Recalculate layout now that we have data (tree width depends on content)
		a.updateLayout()

		return a, nil

	case scanProgressMsg:
		progress := fmt.Sprintf("%d files, %s",
			msg.progress.FilesScanned,
			FormatSize(msg.progress.BytesFound))
		a.header.SetScanning(true, progress)
		return a, nil

	case focusDebounceMsg:
		// Only apply focus if this is still the latest version (user stopped scrolling)
		if msg.version == a.focusVersion && msg.node != nil {
			a.treemap.SetFocus(msg.node)
		}
		return a, nil
	}

	return a, nil
}

// handleKey handles keyboard input
func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Help overlay takes precedence
	if a.help.IsVisible() {
		if key.Matches(msg, a.keys.Help) || key.Matches(msg, a.keys.Back) {
			a.help.SetVisible(false)
			return a, nil
		}
		return a, nil
	}

	// Drive selector overlay
	if a.driveSelector.IsVisible() {
		switch {
		case key.Matches(msg, a.keys.Back):
			a.driveSelector.SetVisible(false)
			return a, nil
		case key.Matches(msg, a.keys.Up):
			a.driveSelector.MoveUp()
			return a, nil
		case key.Matches(msg, a.keys.Down):
			a.driveSelector.MoveDown()
			return a, nil
		case key.Matches(msg, a.keys.Enter):
			a.driveSelector.SetVisible(false)
			return a, a.selectDrive(a.driveSelector.Selected())
		}
		return a, nil
	}

	switch {
	case key.Matches(msg, a.keys.Quit):
		return a, tea.Quit

	case key.Matches(msg, a.keys.Help):
		a.help.Toggle()
		return a, nil

	case key.Matches(msg, a.keys.SelectDrive):
		if len(a.drives) > 0 {
			a.driveSelector.SetVisible(true)
		}
		return a, nil

	case key.Matches(msg, a.keys.Tab):
		if a.activePanel == PanelTree {
			a.activePanel = PanelTreemap
			a.tree.SetFocused(false)
			a.treemap.SetFocused(true)
			a.syncSelectionFromTreemap()
		} else {
			a.activePanel = PanelTree
			a.tree.SetFocused(true)
			a.treemap.SetFocused(false)
			return a, a.syncSelection()
		}
		return a, nil

	case key.Matches(msg, a.keys.Up):
		if a.activePanel == PanelTree {
			a.tree.MoveUp()
			return a, a.syncSelection()
		} else {
			a.treemap.MoveToBlock(0, -1)
			a.syncSelectionFromTreemap()
		}
		return a, nil

	case key.Matches(msg, a.keys.Down):
		if a.activePanel == PanelTree {
			a.tree.MoveDown()
			return a, a.syncSelection()
		} else {
			a.treemap.MoveToBlock(0, 1)
			a.syncSelectionFromTreemap()
		}
		return a, nil

	case key.Matches(msg, a.keys.Left):
		if a.activePanel == PanelTree {
			a.tree.Collapse()
			a.updateLayout()
		} else {
			a.treemap.MoveToBlock(-1, 0)
			a.syncSelectionFromTreemap()
		}
		return a, nil

	case key.Matches(msg, a.keys.Right):
		if a.activePanel == PanelTree {
			a.tree.Expand()
			a.updateLayout()
		} else {
			a.treemap.MoveToBlock(1, 0)
			a.syncSelectionFromTreemap()
		}
		return a, nil

	case key.Matches(msg, a.keys.Top):
		if a.activePanel == PanelTree {
			a.tree.GoToTop()
			return a, a.syncSelection()
		}
		return a, nil

	case key.Matches(msg, a.keys.Bottom):
		if a.activePanel == PanelTree {
			a.tree.GoToBottom()
			return a, a.syncSelection()
		}
		return a, nil

	case key.Matches(msg, a.keys.PageUp):
		if a.activePanel == PanelTree {
			a.tree.PageUp()
			return a, a.syncSelection()
		}
		return a, nil

	case key.Matches(msg, a.keys.PageDown):
		if a.activePanel == PanelTree {
			a.tree.PageDown()
			return a, a.syncSelection()
		}
		return a, nil

	case key.Matches(msg, a.keys.Enter):
		if a.activePanel == PanelTreemap {
			a.treemap.ZoomIn()
		} else {
			// Toggle expand/collapse in tree view
			a.tree.Toggle()
			a.updateLayout()
			return a, a.syncSelection()
		}
		return a, nil

	case key.Matches(msg, a.keys.Back):
		if a.activePanel == PanelTreemap {
			a.treemap.ZoomOut()
		} else {
			// Collapse in tree view
			a.tree.Collapse()
			a.updateLayout()
		}
		return a, nil

	case key.Matches(msg, a.keys.ToggleDiff):
		a.showDiff = !a.showDiff
		a.tree.SetShowDiff(a.showDiff)
		a.treemap.SetShowDiff(a.showDiff)
		return a, nil

	case key.Matches(msg, a.keys.CycleSort):
		a.sortMode = (a.sortMode + 1) % 3
		return a, nil

	case key.Matches(msg, a.keys.Rescan):
		if !a.scanning {
			if drive := a.header.Selected(); drive != nil {
				return a, a.selectDrive(a.header.selected)
			}
		}
		return a, nil
	}

	return a, nil
}

// selectDrive selects a drive and starts scanning
func (a *App) selectDrive(idx int) tea.Cmd {
	if idx < 0 || idx >= len(a.drives) {
		return nil
	}

	a.header.SetSelected(idx)
	a.scanning = true
	a.header.SetScanning(true, "")
	a.root = nil
	a.prevRoot = nil
	a.tree.SetRoot(nil)
	a.treemap.SetRoot(nil)

	// Create a new scanner for each scan
	a.scanner = scanner.NewWalker(8)

	return a.startScan(a.drives[idx].Path)
}

// syncSelection syncs the tree selection to the treemap
func (a *App) syncSelection() tea.Cmd {
	node := a.tree.Selected()
	if node == nil {
		return nil
	}
	a.treemap.SetSelected(node)

	// Schedule debounced focus update for directories
	if node.IsDir && len(node.Children) > 0 {
		a.focusVersion++
		version := a.focusVersion
		return tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
			return focusDebounceMsg{version: version, node: node}
		})
	}
	return nil
}

// syncSelectionFromTreemap syncs treemap selection to tree
func (a *App) syncSelectionFromTreemap() {
	// Note: We don't expand tree to match treemap selection
	// as that could be jarring. The treemap shows what's selected.
}

// updateLayout calculates component sizes based on window dimensions
func (a *App) updateLayout() {
	// Header height (1 line + padding)
	headerHeight := 1

	// Help bar height (1 line)
	helpBarHeight := 1

	// Available height for panels
	panelHeight := a.height - headerHeight - helpBarHeight - 2
	if panelHeight < 1 {
		panelHeight = 1
	}

	// Tree panel takes only what it needs, max 50% of screen
	treeWidth := a.tree.RequiredWidth()
	maxTreeWidth := a.width / 2
	if treeWidth > maxTreeWidth {
		treeWidth = maxTreeWidth
	}
	if treeWidth < 20 {
		treeWidth = 20
	}

	// Update component sizes
	a.header.SetWidth(a.width)
	a.tree.SetSize(treeWidth, panelHeight)
	a.treemap.SetSize(a.width-treeWidth, panelHeight)
	a.help.SetSize(a.width, a.height)
	a.driveSelector.SetSize(a.width, a.height)
}

// View implements tea.Model
func (a App) View() string {
	start := time.Now()
	defer logTiming("App.View", start)

	if a.width == 0 || a.height == 0 {
		if a.scanning {
			return "Scanning drive..."
		}
		return "Loading..."
	}

	var sections []string

	// Header
	sections = append(sections, a.header.View())

	// Error display
	if a.err != nil {
		errStyle := lipgloss.NewStyle().
			Foreground(ColorDanger).
			Padding(0, 1)
		sections = append(sections, errStyle.Render(fmt.Sprintf("Error: %v", a.err)))
	}

	// Show scanning panel in center if scanning OR if no data loaded yet
	if a.scanning || a.root == nil {
		// Use same panel height as tree/treemap panels
		panelHeight := a.height - 4 // header + help bar + margins
		if panelHeight < 1 {
			panelHeight = 1
		}

		var statusText string
		if a.scanning {
			progress := a.header.ScanProgress()
			if progress != "" {
				statusText = progress
			} else {
				statusText = "Analyzing..."
			}
		} else {
			statusText = "Loading..."
		}

		// Create the status text box
		scanningBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 3).
			Render(statusText)

		// Center the box within a full-size panel
		centered := lipgloss.Place(
			a.width, panelHeight,
			lipgloss.Center, lipgloss.Center,
			scanningBox,
		)

		sections = append(sections, centered)
	} else {
		// Panels side by side
		treeView := a.tree.View()
		treemapView := a.treemap.View()
		panels := lipgloss.JoinHorizontal(lipgloss.Top, treeView, treemapView)
		sections = append(sections, panels)
	}

	// Help bar at bottom
	sections = append(sections, HelpBar(a.width))

	// Join all sections vertically
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Help overlay on top if visible (highest priority)
	if a.help.IsVisible() {
		overlay := a.help.View()
		return lipgloss.Place(
			a.width, a.height,
			lipgloss.Center, lipgloss.Center,
			overlay,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("#1F1F23")),
		)
	}

	// Drive selector overlay
	if a.driveSelector.IsVisible() {
		overlay := a.driveSelector.View()
		return lipgloss.Place(
			a.width, a.height,
			lipgloss.Center, lipgloss.Center,
			overlay,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("#1F1F23")),
		)
	}

	return content
}
