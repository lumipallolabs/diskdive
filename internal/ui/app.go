package ui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/samuli/diskdive/internal/cache"
	"github.com/samuli/diskdive/internal/model"
	"github.com/samuli/diskdive/internal/scanner"
)

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

// scanCompleteMsg is sent when a scan finishes
type scanCompleteMsg struct {
	root *model.Node
	err  error
}

// scanProgressMsg is sent during scanning
type scanProgressMsg struct {
	progress scanner.Progress
}

// App is the main application model
type App struct {
	// Components
	header  Header
	tree    TreePanel
	treemap TreemapPanel
	help    HelpOverlay

	// State
	keys    KeyMap
	cache   *cache.Cache
	scanner scanner.Scanner

	// Data
	drives   []model.Drive
	root     *model.Node
	prevRoot *model.Node

	// UI state
	activePanel Panel
	sortMode    SortMode
	showDiff    bool
	scanning    bool
	err         error

	// Dimensions
	width  int
	height int
}

// NewApp creates a new application instance
func NewApp() App {
	drives, _ := model.GetDrives()

	app := App{
		header:      NewHeader(drives),
		tree:        NewTreePanel(),
		treemap:     NewTreemapPanel(),
		help:        NewHelpOverlay(),
		keys:        DefaultKeyMap(),
		cache:       cache.New(cache.DefaultDir()),
		scanner:     scanner.NewWalker(8),
		drives:      drives,
		activePanel: PanelTree,
		sortMode:    SortBySize,
	}

	app.tree.SetFocused(true)
	app.treemap.SetFocused(false)

	return app
}

// Init implements tea.Model
func (a App) Init() tea.Cmd {
	// Start scanning first drive if available
	if len(a.drives) > 0 {
		return a.startScan(a.drives[0].Path)
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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.updateLayout()
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)

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

		return a, nil

	case scanProgressMsg:
		progress := fmt.Sprintf("%d files, %s",
			msg.progress.FilesScanned,
			FormatSize(msg.progress.BytesFound))
		a.header.SetScanning(true, progress)
		return a, nil
	}

	return a, nil
}

// handleKey handles keyboard input
func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Help overlay takes precedence
	if a.help.IsVisible() {
		if key.Matches(msg, a.keys.Help) || key.Matches(msg, a.keys.Quit) {
			a.help.SetVisible(false)
			return a, nil
		}
		return a, nil
	}

	switch {
	case key.Matches(msg, a.keys.Quit):
		return a, tea.Quit

	case key.Matches(msg, a.keys.Help):
		a.help.Toggle()
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
			a.syncSelection()
		}
		return a, nil

	case key.Matches(msg, a.keys.Up):
		if a.activePanel == PanelTree {
			a.tree.MoveUp()
			a.syncSelection()
		} else {
			a.treemap.MoveToBlock(0, -1)
			a.syncSelectionFromTreemap()
		}
		return a, nil

	case key.Matches(msg, a.keys.Down):
		if a.activePanel == PanelTree {
			a.tree.MoveDown()
			a.syncSelection()
		} else {
			a.treemap.MoveToBlock(0, 1)
			a.syncSelectionFromTreemap()
		}
		return a, nil

	case key.Matches(msg, a.keys.Left):
		if a.activePanel == PanelTree {
			a.tree.Collapse()
		} else {
			a.treemap.MoveToBlock(-1, 0)
			a.syncSelectionFromTreemap()
		}
		return a, nil

	case key.Matches(msg, a.keys.Right):
		if a.activePanel == PanelTree {
			a.tree.Expand()
		} else {
			a.treemap.MoveToBlock(1, 0)
			a.syncSelectionFromTreemap()
		}
		return a, nil

	case key.Matches(msg, a.keys.Top):
		if a.activePanel == PanelTree {
			a.tree.GoToTop()
			a.syncSelection()
		}
		return a, nil

	case key.Matches(msg, a.keys.Bottom):
		if a.activePanel == PanelTree {
			a.tree.GoToBottom()
			a.syncSelection()
		}
		return a, nil

	case key.Matches(msg, a.keys.Enter):
		if a.activePanel == PanelTreemap {
			a.treemap.ZoomIn()
		} else {
			// Expand in tree view
			a.tree.Expand()
		}
		return a, nil

	case key.Matches(msg, a.keys.Back):
		if a.activePanel == PanelTreemap {
			a.treemap.ZoomOut()
		} else {
			// Collapse in tree view
			a.tree.Collapse()
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

	// Drive selection
	case key.Matches(msg, a.keys.Drive1):
		return a, a.selectDrive(0)
	case key.Matches(msg, a.keys.Drive2):
		return a, a.selectDrive(1)
	case key.Matches(msg, a.keys.Drive3):
		return a, a.selectDrive(2)
	case key.Matches(msg, a.keys.Drive4):
		return a, a.selectDrive(3)
	case key.Matches(msg, a.keys.Drive5):
		return a, a.selectDrive(4)
	case key.Matches(msg, a.keys.Drive6):
		return a, a.selectDrive(5)
	case key.Matches(msg, a.keys.Drive7):
		return a, a.selectDrive(6)
	case key.Matches(msg, a.keys.Drive8):
		return a, a.selectDrive(7)
	case key.Matches(msg, a.keys.Drive9):
		return a, a.selectDrive(8)
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
	a.header.SetScanning(true, "Starting...")
	a.root = nil
	a.prevRoot = nil
	a.tree.SetRoot(nil)
	a.treemap.SetRoot(nil)

	// Create a new scanner for each scan
	a.scanner = scanner.NewWalker(8)

	return a.startScan(a.drives[idx].Path)
}

// syncSelection syncs the tree selection to the treemap
func (a *App) syncSelection() {
	if node := a.tree.Selected(); node != nil {
		a.treemap.SetSelected(node)
	}
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

	// Split width between tree and treemap (50/50)
	halfWidth := a.width / 2
	if halfWidth < 10 {
		halfWidth = 10
	}

	// Update component sizes
	a.header.SetWidth(a.width)
	a.tree.SetSize(halfWidth, panelHeight)
	a.treemap.SetSize(a.width-halfWidth, panelHeight)
	a.help.SetSize(a.width, a.height)
}

// View implements tea.Model
func (a App) View() string {
	if a.width == 0 || a.height == 0 {
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

	// Panels side by side
	treeView := a.tree.View()
	treemapView := a.treemap.View()
	panels := lipgloss.JoinHorizontal(lipgloss.Top, treeView, treemapView)
	sections = append(sections, panels)

	// Help bar at bottom
	sections = append(sections, HelpBar(a.width))

	// Join all sections vertically
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Help overlay on top if visible
	if a.help.IsVisible() {
		overlay := a.help.View()
		// Position overlay over the content
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
