package core

import (
	"context"
	"sync"
	"time"

	"github.com/samuli/diskdive/internal/logging"
	"github.com/samuli/diskdive/internal/model"
	"github.com/samuli/diskdive/internal/scanner"
	"github.com/samuli/diskdive/internal/stats"
	"github.com/samuli/diskdive/internal/watcher"
)

// MinSignificantSize is the minimum size for a deletion to count in freed stats
const MinSignificantSize = 200 * 1024 // 200 KB

// Controller manages the core application logic without UI dependencies
type Controller struct {
	mu sync.RWMutex

	// State
	drives        []model.Drive
	selectedDrive int
	customPath    string
	root          *model.Node
	tree          *TreeState
	scan          ScanState
	freed         FreedState

	// Internal services
	scanner      scanner.Scanner
	watcher      *watcher.Watcher
	statsManager *stats.Manager

	// Event handling
	eventCh   chan Event
	listeners []func(Event)

	// Selection debouncing
	focusVersion int
}

// NewController creates a new application controller
func NewController(customPath string) *Controller {
	drives, _ := model.GetDrives()

	// Load stats
	statsMgr := stats.NewManager()
	if err := statsMgr.Load(); err != nil {
		logging.Debug.Printf("Failed to load stats: %v", err)
	}

	c := &Controller{
		drives:       drives,
		customPath:   customPath,
		tree:         NewTreeState(),
		scanner:      scanner.NewWalker(8),
		statsManager: statsMgr,
		eventCh:      make(chan Event, 100),
		freed: FreedState{
			Lifetime: statsMgr.FreedLifetime(),
		},
	}

	// Find saved default drive
	if customPath == "" {
		defaultDrive := statsMgr.DefaultDrive()
		for i, d := range drives {
			if d.Path == defaultDrive {
				c.selectedDrive = i
				break
			}
		}
	}

	return c
}

// State returns a read-only snapshot of the current state
func (c *Controller) State() AppState {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return AppState{
		Drives:        c.drives,
		SelectedDrive: c.selectedDrive,
		CustomPath:    c.customPath,
		Scan:          c.scan,
		Freed:         c.freed,
		Tree:          c.tree,
	}
}

// Drives returns the available drives
func (c *Controller) Drives() []model.Drive {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.drives
}

// SelectedDrive returns the currently selected drive
func (c *Controller) SelectedDrive() *model.Drive {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.selectedDrive < 0 || c.selectedDrive >= len(c.drives) {
		return nil
	}
	drive := c.drives[c.selectedDrive]
	return &drive
}

// SelectedDriveIndex returns the index of the selected drive
func (c *Controller) SelectedDriveIndex() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.selectedDrive
}

// HasSavedDefaultDrive returns true if there's a valid saved default drive
func (c *Controller) HasSavedDefaultDrive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.customPath != "" {
		return true // Custom path counts as having a target
	}

	defaultDrive := c.statsManager.DefaultDrive()
	for _, d := range c.drives {
		if d.Path == defaultDrive {
			return true
		}
	}
	return false
}

// CustomPath returns the custom scan path if set
func (c *Controller) CustomPath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.customPath
}

// Root returns the root node of the scanned tree
func (c *Controller) Root() *model.Node {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.root
}

// ScanState returns the current scan state
func (c *Controller) ScanState() ScanState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.scan
}

// FreedState returns the current freed space state
func (c *Controller) FreedState() FreedState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.freed
}

// IsShowingDiff returns whether diff mode is enabled
func (c *Controller) IsShowingDiff() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tree.ShowDiff
}

// SelectDrive selects a drive by index and prepares for scanning
func (c *Controller) SelectDrive(idx int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if idx < 0 || idx >= len(c.drives) {
		return nil
	}

	c.selectedDrive = idx
	c.freed.Session = 0
	c.root = nil
	c.tree = NewTreeState()

	// Save as default
	c.statsManager.SetDefaultDrive(c.drives[idx].Path)

	c.emit(DriveChangedEvent{
		Drive: &c.drives[idx],
		Index: idx,
	})

	return nil
}

// StartScan begins scanning the selected drive or custom path
func (c *Controller) StartScan(ctx context.Context) (<-chan Event, error) {
	c.mu.Lock()

	var scanPath string
	if c.customPath != "" {
		scanPath = c.customPath
	} else if c.selectedDrive >= 0 && c.selectedDrive < len(c.drives) {
		scanPath = c.drives[c.selectedDrive].Path
	}

	if scanPath == "" {
		c.mu.Unlock()
		return nil, nil
	}

	// Reset state for new scan
	c.scanner = scanner.NewWalker(8)
	c.scan = ScanState{
		Phase: PhaseScanning,
	}
	c.root = nil
	c.tree = NewTreeState()

	c.mu.Unlock()

	// Create event channel for this scan
	eventCh := make(chan Event, 100)

	go c.runScan(ctx, scanPath, eventCh)

	return eventCh, nil
}

// runScan executes the scan in a goroutine
func (c *Controller) runScan(ctx context.Context, path string, eventCh chan Event) {
	defer close(eventCh)

	logging.Debug.Printf("[Controller] Starting scan of %s", path)

	c.mu.Lock()
	c.scan.StartTime = time.Now()
	c.mu.Unlock()

	eventCh <- ScanStartedEvent{Path: path}

	// Listen for progress in separate goroutine
	go func() {
		for progress := range c.scanner.Progress() {
			c.mu.Lock()
			c.scan.FilesScanned = progress.FilesScanned
			c.scan.BytesFound = progress.BytesFound
			c.mu.Unlock()

			eventCh <- ScanProgressEvent{
				FilesScanned: progress.FilesScanned,
				BytesFound:   progress.BytesFound,
			}
		}
	}()

	// Run scan
	root, err := c.scanner.Scan(ctx, path)

	if err != nil {
		c.mu.Lock()
		c.scan.Phase = PhaseIdle
		c.mu.Unlock()

		eventCh <- ScanCompletedEvent{Err: err}
		eventCh <- ErrorEvent{Err: err}
		return
	}

	// Computing sizes phase
	c.mu.Lock()
	c.scan.Phase = PhaseComputingSizes
	c.mu.Unlock()

	eventCh <- ScanPhaseChangedEvent{Phase: PhaseComputingSizes}

	logging.Debug.Printf("[Controller] Computing sizes...")
	root.ComputeSizes()

	// Complete
	c.mu.Lock()
	c.scan.Phase = PhaseComplete
	c.root = root
	c.tree.Root = root
	c.tree.Expanded[root.Path] = true
	c.mu.Unlock()

	eventCh <- ScanPhaseChangedEvent{Phase: PhaseComplete}
	eventCh <- ScanCompletedEvent{Root: root}

	logging.Debug.Printf("[Controller] Scan complete")
}

// FinalizeScan marks the scan as fully complete (after UI delay)
func (c *Controller) FinalizeScan() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scan.Phase = PhaseIdle
}

// ToggleDiff toggles diff display mode
func (c *Controller) ToggleDiff() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tree.ShowDiff = !c.tree.ShowDiff
	return c.tree.ShowDiff
}

// StartWatching starts the filesystem watcher for the current scan root
func (c *Controller) StartWatching() (<-chan Event, error) {
	c.mu.Lock()

	var watchPath string
	if c.customPath != "" {
		watchPath = c.customPath
	} else if c.selectedDrive >= 0 && c.selectedDrive < len(c.drives) {
		watchPath = c.drives[c.selectedDrive].Path
	}

	if watchPath == "" || c.root == nil {
		c.mu.Unlock()
		return nil, nil
	}

	// Stop existing watcher
	if c.watcher != nil {
		_ = c.watcher.Stop()
	}

	// Create new watcher
	w, err := watcher.New()
	if err != nil {
		c.mu.Unlock()
		return nil, err
	}

	c.watcher = w
	root := c.root
	c.mu.Unlock()

	if err := w.AddRecursive(watchPath); err != nil {
		logging.Debug.Printf("Failed to add recursive watch: %v", err)
	}
	w.Start()
	logging.Debug.Printf("Filesystem watcher started for %s", watchPath)

	// Create event channel
	eventCh := make(chan Event, 100)

	go c.watchLoop(w, root, eventCh)

	return eventCh, nil
}

// watchLoop processes filesystem events
func (c *Controller) watchLoop(w *watcher.Watcher, root *model.Node, eventCh chan Event) {
	defer close(eventCh)

	for event := range w.Events() {
		if event.Type == watcher.EventDeleted {
			c.handleDeletion(event.Path, root, eventCh)
		}
	}
}

// handleDeletion processes a deletion event
func (c *Controller) handleDeletion(path string, root *model.Node, eventCh chan Event) {
	node := c.findNodeByPath(root, path)
	if node == nil {
		logging.Debug.Printf("Watcher: DELETE event for path not in tree: %s", path)
		return
	}

	if node.IsDeleted {
		return
	}

	size := node.TotalSize()
	node.MarkDeleted()
	logging.Debug.Printf("Watcher: MARKED DELETED: %s (size: %d)", path, size)

	if size >= MinSignificantSize {
		c.mu.Lock()
		c.freed.Session += size
		c.freed.Lifetime += size
		if c.statsManager != nil {
			c.statsManager.AddFreed(size)
		}
		freed := c.freed
		c.mu.Unlock()

		eventCh <- DeletionDetectedEvent{
			Path:         path,
			Size:         size,
			SessionFreed: freed.Session,
			TotalFreed:   freed.Lifetime,
		}

		logging.Debug.Printf("Watcher: freed %d bytes (session: %d, lifetime: %d)",
			size, freed.Session, freed.Lifetime)
	}
}

// findNodeByPath searches for a node by its path
func (c *Controller) findNodeByPath(node *model.Node, path string) *model.Node {
	if node.Path == path {
		return node
	}
	for _, child := range node.Children {
		if found := c.findNodeByPath(child, path); found != nil {
			return found
		}
	}
	return nil
}

// Stop cleans up resources
func (c *Controller) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.watcher != nil {
		_ = c.watcher.Stop()
	}
	if c.statsManager != nil {
		_ = c.statsManager.Close()
	}
}

// emit sends an event to all listeners
func (c *Controller) emit(event Event) {
	select {
	case c.eventCh <- event:
	default:
		// Channel full, drop event
	}
}
