package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gabriel-vasile/mimetype"
	"github.com/samuli/diskdive/internal/core"
	"github.com/samuli/diskdive/internal/logging"
	"github.com/samuli/diskdive/internal/model"
)

// Panel identifies which panel is active
type Panel int

const (
	PanelTree Panel = iota
	PanelTreemap
)

// Message types for Bubble Tea
type (
	scanStartMsg         struct{}
	deletionDetectedMsg  struct{ event core.DeletionDetectedEvent }
	creationDetectedMsg  struct{ event core.CreationDetectedEvent }
	focusDebounceMsg     struct {
		version int
		node    *model.Node
	}
	spinnerTickMsg       struct{}
	scanCompleteDelayMsg struct{ root *model.Node }
)

// Spinner frames - modern braille dots spinner
var spinnerFrames = []string{
	"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
}

// Timing constants
const (
	spinnerTickInterval  = 80 * time.Millisecond
	borderRotationSpeed  = 33  // milliseconds per frame
	focusDebounceTimeout = 300 * time.Millisecond
)

// App is the main TUI application model
type App struct {
	// Core controller (business logic)
	ctrl *core.Controller

	// UI Components
	header        Header
	tree          TreePanel
	treemap       TreemapPanel
	help          HelpOverlay
	driveSelector DriveSelector
	keys          KeyMap
	version       string

	// UI state (TUI-specific)
	activePanel  Panel
	err          error
	focusVersion int // for debouncing

	// Event channels (for continuing to listen after each event)
	scanEventCh    <-chan core.Event
	watcherEventCh <-chan core.Event

	// Dimensions
	width           int
	height          int
	rightPanelWidth int
}

// NewApp creates a new application instance
func NewApp(version string, scanPath string) App {
	ctrl := core.NewController(scanPath)
	drives := ctrl.Drives()

	app := App{
		ctrl:          ctrl,
		header:        NewHeader(drives, version),
		tree:          NewTreePanel(),
		treemap:       NewTreemapPanel(),
		help:          NewHelpOverlay(version),
		driveSelector: NewDriveSelector(drives),
		keys:          DefaultKeyMap(),
		version:       version,
		activePanel:   PanelTree,
	}

	app.tree.SetFocused(true)
	app.treemap.SetFocused(false)

	// Set up initial state
	if scanPath != "" {
		// Custom path - start scanning immediately
		app.header.SetScanning(true, "")
	} else if ctrl.HasSavedDefaultDrive() {
		// Has saved default - select it and prepare to scan
		app.header.SetSelected(ctrl.SelectedDriveIndex())
		app.header.SetScanning(true, "")
	} else if len(drives) > 0 {
		// No default - show drive selector
		app.driveSelector.SetVisible(true)
	}

	// Update header with loaded stats
	freed := ctrl.FreedState()
	app.header.SetFreedStats(freed.Session, freed.Lifetime)

	return app
}

// Init implements tea.Model
func (a App) Init() tea.Cmd {
	// Start scanning if we have a target
	if a.ctrl.CustomPath() != "" || (len(a.ctrl.Drives()) > 0 && !a.driveSelector.IsVisible()) {
		return func() tea.Msg {
			return scanStartMsg{}
		}
	}
	return nil
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
		return a.startScan()

	case scanEventMsg:
		// Handle scan events and always continue listening
		return a.handleScanEvent(msg.event)

	case scanCompleteDelayMsg:
		return a.finalizeScan(msg.root)

	case deletionDetectedMsg:
		a.header.SetFreedStats(msg.event.SessionFreed, msg.event.TotalFreed)
		if msg.event.DiskFree > 0 {
			a.header.UpdateDiskFree(msg.event.DiskFree)
		}
		a.tree.RefreshVisible()
		a.treemap.InvalidateCache()
		return a, a.listenForWatcherEvents()

	case creationDetectedMsg:
		logging.Debug.Printf("[TUI] creationDetectedMsg received for path: %s", msg.event.Path)
		if msg.event.DiskFree > 0 {
			a.header.UpdateDiskFree(msg.event.DiskFree)
		}
		logging.Debug.Printf("[TUI] calling tree.RefreshVisible()")
		a.tree.RefreshVisible()
		a.treemap.InvalidateCache()
		logging.Debug.Printf("[TUI] creationDetectedMsg processing complete")
		return a, a.listenForWatcherEvents()

	case focusDebounceMsg:
		if msg.version == a.focusVersion && msg.node != nil {
			a.treemap.SetFocus(msg.node)
		}
		return a, nil

	case spinnerTickMsg:
		state := a.ctrl.ScanState()
		if state.IsScanning() || a.ctrl.Root() == nil {
			return a, tea.Tick(spinnerTickInterval, func(t time.Time) tea.Msg {
				return spinnerTickMsg{}
			})
		}
		return a, nil
	}

	return a, nil
}

// handleScanEvent processes scan events and continues listening
func (a App) handleScanEvent(event core.Event) (tea.Model, tea.Cmd) {
	switch e := event.(type) {
	case core.ScanProgressEvent:
		state := a.ctrl.ScanState()
		progress := fmt.Sprintf("%d files, %s, %s",
			state.FilesScanned,
			FormatSize(state.BytesFound),
			state.Elapsed())
		a.header.SetScanning(true, progress)
		return a, a.listenForScanEvents()

	case core.ScanPhaseChangedEvent:
		logging.Debug.Printf("[TUI] Phase changed to: %s", e.Phase)
		return a, a.listenForScanEvents()

	case core.ScanCompletedEvent:
		if e.Err != nil {
			a.err = e.Err
			a.header.SetScanning(false, "")
			return a, nil
		}
		// Show "Complete" briefly before showing data
		return a, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return scanCompleteDelayMsg{root: e.Root}
		})

	default:
		// Unknown event (like ScanStartedEvent) - just continue listening
		return a, a.listenForScanEvents()
	}
}

// startScan begins the scanning process
func (a App) startScan() (tea.Model, tea.Cmd) {
	ctx := context.Background()
	eventCh, err := a.ctrl.StartScan(ctx)
	if err != nil {
		a.err = err
		return a, nil
	}
	if eventCh == nil {
		return a, nil
	}

	// Store channel for continued listening
	a.scanEventCh = eventCh

	// Start listening for events and ticking spinner
	return a, tea.Batch(
		a.listenForScanEvents(),
		tea.Tick(spinnerTickInterval, func(t time.Time) tea.Msg {
			return spinnerTickMsg{}
		}),
	)
}

// scanEventMsg wraps any scan event for continued listening
type scanEventMsg struct {
	event core.Event
}

// listenForScanEvents creates a command that listens for scan events
func (a App) listenForScanEvents() tea.Cmd {
	if a.scanEventCh == nil {
		return nil
	}
	eventCh := a.scanEventCh
	return func() tea.Msg {
		event, ok := <-eventCh
		if !ok {
			return nil // Channel closed
		}
		return scanEventMsg{event: event}
	}
}

// finalizeScan completes the scan and shows data
func (a App) finalizeScan(root *model.Node) (tea.Model, tea.Cmd) {
	a.ctrl.FinalizeScan()
	a.tree.SetRoot(root)
	a.treemap.SetRoot(root)
	a.header.SetScanning(false, "")
	a.err = nil
	a.updateLayout()

	// Start filesystem watcher
	return a, a.startWatcher()
}

// startWatcher starts watching for deletions
func (a *App) startWatcher() tea.Cmd {
	eventCh, err := a.ctrl.StartWatching()
	if err != nil || eventCh == nil {
		return nil
	}
	a.watcherEventCh = eventCh
	return a.listenForWatcherEvents()
}

// listenForWatcherEvents creates a command that listens for watcher events
func (a App) listenForWatcherEvents() tea.Cmd {
	if a.watcherEventCh == nil {
		return nil
	}
	eventCh := a.watcherEventCh
	return func() tea.Msg {
		event, ok := <-eventCh
		if !ok {
			return nil // Channel closed
		}
		switch e := event.(type) {
		case core.DeletionDetectedEvent:
			return deletionDetectedMsg{event: e}
		case core.CreationDetectedEvent:
			return creationDetectedMsg{event: e}
		}
		return nil
	}
}

// handleKey handles keyboard input
func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Help overlay - any key closes it
	if a.help.IsVisible() {
		a.help.SetVisible(false)
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
			return a.selectDrive(a.driveSelector.Selected())
		}
		return a, nil
	}

	switch {
	case key.Matches(msg, a.keys.Quit):
		a.ctrl.Stop()
		return a, tea.Quit

	case key.Matches(msg, a.keys.Help):
		a.help.Toggle()
		return a, nil

	case key.Matches(msg, a.keys.SelectDrive):
		if len(a.ctrl.Drives()) > 0 {
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
			if node := a.treemap.Selected(); node != nil {
				a.tree.ExpandTo(node)
				a.updateLayout()
			}
		} else {
			a.tree.Toggle()
			a.updateLayout()
			return a, a.syncSelection()
		}
		return a, nil

	case key.Matches(msg, a.keys.Back):
		if a.activePanel == PanelTreemap {
			a.treemap.ZoomOut()
		} else {
			a.tree.Collapse()
			a.updateLayout()
		}
		return a, nil

	case key.Matches(msg, a.keys.Rescan):
		state := a.ctrl.ScanState()
		if !state.IsScanning() {
			if a.ctrl.SelectedDrive() != nil {
				return a.selectDrive(a.ctrl.SelectedDriveIndex())
			}
		}
		return a, nil

	case key.Matches(msg, a.keys.OpenExplorer):
		return a, a.openInExplorer()

	case key.Matches(msg, a.keys.Preview):
		return a, a.previewFile()
	}

	return a, nil
}

// selectDrive selects a drive and starts scanning
func (a *App) selectDrive(idx int) (tea.Model, tea.Cmd) {
	if err := a.ctrl.SelectDrive(idx); err != nil {
		a.err = err
		return a, nil
	}

	freed := a.ctrl.FreedState()
	a.header.SetFreedStats(freed.Session, freed.Lifetime)
	a.header.SetSelected(idx)
	a.header.SetScanning(true, "")
	a.tree.SetRoot(nil)
	a.treemap.SetRoot(nil)

	return a.startScan()
}

// syncSelection syncs tree selection to treemap
func (a *App) syncSelection() tea.Cmd {
	node := a.tree.Selected()
	if node == nil {
		return nil
	}
	a.treemap.SetSelected(node)

	var focusTarget *model.Node
	if node.IsDir && len(node.Children) > 0 {
		focusTarget = node
	} else if !node.IsDir && node.Parent != nil {
		focusTarget = node.Parent
	}

	if focusTarget == nil {
		return nil
	}

	// For directories, update immediately; for files, debounce
	if node.IsDir {
		a.treemap.SetFocus(focusTarget)
		return nil
	}

	a.focusVersion++
	version := a.focusVersion
	return tea.Tick(focusDebounceTimeout, func(t time.Time) tea.Msg {
		return focusDebounceMsg{version: version, node: focusTarget}
	})
}

// syncSelectionFromTreemap syncs treemap selection to tree
func (a *App) syncSelectionFromTreemap() {
	// Don't expand tree to match - could be jarring
}

// openInExplorer opens the selected item in file manager
func (a *App) openInExplorer() tea.Cmd {
	node := a.tree.Selected()
	if node == nil {
		return nil
	}
	logging.Debug.Printf("openInExplorer: revealing %s", node.Path)
	if err := openInFileManager(node.Path); err != nil {
		logging.Debug.Printf("openInExplorer: error: %v", err)
	}
	return nil
}

// previewFile opens Quick Look preview
func (a *App) previewFile() tea.Cmd {
	node := a.tree.Selected()
	if node == nil {
		return nil
	}
	logging.Debug.Printf("previewFile: previewing %s", node.Path)
	if err := previewInQuickLook(node.Path); err != nil {
		logging.Debug.Printf("previewFile: error: %v", err)
	}
	return nil
}

// updateLayout calculates component sizes
func (a *App) updateLayout() {
	headerHeight := 2
	helpBarHeight := 1
	infoBarHeight := 2

	panelHeight := a.height - headerHeight - helpBarHeight
	if panelHeight < 1 {
		panelHeight = 1
	}

	treeWidth := a.tree.RequiredWidth()
	maxTreeWidth := a.width / 2
	if treeWidth > maxTreeWidth {
		treeWidth = maxTreeWidth
	}
	if treeWidth < 20 {
		treeWidth = 20
	}

	a.header.SetWidth(a.width)
	a.tree.SetSize(treeWidth, panelHeight)
	a.rightPanelWidth = a.width - treeWidth
	a.treemap.SetSize(a.rightPanelWidth, panelHeight-infoBarHeight)
	a.help.SetSize(a.width, a.height)
	a.driveSelector.SetSize(a.width, a.height)
}

// View implements tea.Model
func (a App) View() string {
	state := a.ctrl.ScanState()
	root := a.ctrl.Root()

	if a.width == 0 || a.height == 0 {
		if state.IsScanning() {
			return "Scanning drive..."
		}
		return "Loading..."
	}

	var sections []string
	sections = append(sections, a.header.View())

	if a.err != nil {
		errStyle := lipgloss.NewStyle().
			Foreground(ColorDanger).
			Padding(0, 1)
		sections = append(sections, errStyle.Render(fmt.Sprintf("Error: %v", a.err)))
	}

	if state.IsScanning() || root == nil {
		sections = append(sections, a.renderScanningPanel(state))
	} else {
		sections = append(sections, a.renderMainPanels())
	}

	sections = append(sections, HelpBar(a.width))
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Overlays
	if a.help.IsVisible() {
		return a.renderOverlay(a.help.View())
	}
	if a.driveSelector.IsVisible() {
		return a.renderOverlay(a.driveSelector.View())
	}

	return content
}

// renderOverlay renders an overlay centered on screen
func (a App) renderOverlay(overlay string) string {
	return lipgloss.Place(
		a.width, a.height,
		lipgloss.Center, lipgloss.Center,
		overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(ColorBackground),
	)
}

// renderScanningPanel renders the scanning progress panel
func (a App) renderScanningPanel(state core.ScanState) string {
	panelHeight := a.height - 4
	if panelHeight < 1 {
		panelHeight = 1
	}

	var logLines []string
	doneStyle := lipgloss.NewStyle().Foreground(ColorSuccess)
	spinnerStyle := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)

	spinnerIdx := int(time.Now().UnixMilli()/int64(spinnerTickInterval.Milliseconds())) % len(spinnerFrames)
	spinner := spinnerFrames[spinnerIdx]

	// Progress bar
	var progressBar string
	if a.ctrl.CustomPath() == "" {
		if drive := a.ctrl.SelectedDrive(); drive != nil && drive.UsedBytes() > 0 {
			progress := float64(state.BytesFound) / float64(drive.UsedBytes())
			if progress > 1.0 {
				progress = 1.0
			}
			maxDots := 20
			numDots := int(progress * float64(maxDots))
			emptyDots := maxDots - numDots
			dotStyle := lipgloss.NewStyle().Foreground(ColorCyan)
			emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3F3F46"))
			bracketStyle := lipgloss.NewStyle().Foreground(ColorCyan)
			progressBar = " " + bracketStyle.Render("[") + dotStyle.Render(strings.Repeat("·", numDots)) + emptyStyle.Render(strings.Repeat("·", emptyDots)) + bracketStyle.Render("]")
		}
	}

	// Phase display
	phases := []struct {
		phase core.ScanPhase
		name  string
	}{
		{core.PhaseScanning, "Scanning files"},
		{core.PhaseComputingSizes, "Computing sizes"},
		{core.PhaseComplete, "Complete"},
	}

	for _, p := range phases {
		if p.phase > state.Phase {
			break
		}
		var line string
		if p.phase < state.Phase || p.phase == core.PhaseComplete {
			check := doneStyle.Render("✓")
			text := doneStyle.Render(p.name)
			line = fmt.Sprintf("  %s %s", check, text)
		} else {
			spin := spinnerStyle.Render(spinner)
			textStyle := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
			text := textStyle.Render(p.name)
			line = fmt.Sprintf("  %s %s%s", spin, text, progressBar)
		}
		logLines = append(logLines, line)
	}

	// Stats
	if state.FilesScanned > 0 {
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
		fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFFF")).Bold(true)
		dataStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#C084FC")).Bold(true)
		timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Bold(true)

		logLines = append(logLines, "")
		logLines = append(logLines, fmt.Sprintf("    %s %s", labelStyle.Render("FILES"), fileStyle.Render(fmt.Sprintf("%d files", state.FilesScanned))))
		logLines = append(logLines, fmt.Sprintf("    %s  %s", labelStyle.Render("DATA"), dataStyle.Render(FormatSize(state.BytesFound))))
		logLines = append(logLines, fmt.Sprintf("    %s  %s", labelStyle.Render("TIME"), timeStyle.Render(state.Elapsed().String())))
	}

	logContent := strings.Join(logLines, "\n")
	innerContent := lipgloss.NewStyle().
		Padding(0, 3).
		Width(48).
		Render(logContent)

	boxHeight := 9
	scanningBox := renderSpinningBorder(
		lipgloss.Place(48, boxHeight-2, lipgloss.Left, lipgloss.Center, innerContent),
		50, boxHeight, time.Now())

	return lipgloss.Place(a.width, panelHeight, lipgloss.Center, lipgloss.Center, scanningBox)
}

// renderMainPanels renders the tree and treemap panels
func (a App) renderMainPanels() string {
	treeView := a.tree.View()
	infoBar := a.infoBar()

	var rightContent string
	selected := a.tree.Selected()
	if selected != nil && !selected.IsDir {
		rightContent = a.fileDetailsPanel()
	} else {
		rightContent = a.treemap.View()
	}

	rightPanel := lipgloss.JoinVertical(lipgloss.Left, infoBar, rightContent)
	return lipgloss.JoinHorizontal(lipgloss.Top, treeView, rightPanel)
}

// infoBar creates the info bar showing metadata
func (a App) infoBar() string {
	node := a.tree.Selected()
	if node == nil {
		return ""
	}

	borderColor := lipgloss.Color("#2D6A6A")
	if a.activePanel == PanelTreemap {
		borderColor = ColorCyan
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	content := " " + a.buildNodeInfo(node) + " "
	contentWidth := lipgloss.Width(content)
	topBorder := borderStyle.Render("╭" + strings.Repeat("─", contentWidth) + "╮")
	middleLine := borderStyle.Render("│") + content + borderStyle.Render("│")

	return topBorder + "\n" + middleLine
}

// buildNodeInfo creates the info string for a node
func (a App) buildNodeInfo(node *model.Node) string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	iconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))

	var icon string
	if node.IsDir {
		icon = iconStyle.Render("📁")
	} else {
		icon = iconStyle.Render("📄")
	}

	sep := dimStyle.Render(" │ ")
	name := nameStyle.Render(node.Name)

	var parts []string
	parts = append(parts, icon, " ", name)

	if node.IsDir {
		count := countFiles(node)
		parts = append(parts, sep, dimStyle.Render(fmt.Sprintf("%d files", count)))

		if info, err := os.Stat(node.Path); err == nil {
			createTime := getCreationTime(info)
			modTime := info.ModTime()

			if createTimeStr := FormatTime(createTime); createTimeStr != "" {
				parts = append(parts, sep, dimStyle.Render("C: "+createTimeStr))
			}

			modTimeStr := FormatTime(modTime)
			if modTimeStr != FormatTime(createTime) {
				parts = append(parts, sep, dimStyle.Render("M: "+modTimeStr))
			}
		}
	}

	return strings.Join(parts, "")
}

// fileDetailsPanel renders detailed file information
func (a App) fileDetailsPanel() string {
	node := a.tree.Selected()
	if node == nil || node.IsDir {
		return ""
	}

	panelHeight := a.height - 5
	panelWidth := a.rightPanelWidth - 2
	innerWidth := panelWidth - 2
	innerHeight := panelHeight - 2

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFFF"))

	var contentLines []string

	if fileType := getFileType(node.Path); fileType != "" {
		contentLines = append(contentLines, labelStyle.Render("Type: ")+valueStyle.Render(fileType))
	}

	contentLines = append(contentLines, labelStyle.Render("Size: ")+valueStyle.Render(FormatSize(node.TotalSize())))

	if info, err := os.Stat(node.Path); err == nil {
		if timeStr := FormatTime(getCreationTime(info)); timeStr != "" {
			contentLines = append(contentLines, labelStyle.Render("Created: ")+valueStyle.Render(timeStr))
		}
		contentLines = append(contentLines, labelStyle.Render("Modified: ")+valueStyle.Render(FormatTime(info.ModTime())))
		contentLines = append(contentLines, labelStyle.Render("Permissions: ")+valueStyle.Render(info.Mode().String()))
	}

	contentLines = append(contentLines, "")
	contentLines = append(contentLines, labelStyle.Render("Path:"))
	contentLines = append(contentLines, pathStyle.Render(node.Path))

	borderColor := lipgloss.Color("#2D6A6A")
	if a.activePanel == PanelTreemap {
		borderColor = ColorCyan
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	var result strings.Builder
	result.WriteString(borderStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮"))
	result.WriteString("\n")

	for i := 0; i < innerHeight; i++ {
		var line string
		if i < len(contentLines) {
			line = " " + contentLines[i]
		}
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth {
			line += strings.Repeat(" ", innerWidth-lineWidth)
		} else if lineWidth > innerWidth {
			line = line[:innerWidth]
		}
		result.WriteString(borderStyle.Render("│") + line + borderStyle.Render("│"))
		result.WriteString("\n")
	}

	result.WriteString(borderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯"))
	return result.String()
}

// getFileType detects file type using magic numbers
func getFileType(path string) string {
	mtype, err := mimetype.DetectFile(path)
	if err != nil {
		return ""
	}
	ext := mtype.Extension()
	if ext != "" {
		return strings.ToUpper(strings.TrimPrefix(ext, "."))
	}
	return ""
}

// countFiles counts all files in a node tree
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

// renderSpinningBorder draws a box with spinning gradient border
func renderSpinningBorder(content string, width, height int, t time.Time) string {
	shades := []string{
		"#00FFFF", "#30EBE0", "#5EEAD4", "#70E0D8", "#85D5E0", "#9AC5E8", "#A8B0F0", "#B89AF8",
		"#C084FC", "#C880F0", "#D080E8", "#D87CDE", "#E07CD4", "#F079CC", "#FF79C6", "#F079CC",
		"#E07CD4", "#D87CDE", "#D080E8", "#C880F0", "#C084FC", "#B89AF8", "#A8B0F0", "#9AC5E8",
		"#85D5E0", "#70E0D8", "#5EEAD4", "#30EBE0",
	}

	innerW := width - 2
	innerH := height - 2
	perimeter := 2*innerW + 2*innerH + 4

	offset := int(t.UnixMilli()/borderRotationSpeed) % perimeter

	getColor := func(pos int) lipgloss.Style {
		adjustedPos := (pos - offset + perimeter) % perimeter
		shadeIdx := (adjustedPos * len(shades) / perimeter) % len(shades)
		return lipgloss.NewStyle().Foreground(lipgloss.Color(shades[shadeIdx]))
	}

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

	result.WriteString(getColor(pos).Render(topLeft))
	pos++
	for i := 0; i < innerW; i++ {
		result.WriteString(getColor(pos).Render(horizontal))
		pos++
	}
	result.WriteString(getColor(pos).Render(topRight))
	pos++
	result.WriteString("\n")

	contentLines := strings.Split(content, "\n")
	for len(contentLines) < innerH {
		contentLines = append(contentLines, "")
	}

	for i := 0; i < innerH; i++ {
		leftColor := getColor(perimeter - 1 - i)
		result.WriteString(leftColor.Render(vertical))

		line := ""
		if i < len(contentLines) {
			line = contentLines[i]
		}
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerW {
			line += strings.Repeat(" ", innerW-lineWidth)
		}
		result.WriteString(line)

		result.WriteString(getColor(pos).Render(vertical))
		pos++
		result.WriteString("\n")
	}

	bottomStart := pos
	result.WriteString(getColor(perimeter - innerH - 1).Render(bottomLeft))
	for i := 0; i < innerW; i++ {
		result.WriteString(getColor(bottomStart + innerW - i).Render(horizontal))
	}
	result.WriteString(getColor(bottomStart).Render(bottomRight))

	return result.String()
}
