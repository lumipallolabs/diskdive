package scanner

import (
	"context"
	"io/fs"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charlievieth/fastwalk"
	"github.com/lumipallolabs/diskdive/internal/model"
)

// Walker implements parallel filesystem scanning
type Walker struct {
	workers    int
	progressCh chan Progress
	progress   Progress
	mu         sync.Mutex
}

// NewWalker creates a new parallel filesystem walker
func NewWalker(workers int) *Walker {
	if workers < 1 {
		workers = 8
	}
	return &Walker{
		workers:    workers,
		progressCh: make(chan Progress, 100),
	}
}

// Progress returns the progress channel
func (w *Walker) Progress() <-chan Progress {
	return w.progressCh
}

// nodeEntry is a temporary structure for building the tree
type nodeEntry struct {
	path  string
	name  string
	size  int64
	isDir bool
}

// Scan scans the filesystem starting at root using fastwalk
func (w *Walker) Scan(ctx context.Context, root string) (*model.Node, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	// Get platform-specific root info for mount point detection
	rootInfo := getPlatformRootInfo(absRoot)

	// Collect entries with mutex - simpler and faster than channel for high throughput
	// Start with 100k capacity - Go will grow as needed, avoids large upfront allocation on low-RAM machines
	entries := make([]nodeEntry, 0, 100000)
	var entriesMu sync.Mutex

	// Track seen paths/inodes for deduplication
	var seenItems sync.Map

	// Configure fastwalk
	conf := &fastwalk.Config{
		Follow:     false, // Don't follow symlinks
		NumWorkers: w.workers,
	}

	// Start progress reporter goroutine
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				// Send current progress (non-blocking)
				select {
				case w.progressCh <- Progress{
					FilesScanned: atomic.LoadInt64(&w.progress.FilesScanned),
					DirsScanned:  atomic.LoadInt64(&w.progress.DirsScanned),
					BytesFound:   atomic.LoadInt64(&w.progress.BytesFound),
				}:
				default:
				}
			}
		}
	}()

	// Walk filesystem with fastwalk
	walkErr := fastwalk.Walk(conf, absRoot, func(path string, d fs.DirEntry, err error) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return nil // Skip entries with errors
		}

		// Skip the root itself
		if path == absRoot {
			return nil
		}

		// Platform-specific directory checks (mount points, firmlinks)
		if d.IsDir() {
			if shouldSkipDir(path, d, rootInfo, &seenItems) {
				return fs.SkipDir
			}
		}

		var size int64
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return nil
			}

			// Get file size (platform-specific for accurate disk usage)
			size = getFileSize(info, &seenItems)
			if size < 0 {
				// Negative means skip (e.g., already counted hard link)
				return nil
			}

			atomic.AddInt64(&w.progress.FilesScanned, 1)
			atomic.AddInt64(&w.progress.BytesFound, size)
		} else {
			atomic.AddInt64(&w.progress.DirsScanned, 1)
		}

		// Append to entries slice
		entriesMu.Lock()
		entries = append(entries, nodeEntry{
			path:  path,
			name:  d.Name(),
			size:  size,
			isDir: d.IsDir(),
		})
		entriesMu.Unlock()

		return nil
	})

	// Stop progress reporter
	close(done)

	if walkErr != nil && walkErr != ctx.Err() {
		close(w.progressCh)
		return nil, walkErr
	}

	// Build the tree structure from flat entries
	rootNode := w.buildTree(absRoot, entries)

	close(w.progressCh)
	return rootNode, nil
}

// buildTree constructs the tree structure from flat entries
func (w *Walker) buildTree(rootPath string, entries []nodeEntry) *model.Node {
	// Map to hold all nodes
	nodes := make(map[string]*model.Node, len(entries)+1)
	// Map to count children per directory (for pre-allocation)
	childCounts := make(map[string]int, len(entries)/10)

	// Create root node
	rootNode := &model.Node{
		Path:  rootPath,
		Name:  filepath.Base(rootPath),
		IsDir: true,
	}
	nodes[rootPath] = rootNode

	// First pass: count children per parent and create nodes
	for i := range entries {
		e := &entries[i]

		// Count children for parent
		parentPath := filepath.Dir(e.path)
		childCounts[parentPath]++

		// Create node
		nodes[e.path] = &model.Node{
			Path:  e.path,
			Name:  e.name,
			Size:  e.size,
			IsDir: e.isDir,
		}
	}

	// Pre-allocate Children slices
	for path, count := range childCounts {
		if node, exists := nodes[path]; exists {
			node.Children = make([]*model.Node, 0, count)
		}
	}

	// Second pass: link parent/child relationships
	for i := range entries {
		e := &entries[i]
		node := nodes[e.path]
		parentPath := filepath.Dir(e.path)
		if parent, exists := nodes[parentPath]; exists {
			node.Parent = parent
			parent.Children = append(parent.Children, node)
		}
	}

	return rootNode
}

// Ensure Walker implements Scanner
var _ Scanner = (*Walker)(nil)
