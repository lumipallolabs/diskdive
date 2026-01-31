package core

import (
	"time"

	"github.com/samuli/diskdive/internal/model"
)

// ScanPhase represents the current phase of scanning
type ScanPhase int

const (
	PhaseIdle ScanPhase = iota
	PhaseScanning
	PhaseComputingSizes
	PhaseComplete
)

// String returns a human-readable phase name
func (p ScanPhase) String() string {
	switch p {
	case PhaseIdle:
		return ""
	case PhaseScanning:
		return "Scanning files"
	case PhaseComputingSizes:
		return "Computing sizes"
	case PhaseComplete:
		return "Complete"
	default:
		return ""
	}
}

// ScanState holds the current scan state
type ScanState struct {
	Phase        ScanPhase
	StartTime    time.Time
	FilesScanned int64
	BytesFound   int64
}

// IsScanning returns true if a scan is in progress (including the brief "Complete" display)
func (s ScanState) IsScanning() bool {
	return s.Phase == PhaseScanning || s.Phase == PhaseComputingSizes || s.Phase == PhaseComplete
}

// Elapsed returns time since scan started
func (s ScanState) Elapsed() time.Duration {
	if s.StartTime.IsZero() {
		return 0
	}
	return time.Since(s.StartTime).Truncate(time.Second)
}

// FreedState tracks space recovered from deletions
type FreedState struct {
	Session  int64 // Bytes freed this session
	Lifetime int64 // Bytes freed all time
}

// TreeState holds tree navigation state
type TreeState struct {
	Root     *model.Node
	Selected *model.Node
	Expanded map[string]bool // Path -> expanded
}

// NewTreeState creates a new tree state
func NewTreeState() *TreeState {
	return &TreeState{
		Expanded: make(map[string]bool),
	}
}

// AppState holds the complete application state (read-only view)
type AppState struct {
	Drives        []model.Drive
	SelectedDrive int
	CustomPath    string // If scanning a custom path instead of drive
	Scan          ScanState
	Freed         FreedState
	Tree          *TreeState
	Error         error
}
