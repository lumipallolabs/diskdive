package scanner

import (
	"context"

	"github.com/samuli/diskdive/internal/model"
)

// Progress reports scanning progress
type Progress struct {
	FilesScanned int64
	DirsScanned  int64
	BytesFound   int64
	CurrentPath  string
}

// Scanner defines the interface for filesystem scanning
type Scanner interface {
	// Scan scans the given root path and returns a tree of nodes
	Scan(ctx context.Context, root string) (*model.Node, error)

	// Progress returns a channel that receives progress updates
	Progress() <-chan Progress
}
