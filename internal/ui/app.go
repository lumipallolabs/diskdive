package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/samuli/diskdive/internal/logging"
	"github.com/samuli/diskdive/internal/model"
	"github.com/samuli/diskdive/internal/scanner"
	"github.com/samuli/diskdive/internal/stats"
	"github.com/samuli/diskdive/internal/watcher"
)

// Panel identifies which panel is active
type Panel int

const (
	PanelTree Panel = iota
	PanelTreemap
)


// scanStartMsg triggers the actual scan start (after UI has rendered)
type scanStartMsg struct{}

// scanCompleteMsg is sent when filesystem scan finishes
type scanCompleteMsg struct {
	root *model.Node
	err  error
}

// computeSizesMsg triggers size computation phase
type computeSizesMsg struct {
	root *model.Node
}

// computeSizesDoneMsg is sent when size computation completes
type computeSizesDoneMsg struct {
	root *model.Node
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

// spinnerTickMsg triggers spinner animation
type spinnerTickMsg struct{}

// scanCompleteDelayMsg is sent after showing "complete" for a moment
type scanCompleteDelayMsg struct {
	root *model.Node
}

// watcherEventMsg is sent when the filesystem watcher detects a change
type watcherEventMsg struct {
	event watcher.Event
}

// startWatcherMsg triggers starting the filesystem watcher
type startWatcherMsg struct {
	root string
}

// Spinner frames - modern braille dots spinner
var spinnerFrames = []string{
	"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
}

// Timing constants
const (
	spinnerTickInterval  = 80 * time.Millisecond
	borderRotationSpeed  = 33  // milliseconds per frame (faster spin)
	focusDebounceTimeout = 300 * time.Millisecond
)

// Phase represents a scan phase with its display name
type Phase struct {
	id   int
	name string
}

var phases = []Phase{
	{0, "Scanning files"},
	{1, "Computing sizes"},
	{2, "Complete"},
	{3, ""}, // phaseDone has no display name
}

// Phase IDs for easy reference
const (
	phaseScanning = iota
	phaseComputingSizes
	phaseComplete
	phaseDone
)

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
	scanner scanner.Scanner

	// Filesystem watcher and stats
	watcher          *watcher.Watcher
	statsManager     *stats.Manager
	freedThisSession int64
	freedLifetime    int64

	// Data
	drives     []model.Drive
	root       *model.Node
	customPath string // optional path from command line

	// UI state
	activePanel Panel
	showDiff    bool
	scanning       bool
	scanPhase      int       // current phase index (0-4)
	scanStartTime  time.Time // when scan started
	scanFileCount   string // "1,234 files" from scanning
	scanBytesFound  string // "50 GB" from scanning
	scanBytesRaw    int64  // raw bytes for progress calculation
	err            error
	spinnerFrame   int
	focusVersion   int // incremented on each selection, used for debouncing

	// Dimensions
	width  int
	height int
}

// NewApp creates a new application instance
// scanPath is optional - if provided, scans that path instead of a drive
func NewApp(version string, scanPath string) App {
	drives, _ := model.GetDrives()

	// Initialize and load stats
	statsMgr := stats.NewManager()
	if err := statsMgr.Load(); err != nil {
		logging.Debug.Printf("Failed to load stats: %v", err)
	}

	// If custom path provided, skip drive selection logic
	if scanPath != "" {
		app := App{
			header:        NewHeader(drives, version),
			tree:          NewTreePanel(),
			treemap:       NewTreemapPanel(),
			help:          NewHelpOverlay(),
			driveSelector: NewDriveSelector(drives),
			keys:          DefaultKeyMap(),
			scanner:       scanner.NewWalker(8),
			statsManager:  statsMgr,
			freedLifetime: statsMgr.FreedLifetime(),
			drives:        drives,
			customPath:    scanPath,
			activePanel:   PanelTree,
			showDiff:      true,
			scanning:      true,
		}
		app.tree.SetFocused(true)
		app.treemap.SetFocused(false)
		app.header.SetScanning(true, "")
		app.header.SetFreedStats(app.freedThisSession, app.freedLifetime)
		return app
	}

	// Check if there's a saved default drive that's still available
	defaultDrive := statsMgr.DefaultDrive()
	defaultIdx := -1
	for i, d := range drives {
		if d.Path == defaultDrive {
			defaultIdx = i
			break
		}
	}

	// If no valid default, show drive selector on startup
	showDriveSelector := defaultIdx < 0 && len(drives) > 0

	app := App{
		header:        NewHeader(drives, version),
		tree:          NewTreePanel(),
		treemap:       NewTreemapPanel(),
		help:          NewHelpOverlay(),
		driveSelector: NewDriveSelector(drives),
		keys:    DefaultKeyMap(),
		scanner: scanner.NewWalker(8),
		statsManager:  statsMgr,
		freedLifetime: statsMgr.FreedLifetime(),
		drives:      drives,
		activePanel: PanelTree,
		showDiff:    true, // Show deleted items by default
		scanning:    !showDriveSelector && len(drives) > 0, // Only scan if we have a default
	}

	app.tree.SetFocused(true)
	app.treemap.SetFocused(false)

	// Set up initial state based on whether we have a saved default
	if showDriveSelector {
		app.driveSelector.SetVisible(true)
	} else if defaultIdx >= 0 {
		app.header.SetSelected(defaultIdx)
		app.header.SetScanning(true, "")
	} else if len(drives) > 0 {
		app.header.SetScanning(true, "")
	}

	// Update header with loaded stats
	app.header.SetFreedStats(app.freedThisSession, app.freedLifetime)

	return app
}

// Init implements tea.Model
func (a App) Init() tea.Cmd {
	// Set terminal title
	titleCmd := tea.SetWindowTitle("DISKDIVE")

	// Start scanning if we have a custom path or a valid drive
	if a.customPath != "" || (len(a.drives) > 0 && !a.driveSelector.IsVisible()) {
		return tea.Batch(titleCmd, func() tea.Msg {
			return scanStartMsg{}
		})
	}
	return titleCmd
}

// startScan starts scanning a path and returns a command
func (a App) startScan(path string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		logging.Debug.Printf("[UI] Starting scan of %s", path)
		root, err := a.scanner.Scan(ctx, path)
		logging.Debug.Printf("[UI] Scan completed, returning scanCompleteMsg (err=%v)", err)
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

	case scanStartMsg:
		// Now actually start the scan and spinner
		var scanPath string
		if a.customPath != "" {
			scanPath = a.customPath
		} else if drive := a.header.Selected(); drive != nil {
			scanPath = drive.Path
		}
		if scanPath != "" {
			a.scanPhase = phaseScanning
			a.scanStartTime = time.Now()
			a.scanFileCount = ""
			a.scanBytesFound = ""
			spinnerCmd := tea.Tick(spinnerTickInterval, func(t time.Time) tea.Msg {
				return spinnerTickMsg{}
			})
			progressCmd := a.listenForProgress()
			return a, tea.Batch(a.startScan(scanPath), spinnerCmd, progressCmd)
		}
		return a, nil

	case scanCompleteMsg:
		logging.Debug.Printf("[UI] Received scanCompleteMsg (err=%v)", msg.err)
		if msg.err != nil {
			a.scanning = false
			a.scanPhase = phaseDone
			a.err = msg.err
			a.header.SetScanning(false, "")
			logging.Debug.Printf("[UI] Scan failed with error: %v", msg.err)
			return a, nil
		}
		// Move to computing sizes phase
		logging.Debug.Printf("[UI] Moving to computing sizes phase")
		a.scanPhase = phaseComputingSizes
		return a, func() tea.Msg {
			return computeSizesMsg{root: msg.root}
		}

	case computeSizesMsg:
		logging.Debug.Printf("[UI] Received computeSizesMsg")
		return a, func() tea.Msg {
			start := time.Now()
			logging.Debug.Printf("[PHASE] Starting ComputeSizes...")
			msg.root.ComputeSizes()
			logging.Debug.Printf("[PHASE] ComputeSizes took %v", time.Since(start))
			return computeSizesDoneMsg{root: msg.root}
		}

	case computeSizesDoneMsg:
		logging.Debug.Printf("[UI] Sizes computed, showing complete for a moment")
		// Show "Complete" phase for 500ms before transitioning
		a.scanPhase = phaseComplete
		return a, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return scanCompleteDelayMsg{root: msg.root}
		})

	case scanCompleteDelayMsg:
		logging.Debug.Printf("[UI] Complete delay done, showing data to user")
		// Now actually show the data
		a.scanning = false
		a.scanPhase = phaseDone
		a.root = msg.root
		a.tree.SetRoot(msg.root)
		a.treemap.SetRoot(msg.root)
		a.tree.SetShowDiff(a.showDiff)
		a.treemap.SetShowDiff(a.showDiff)
		a.header.SetScanning(false, "")
		a.err = nil

		// Recalculate layout now that we have data (tree width depends on content)
		a.updateLayout()

		logging.Debug.Printf("[UI] UI ready, showing data")

		// Start filesystem watcher
		if drive := a.header.Selected(); drive != nil {
			return a, func() tea.Msg {
				return startWatcherMsg{root: drive.Path}
			}
		}
		return a, nil

	case startWatcherMsg:
		// Stop any existing watcher
		if a.watcher != nil {
			_ = a.watcher.Stop()
		}

		// Create and start new watcher
		w, err := watcher.New()
		if err != nil {
			logging.Debug.Printf("Failed to create watcher: %v", err)
			return a, nil
		}

		a.watcher = w
		if err := w.AddRecursive(msg.root); err != nil {
			logging.Debug.Printf("Failed to add recursive watch: %v", err)
		}
		w.Start()
		logging.Debug.Printf("Filesystem watcher started for %s", msg.root)

		// Start listening for watcher events
		return a, a.listenForWatcherEvents()

	case watcherEventMsg:
		// Handle filesystem change
		if msg.event.Type == watcher.EventDeleted {
			a.handleDeletion(msg.event.Path)
		}
		// Continue listening for more events
		return a, a.listenForWatcherEvents()

	case scanProgressMsg:
		a.scanFileCount = fmt.Sprintf("%d files", msg.progress.FilesScanned)
		a.scanBytesFound = FormatSize(msg.progress.BytesFound)
		a.scanBytesRaw = msg.progress.BytesFound
		elapsed := time.Since(a.scanStartTime).Truncate(time.Second)
		progress := fmt.Sprintf("%s, %s, %s", a.scanFileCount, a.scanBytesFound, elapsed)
		a.header.SetScanning(true, progress)
		// Keep listening for more progress
		return a, a.listenForProgress()

	case focusDebounceMsg:
		// Only apply focus if this is still the latest version (user stopped scrolling)
		if msg.version == a.focusVersion && msg.node != nil {
			a.treemap.SetFocus(msg.node)
		}
		return a, nil

	case spinnerTickMsg:
		// Keep ticking while scanning to force UI redraws
		if a.scanning || a.root == nil {
			return a, tea.Tick(spinnerTickInterval, func(t time.Time) tea.Msg {
				return spinnerTickMsg{}
			})
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
		// Clean up watcher and stats before quitting
		if a.watcher != nil {
			_ = a.watcher.Stop()
		}
		if a.statsManager != nil {
			_ = a.statsManager.Close()
		}
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
			a.treemap.SelectFirst()
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
			// Sync tree to match treemap selection
			if node := a.treemap.Selected(); node != nil {
				a.tree.ExpandTo(node)
				a.updateLayout()
			}
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
		a.updateLayout()
		return a, nil

	case key.Matches(msg, a.keys.Rescan):
		if !a.scanning {
			if drive := a.header.Selected(); drive != nil {
				return a, a.selectDrive(a.header.selected)
			}
		}
		return a, nil

	case key.Matches(msg, a.keys.OpenExplorer):
		logging.Debug.Printf("OpenExplorer key pressed")
		cmd := a.openInExplorer()
		return a, cmd
	}

	return a, nil
}

// selectDrive selects a drive and starts scanning
func (a *App) selectDrive(idx int) tea.Cmd {
	if idx < 0 || idx >= len(a.drives) {
		return nil
	}

	// Stop existing watcher
	if a.watcher != nil {
		_ = a.watcher.Stop()
		a.watcher = nil
	}

	// Reset session freed counter for new scan
	a.freedThisSession = 0
	a.header.SetFreedStats(a.freedThisSession, a.freedLifetime)

	a.header.SetSelected(idx)

	// Save as default drive for next startup
	if a.statsManager != nil {
		a.statsManager.SetDefaultDrive(a.drives[idx].Path)
	}
	a.scanning = true
	a.scanPhase = phaseScanning
	a.scanStartTime = time.Now()
	a.scanFileCount = ""
	a.scanBytesFound = ""
	a.header.SetScanning(true, "")
	a.root = nil
	a.tree.SetRoot(nil)
	a.treemap.SetRoot(nil)

	// Create a new scanner for each scan
	a.scanner = scanner.NewWalker(8)

	// Start scan, spinner, and progress listener
	spinnerCmd := tea.Tick(spinnerTickInterval, func(t time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
	progressCmd := a.listenForProgress()
	return tea.Batch(a.startScan(a.drives[idx].Path), spinnerCmd, progressCmd)
}

// syncSelection syncs the tree selection to the treemap
func (a *App) syncSelection() tea.Cmd {
	node := a.tree.Selected()
	if node == nil {
		return nil
	}
	a.treemap.SetSelected(node)

	// Determine focus target: for files, show parent directory so file appears among siblings
	var focusTarget *model.Node
	if node.IsDir && len(node.Children) > 0 {
		focusTarget = node
	} else if !node.IsDir && node.Parent != nil {
		focusTarget = node.Parent
	}

	// Schedule debounced focus update
	if focusTarget != nil {
		a.focusVersion++
		version := a.focusVersion
		return tea.Tick(focusDebounceTimeout, func(t time.Time) tea.Msg {
			return focusDebounceMsg{version: version, node: focusTarget}
		})
	}
	return nil
}

// syncSelectionFromTreemap syncs treemap selection to tree
func (a *App) syncSelectionFromTreemap() {
	// Note: We don't expand tree to match treemap selection
	// as that could be jarring. The treemap shows what's selected.
}

// listenForWatcherEvents returns a command that waits for the next watcher event
func (a *App) listenForWatcherEvents() tea.Cmd {
	if a.watcher == nil {
		return nil
	}

	return func() tea.Msg {
		event, ok := <-a.watcher.Events()
		if !ok {
			return nil // Channel closed
		}
		return watcherEventMsg{event: event}
	}
}

// listenForProgress returns a command that waits for progress updates from scanner
func (a *App) listenForProgress() tea.Cmd {
	return func() tea.Msg {
		progress, ok := <-a.scanner.Progress()
		if !ok {
			return nil // Channel closed (scan complete)
		}
		return scanProgressMsg{progress: progress}
	}
}

// handleDeletion handles a file/directory deletion event
func (a *App) handleDeletion(path string) {
	if a.root == nil {
		return
	}

	// Find the node by path
	node := a.findNodeByPath(a.root, path)
	if node == nil {
		logging.Debug.Printf("Watcher: DELETE event for path not in tree: %s", path)
		return
	}

	// Already marked as deleted
	if node.IsDeleted {
		return
	}

	// Get size before marking as deleted
	size := node.TotalSize()

	// Mark as deleted (also propagates to ancestors)
	node.MarkDeleted()
	logging.Debug.Printf("Watcher: MARKED DELETED: %s (size: %d)", path, size)

	// Only count deletions over 200KB in freed stats (filters out small OS file changes)
	const minFreedSize = 200 * 1024 // 200KB
	if size >= minFreedSize {
		// Update freed counters
		a.freedThisSession += size
		a.freedLifetime += size

		// Update stats manager (will debounce saves)
		if a.statsManager != nil {
			a.statsManager.AddFreed(size)
		}

		// Update header display
		a.header.SetFreedStats(a.freedThisSession, a.freedLifetime)

		logging.Debug.Printf("Watcher: marked %s as deleted, freed %d bytes", path, size)
	}
}

// findNodeByPath searches for a node by its path
func (a *App) findNodeByPath(node *model.Node, path string) *model.Node {
	if node.Path == path {
		return node
	}

	for _, child := range node.Children {
		if found := a.findNodeByPath(child, path); found != nil {
			return found
		}
	}

	return nil
}

// openInExplorer opens the selected item in the system file manager (revealed with selection)
func (a *App) openInExplorer() tea.Cmd {
	node := a.tree.Selected()
	if node == nil {
		logging.Debug.Printf("openInExplorer: no node selected")
		return nil
	}

	logging.Debug.Printf("openInExplorer: revealing %s", node.Path)
	// Open in file manager (platform-specific implementation reveals item in parent)
	if err := openInFileManager(node.Path); err != nil {
		logging.Debug.Printf("openInExplorer: error: %v", err)
	}
	return nil
}

// updateLayout calculates component sizes based on window dimensions
func (a *App) updateLayout() {
	// Header height (2 lines + separator)
	headerHeight := 3

	// Help bar height (1 line)
	helpBarHeight := 1

	// Info bar height (1 line + separator)
	infoBarHeight := 2

	// Available height for panels
	panelHeight := a.height - headerHeight - helpBarHeight - infoBarHeight
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

// renderSpinningBorder draws a box with a gradient border that spins over time
func renderSpinningBorder(content string, width, height int, t time.Time) string {
	// Cyberpunk neon gradient: cyan → blue → purple → magenta → pink
	// Use theme colors with smooth transitions: cyan -> teal -> violet -> pink -> back
	shades := []string{
		"#00FFFF", // cyan
		"#30EBE0", // cyan->teal
		"#5EEAD4", // teal
		"#70E0D8", //
		"#85D5E0", // teal->violet
		"#9AC5E8", //
		"#A8B0F0", //
		"#B89AF8", //
		"#C084FC", // violet
		"#C880F0", //
		"#D080E8", // violet->pink
		"#D87CDE", //
		"#E07CD4", //
		"#F079CC", //
		"#FF79C6", // pink
		"#F079CC", //
		"#E07CD4", // pink->violet
		"#D87CDE", //
		"#D080E8", //
		"#C880F0", //
		"#C084FC", // violet
		"#B89AF8", //
		"#A8B0F0", //
		"#9AC5E8", // violet->teal
		"#85D5E0", //
		"#70E0D8", //
		"#5EEAD4", // teal
		"#30EBE0", // teal->cyan
	}

	// Calculate perimeter positions
	// Top: width chars, Right: height-2 chars, Bottom: width chars, Left: height-2 chars
	innerW := width - 2
	innerH := height - 2
	perimeter := 2*innerW + 2*innerH + 4 // +4 for corners

	// Time-based offset for spinning effect (reverse direction)
	offset := int(t.UnixMilli()/borderRotationSpeed) % perimeter

	// Helper to get color at position
	getColor := func(pos int) lipgloss.Style {
		// Adjust position by offset for spinning (subtract for reverse direction)
		adjustedPos := (pos - offset + perimeter) % perimeter
		// Map position to shade (spread shades across perimeter)
		shadeIdx := (adjustedPos * len(shades) / perimeter) % len(shades)
		return lipgloss.NewStyle().Foreground(lipgloss.Color(shades[shadeIdx]))
	}

	// Border characters (rounded)
	const (
		topLeft     = "╭"
		topRight    = "╮"
		bottomLeft  = "╰"
		bottomRight = "╯"
		horizontal  = "─"
		vertical    = "│"
	)

	var result strings.Builder
	pos := 0

	// Top border
	result.WriteString(getColor(pos).Render(topLeft))
	pos++
	for i := 0; i < innerW; i++ {
		result.WriteString(getColor(pos).Render(horizontal))
		pos++
	}
	result.WriteString(getColor(pos).Render(topRight))
	pos++
	result.WriteString("\n")

	// Content lines with side borders
	contentLines := strings.Split(content, "\n")
	// Pad content to fill height
	for len(contentLines) < innerH {
		contentLines = append(contentLines, "")
	}

	for i := 0; i < innerH; i++ {
		// Left border (going down)
		leftColor := getColor(perimeter - 1 - i) // Left side goes in reverse
		result.WriteString(leftColor.Render(vertical))

		// Content line (pad to width)
		line := ""
		if i < len(contentLines) {
			line = contentLines[i]
		}
		// Pad line to inner width
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerW {
			line += strings.Repeat(" ", innerW-lineWidth)
		}
		result.WriteString(line)

		// Right border (going down)
		result.WriteString(getColor(pos).Render(vertical))
		pos++
		result.WriteString("\n")
	}

	// Bottom border (going right to left visually, but we write left to right)
	bottomStart := pos
	result.WriteString(getColor(perimeter - innerH - 1).Render(bottomLeft))
	for i := 0; i < innerW; i++ {
		result.WriteString(getColor(bottomStart + innerW - i).Render(horizontal))
	}
	result.WriteString(getColor(bottomStart).Render(bottomRight))

	return result.String()
}

// infoBar creates the info bar showing details about the selected node
func (a App) infoBar() string {
	node := a.tree.Selected()
	if node == nil {
		return ""
	}

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	iconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	fieldSep := dimStyle

	// Get file info
	var modTimeStr, createTimeStr string
	if info, err := os.Stat(node.Path); err == nil {
		now := time.Now()

		// Modification time
		modTime := info.ModTime()
		if modTime.Year() == now.Year() {
			modTimeStr = modTime.Format("Jan 2 15:04")
		} else {
			modTimeStr = modTime.Format("Jan 2, 2006")
		}

		// Creation time (platform-specific)
		if createTime := getCreationTime(info); !createTime.IsZero() {
			if createTime.Year() == now.Year() {
				createTimeStr = createTime.Format("Jan 2 15:04")
			} else {
				createTimeStr = createTime.Format("Jan 2, 2006")
			}
		}
	}

	// Type icon
	var icon string
	if node.IsDir {
		icon = iconStyle.Render("📁")
	} else {
		icon = iconStyle.Render("📄")
	}

	// Build info parts
	sep := fieldSep.Render(" │ ")
	name := nameStyle.Render(node.Name)
	size := dimStyle.Render(FormatSize(node.TotalSize()))

	var parts []string
	parts = append(parts, icon, " ", name, sep, size)

	// For directories, show file count
	if node.IsDir {
		count := countFiles(node)
		countStr := dimStyle.Render(fmt.Sprintf("%d files", count))
		parts = append(parts, sep, countStr)
	}

	// Add creation time if available
	if createTimeStr != "" {
		createLabel := dimStyle.Render("Created: " + createTimeStr)
		parts = append(parts, sep, createLabel)
	}

	// Add modification time if available
	if modTimeStr != "" {
		modLabel := dimStyle.Render("Modified: " + modTimeStr)
		parts = append(parts, sep, modLabel)
	}

	line := " " + strings.Join(parts, "") // Add left padding to align with panel content

	// Truncate if too long
	if lipgloss.Width(line) > a.width {
		line = lipgloss.NewStyle().MaxWidth(a.width).Render(line)
	}

	// Add divider underneath (same style as header separator)
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3F3F46"))
	separator := sepStyle.Render(strings.Repeat("─", a.width))

	return line + "\n" + separator
}

// countFiles counts all files (not directories) in a node tree
func countFiles(node *model.Node) int {
	if !node.IsDir {
		return 1
	}
	count := 0
	for _, child := range node.Children {
		count += countFiles(child)
	}
	return count
}

// View implements tea.Model
func (a App) View() string {

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

		// Build terminal-style log lines (boot style - only show up to current phase)
		var logLines []string

		// Styles for different states
		doneStyle := lipgloss.NewStyle().Foreground(ColorSuccess)
		spinnerStyle := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)

		// Time-based spinner
		spinnerIdx := int(time.Now().UnixMilli()/int64(spinnerTickInterval.Milliseconds())) % len(spinnerFrames)
		spinner := spinnerFrames[spinnerIdx]

		// Calculate scan progress for dots (based on bytes scanned vs used disk space)
		// Only show progress bar when scanning a full drive (not custom paths)
		var progressBar string
		if a.customPath == "" && a.header.Selected() != nil && a.header.Selected().UsedBytes() > 0 {
			drive := a.header.Selected()
			progress := float64(a.scanBytesRaw) / float64(drive.UsedBytes())
			if progress > 1.0 {
				progress = 1.0
			}
			maxDots := 20 // max dots at 100% (fits in panel with brackets)
			numDots := int(progress * float64(maxDots))
			emptyDots := maxDots - numDots
			dotStyle := lipgloss.NewStyle().Foreground(ColorCyan)
			emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3F3F46"))
			bracketStyle := lipgloss.NewStyle().Foreground(ColorCyan)
			progressBar = " " + bracketStyle.Render("[") + dotStyle.Render(strings.Repeat("·", numDots)) + emptyStyle.Render(strings.Repeat("·", emptyDots)) + bracketStyle.Render("]")
		}

		// Cyberpunk stat styles
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")) // dim gray
		fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFFF")).Bold(true)  // cyan
		dataStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#C084FC")).Bold(true)  // purple
		timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Bold(true)  // amber

		// Only show phases up to and including current (boot-style log)
		for _, phase := range phases {
			if phase.name == "" || phase.id > a.scanPhase {
				break // Don't show future phases or unnamed phases
			}
			var line string
			if phase.id < a.scanPhase || phase.id == phaseComplete {
				// Completed phase (or the Complete phase itself shows as done)
				check := doneStyle.Render("✓")
				text := doneStyle.Render(phase.name)
				line = fmt.Sprintf("  %s %s", check, text)
			} else {
				// Current phase with spinner
				spin := spinnerStyle.Render(spinner)
				textStyle := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
				text := textStyle.Render(phase.name)
				line = fmt.Sprintf("  %s %s%s", spin, text, progressBar)
			}
			logLines = append(logLines, line)
		}

		// Add stats on separate lines during/after scanning phase
		if a.scanFileCount != "" {
			elapsed := time.Since(a.scanStartTime).Truncate(time.Second)
			logLines = append(logLines, "")
			logLines = append(logLines, fmt.Sprintf("    %s %s", labelStyle.Render("FILES"), fileStyle.Render(a.scanFileCount)))
			logLines = append(logLines, fmt.Sprintf("    %s  %s", labelStyle.Render("DATA"), dataStyle.Render(a.scanBytesFound)))
			logLines = append(logLines, fmt.Sprintf("    %s  %s", labelStyle.Render("TIME"), timeStyle.Render(elapsed.String())))
		}

		logContent := strings.Join(logLines, "\n")

		// Render content centered within box
		innerContent := lipgloss.NewStyle().
			Padding(0, 3).
			Width(48). // 50 - 2 for border
			Render(logContent)

		// Build spinning gradient border with fixed height, content centered
		boxHeight := 9 // fixed height for consistent look
		scanningBox := renderSpinningBorder(
			lipgloss.Place(48, boxHeight-2, lipgloss.Left, lipgloss.Center, innerContent),
			50, boxHeight, time.Now())

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

		// Info bar (shows selected item details)
		if infoBar := a.infoBar(); infoBar != "" {
			sections = append(sections, infoBar)
		}
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
			lipgloss.WithWhitespaceForeground(ColorBackground),
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
			lipgloss.WithWhitespaceForeground(ColorBackground),
		)
	}

	return content
}
