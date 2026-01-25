//go:build !darwin && !windows

package watcher

// EventType represents the type of filesystem event
type EventType int

const (
	EventDeleted EventType = iota
	EventCreated
	EventModified
)

// Event represents a filesystem change event
type Event struct {
	Type EventType
	Path string
}

// Watcher is a stub for non-macOS platforms
// TODO: Implement using inotify on Linux, ReadDirectoryChangesW on Windows
type Watcher struct {
	eventCh chan Event
}

// New creates a new filesystem watcher (stub)
func New() (*Watcher, error) {
	return &Watcher{
		eventCh: make(chan Event, 100),
	}, nil
}

// Events returns the channel for receiving filesystem events
func (w *Watcher) Events() <-chan Event {
	return w.eventCh
}

// AddRecursive adds a path to watch recursively (stub - does nothing)
func (w *Watcher) AddRecursive(root string) error {
	return nil
}

// Start begins watching for events (stub - does nothing)
func (w *Watcher) Start() {
}

// Stop stops the watcher (stub)
func (w *Watcher) Stop() error {
	close(w.eventCh)
	return nil
}
