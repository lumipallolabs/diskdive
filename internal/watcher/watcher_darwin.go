//go:build darwin

package watcher

import (
	"sync"
	"time"

	"github.com/fsnotify/fsevents"
)

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

// Watcher watches for filesystem changes using macOS FSEvents
type Watcher struct {
	stream  *fsevents.EventStream
	eventCh chan Event
	done    chan struct{}
	wg      sync.WaitGroup
	mu      sync.Mutex
	closed  bool
}

func New() (*Watcher, error) {
	return &Watcher{
		eventCh: make(chan Event, 100),
		done:    make(chan struct{}),
	}, nil
}

func (w *Watcher) Events() <-chan Event {
	return w.eventCh
}

func (w *Watcher) AddRecursive(root string) error {
	dev, err := fsevents.DeviceForPath(root)
	if err != nil {
		return err
	}

	w.stream = &fsevents.EventStream{
		Paths:   []string{root},
		Latency: 500 * time.Millisecond,
		Device:  dev,
		Flags:   fsevents.FileEvents | fsevents.WatchRoot,
	}
	return nil
}

func (w *Watcher) Start() {
	if w.stream == nil {
		return
	}
	w.stream.Start()
	w.wg.Add(1)
	go w.run()
}

func (w *Watcher) run() {
	defer w.wg.Done()

	for {
		select {
		case <-w.done:
			return
		case events, ok := <-w.stream.Events:
			if !ok {
				return
			}
			for _, event := range events {
				w.handleEvent(event)
			}
		}
	}
}

func (w *Watcher) handleEvent(event fsevents.Event) {
	// Only care about deletions and renames (move to Trash is a rename)
	if event.Flags&fsevents.ItemRemoved == 0 && event.Flags&fsevents.ItemRenamed == 0 {
		return
	}

	path := event.Path
	if len(path) > 0 && path[0] != '/' {
		path = "/" + path
	}

	select {
	case w.eventCh <- Event{Type: EventDeleted, Path: path}:
	default:
	}
}

func (w *Watcher) Stop() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()

	close(w.done)
	if w.stream != nil {
		w.stream.Stop()
	}
	w.wg.Wait()
	close(w.eventCh)
	return nil
}
