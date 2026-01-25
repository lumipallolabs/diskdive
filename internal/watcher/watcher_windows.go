//go:build windows

package watcher

import (
	"path/filepath"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
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

// Watcher watches for filesystem changes using Windows ReadDirectoryChangesW
type Watcher struct {
	handle  windows.Handle
	root    string
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
	w.root = root

	pathPtr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return err
	}

	handle, err := windows.CreateFile(
		pathPtr,
		windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OVERLAPPED,
		0,
	)
	if err != nil {
		return err
	}

	w.handle = handle
	return nil
}

func (w *Watcher) Start() {
	w.wg.Add(1)
	go w.run()
}

const notifyFilter = windows.FILE_NOTIFY_CHANGE_FILE_NAME | windows.FILE_NOTIFY_CHANGE_DIR_NAME

func (w *Watcher) run() {
	defer w.wg.Done()
	buf := make([]byte, 64*1024)

	for {
		select {
		case <-w.done:
			return
		default:
		}

		var bytesReturned uint32
		err := windows.ReadDirectoryChanges(
			w.handle,
			&buf[0],
			uint32(len(buf)),
			true, // recursive
			notifyFilter,
			&bytesReturned,
			nil,
			0,
		)
		if err != nil {
			return
		}

		if bytesReturned > 0 {
			w.processEvents(buf[:bytesReturned])
		}
	}
}

const (
	fileActionRemoved        = 2
	fileActionRenamedOldName = 4
)

func (w *Watcher) processEvents(buf []byte) {
	for len(buf) >= 12 {
		nextOffset := *(*uint32)(unsafe.Pointer(&buf[0]))
		action := *(*uint32)(unsafe.Pointer(&buf[4]))
		nameLen := *(*uint32)(unsafe.Pointer(&buf[8]))

		if len(buf) >= 12+int(nameLen) && (action == fileActionRemoved || action == fileActionRenamedOldName) {
			name := windows.UTF16ToString((*[1 << 15]uint16)(unsafe.Pointer(&buf[12]))[:nameLen/2])
			select {
			case w.eventCh <- Event{Type: EventDeleted, Path: filepath.Join(w.root, name)}:
			default:
			}
		}

		if nextOffset == 0 {
			break
		}
		buf = buf[nextOffset:]
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
	if w.handle != 0 {
		windows.CloseHandle(w.handle)
	}
	w.wg.Wait()
	close(w.eventCh)
	return nil
}
