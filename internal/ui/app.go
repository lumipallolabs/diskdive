package ui

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
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

// loadCacheMsg triggers cache loading phase
type loadCacheMsg struct {
	root *model.Node
}

// loadCacheDoneMsg is sent when cache loading completes
type loadCacheDoneMsg struct {
	root *model.Node
	prev *model.Node
}

// applyDiffMsg triggers diff computation phase
type applyDiffMsg struct {
	root *model.Node
	prev *model.Node
}

// applyDiffDoneMsg is sent when diff computation completes
type applyDiffDoneMsg struct {
	root *model.Node
}

// saveCacheMsg triggers cache saving phase
type saveCacheMsg struct {
	root *model.Node
}

// saveCacheDoneMsg is sent when cache saving completes
type saveCacheDoneMsg struct {
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

// Spinner frames - cyberpunk style
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Scan phases
const (
	phaseScanning = iota
	phaseComputingSizes
	phaseLoadingCache
	phaseComparingChanges
	phaseSavingCache
	phaseDone
)

var phaseNames = []string{
	"Scanning files",
	"Computing sizes",
	"Loading cache",
	"Comparing changes",
	"Saving cache",
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
	activePanel    Panel
	sortMode       SortMode
	showDiff       bool
	scanning       bool
	scanPhase      int    // current phase index (0-4)
	scanFileCount  string // "1,234 files" from scanning
	scanBytesFound string // "50 GB" from scanning
	err            error
	spinnerFrame   int
	focusVersion   int // incremented on each selection, used for debouncing

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
	// Set terminal title
	titleCmd := tea.SetWindowTitle("DISKDIVE")

	// Start scanning first drive if available
	// We send scanStartMsg first to allow the UI to render the scanning state
	if len(a.drives) > 0 {
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
		debugLog.Printf("[UI] Starting scan of %s", path)
		root, err := a.scanner.Scan(ctx, path)
		debugLog.Printf("[UI] Scan completed, returning scanCompleteMsg (err=%v)", err)
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
		// Now actually start the scan and spinner
		if len(a.drives) > 0 {
			a.scanPhase = phaseScanning
			a.scanFileCount = ""
			a.scanBytesFound = ""
			spinnerCmd := tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
				return spinnerTickMsg{}
			})
			return a, tea.Batch(a.startScan(a.drives[0].Path), spinnerCmd)
		}
		return a, nil

	case scanCompleteMsg:
		debugLog.Printf("[UI] Received scanCompleteMsg (err=%v)", msg.err)
		if msg.err != nil {
			a.scanning = false
			a.scanPhase = phaseDone
			a.err = msg.err
			a.header.SetScanning(false, "")
			debugLog.Printf("[UI] Scan failed with error: %v", msg.err)
			return a, nil
		}
		// Move to computing sizes phase
		debugLog.Printf("[UI] Moving to computing sizes phase")
		a.scanPhase = phaseComputingSizes
		return a, func() tea.Msg {
			return computeSizesMsg{root: msg.root}
		}

	case computeSizesMsg:
		debugLog.Printf("[UI] Received computeSizesMsg")
		return a, func() tea.Msg {
			start := time.Now()
			debugLog.Printf("[PHASE] Starting ComputeSizes...")
			msg.root.ComputeSizes()
			debugLog.Printf("[PHASE] ComputeSizes took %v", time.Since(start))
			return computeSizesDoneMsg{root: msg.root}
		}

	case computeSizesDoneMsg:
		// Move to loading cache phase
		a.scanPhase = phaseLoadingCache
		return a, func() tea.Msg {
			return loadCacheMsg{root: msg.root}
		}

	case loadCacheMsg:
		return a, func() tea.Msg {
			start := time.Now()
			var prev *model.Node
			if drive := a.header.Selected(); drive != nil {
				prev, _ = a.cache.LoadLatest(drive.Letter)
			}
			debugLog.Printf("[PHASE] LoadCache took %v", time.Since(start))
			return loadCacheDoneMsg{root: msg.root, prev: prev}
		}

	case loadCacheDoneMsg:
		a.prevRoot = msg.prev
		// Move to diff phase
		a.scanPhase = phaseComparingChanges
		return a, func() tea.Msg {
			return applyDiffMsg{root: msg.root, prev: msg.prev}
		}

	case applyDiffMsg:
		return a, func() tea.Msg {
			start := time.Now()
			cache.ApplyDiff(msg.root, msg.prev)
			debugLog.Printf("[PHASE] ApplyDiff took %v", time.Since(start))
			return applyDiffDoneMsg{root: msg.root}
		}

	case applyDiffDoneMsg:
		debugLog.Printf("[UI] Diff complete, showing data to user (cache save in background)")
		// Show data immediately, save cache in background
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

		// Save cache in background (don't block UI)
		go func() {
			start := time.Now()
			if drive := a.header.Selected(); drive != nil {
				_ = a.cache.Save(drive.Letter, msg.root)
			}
			debugLog.Printf("[PHASE] SaveCache (background) took %v", time.Since(start))
		}()

		debugLog.Printf("[UI] UI ready, showing data")
		return a, nil

	case scanProgressMsg:
		a.scanFileCount = fmt.Sprintf("%d files", msg.progress.FilesScanned)
		a.scanBytesFound = FormatSize(msg.progress.BytesFound)
		progress := fmt.Sprintf("%s, %s", a.scanFileCount, a.scanBytesFound)
		a.header.SetScanning(true, progress)
		return a, nil

	case focusDebounceMsg:
		// Only apply focus if this is still the latest version (user stopped scrolling)
		if msg.version == a.focusVersion && msg.node != nil {
			a.treemap.SetFocus(msg.node)
		}
		return a, nil

	case spinnerTickMsg:
		// Keep ticking while scanning to force UI redraws
		if a.scanning || a.root == nil {
			return a, tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
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

	case key.Matches(msg, a.keys.OpenExplorer):
		return a, a.openInExplorer()
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
	a.scanPhase = phaseScanning
	a.scanFileCount = ""
	a.scanBytesFound = ""
	a.header.SetScanning(true, "")
	a.root = nil
	a.prevRoot = nil
	a.tree.SetRoot(nil)
	a.treemap.SetRoot(nil)

	// Create a new scanner for each scan
	a.scanner = scanner.NewWalker(8)

	// Start scan and spinner
	spinnerCmd := tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
	return tea.Batch(a.startScan(a.drives[idx].Path), spinnerCmd)
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

// openInExplorer opens the selected directory in the system file manager
func (a *App) openInExplorer() tea.Cmd {
	node := a.tree.Selected()
	if node == nil {
		return nil
	}

	path := node.Path
	// If it's a file, open its parent directory
	if !node.IsDir && node.Parent != nil {
		path = node.Parent.Path
	}

	// Open in file manager (platform-specific implementation)
	_ = openInFileManager(path)
	return nil
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

// renderSpinningBorder draws a box with a gradient border that spins over time
func renderSpinningBorder(content string, width, height int, t time.Time) string {
	// Cyberpunk neon gradient: cyan → blue → purple → magenta → pink
	shades := []string{
		"#00FFFF", // cyan
		"#00D4FF", // light blue
		"#00AAFF", // sky blue
		"#0080FF", // blue
		"#4060FF", // indigo
		"#8040FF", // violet
		"#A020F0", // purple
		"#C020C0", // magenta
		"#E040A0", // pink-magenta
		"#FF60B0", // hot pink
		"#E040A0", // pink-magenta
		"#C020C0", // magenta
		"#A020F0", // purple
		"#8040FF", // violet
		"#4060FF", // indigo
		"#0080FF", // blue
		"#00AAFF", // sky blue
		"#00D4FF", // light blue
	}

	// Calculate perimeter positions
	// Top: width chars, Right: height-2 chars, Bottom: width chars, Left: height-2 chars
	innerW := width - 2
	innerH := height - 2
	perimeter := 2*innerW + 2*innerH + 4 // +4 for corners

	// Time-based offset for spinning effect (reverse direction)
	offset := int(t.UnixMilli()/50) % perimeter

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

		// Build terminal-style log lines (boot style - only show up to current phase)
		var logLines []string

		// Styles for different states
		doneStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#39FF14")) // neon green
		activeStyle := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)

		// Time-based spinner
		spinnerIdx := int(time.Now().UnixMilli()/80) % len(spinnerFrames)
		spinner := spinnerFrames[spinnerIdx]

		// Only show phases up to and including current (boot-style log)
		for i, name := range phaseNames {
			if i > a.scanPhase {
				break // Don't show future phases
			}
			var line string
			if i < a.scanPhase {
				// Completed phase
				check := doneStyle.Render("✓")
				text := doneStyle.Render(name)
				// Add stats for scanning phase
				if i == phaseScanning && a.scanFileCount != "" {
					stats := doneStyle.Render(fmt.Sprintf(" · %s · %s", a.scanFileCount, a.scanBytesFound))
					line = fmt.Sprintf("  %s %s%s", check, text, stats)
				} else {
					line = fmt.Sprintf("  %s %s", check, text)
				}
			} else {
				// Current phase (i == a.scanPhase)
				spin := activeStyle.Render(spinner)
				text := activeStyle.Render(name)
				// Animated dots (cycle through ., .., ...)
				dotCount := (int(time.Now().UnixMilli()/400) % 3) + 1
				dots := activeStyle.Render(strings.Repeat(".", dotCount))
				// Add live stats for scanning phase
				if i == phaseScanning && a.scanFileCount != "" {
					stats := activeStyle.Render(fmt.Sprintf(" · %s · %s", a.scanFileCount, a.scanBytesFound))
					line = fmt.Sprintf("  %s %s%s%s", spin, text, dots, stats)
				} else {
					line = fmt.Sprintf("  %s %s%s", spin, text, dots)
				}
			}
			logLines = append(logLines, line)
		}

		// Pad with empty lines at top to keep panel height constant (5 phases)
		totalLines := len(phaseNames)
		for len(logLines) < totalLines {
			logLines = append([]string{""}, logLines...)
		}

		logContent := strings.Join(logLines, "\n")

		// Render content with padding (no border - we'll draw it manually)
		innerContent := lipgloss.NewStyle().
			Padding(1, 3).
			Width(48). // 50 - 2 for border
			Height(totalLines).
			Render(logContent)

		// Build spinning gradient border
		scanningBox := renderSpinningBorder(innerContent, 50, totalLines+4, time.Now())

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
