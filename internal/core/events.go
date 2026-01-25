package core

import "github.com/samuli/diskdive/internal/model"

// Event represents a state change from the controller
type Event interface {
	isEvent()
}

// ScanStartedEvent is emitted when a scan begins
type ScanStartedEvent struct {
	Path string
}

func (ScanStartedEvent) isEvent() {}

// ScanProgressEvent is emitted during scanning
type ScanProgressEvent struct {
	FilesScanned int64
	BytesFound   int64
}

func (ScanProgressEvent) isEvent() {}

// ScanPhaseChangedEvent is emitted when scan phase changes
type ScanPhaseChangedEvent struct {
	Phase ScanPhase
}

func (ScanPhaseChangedEvent) isEvent() {}

// ScanCompletedEvent is emitted when scan finishes
type ScanCompletedEvent struct {
	Root *model.Node
	Err  error
}

func (ScanCompletedEvent) isEvent() {}

// SelectionChangedEvent is emitted when selection changes
type SelectionChangedEvent struct {
	Node    *model.Node
	Source  SelectionSource
	Version int // For debouncing
}

func (SelectionChangedEvent) isEvent() {}

// SelectionSource indicates what triggered the selection change
type SelectionSource int

const (
	SelectionFromTree SelectionSource = iota
	SelectionFromTreemap
)

// DriveChangedEvent is emitted when the active drive changes
type DriveChangedEvent struct {
	Drive *model.Drive
	Index int
}

func (DriveChangedEvent) isEvent() {}

// DeletionDetectedEvent is emitted when a file/folder deletion is detected
type DeletionDetectedEvent struct {
	Path         string
	Size         int64
	SessionFreed int64
	TotalFreed   int64
}

func (DeletionDetectedEvent) isEvent() {}

// TreeExpandedEvent is emitted when a tree node is expanded/collapsed
type TreeExpandedEvent struct {
	Node     *model.Node
	Expanded bool
}

func (TreeExpandedEvent) isEvent() {}

// ErrorEvent is emitted when an error occurs
type ErrorEvent struct {
	Err error
}

func (ErrorEvent) isEvent() {}
