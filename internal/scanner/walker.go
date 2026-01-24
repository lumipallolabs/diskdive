package scanner

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/samuli/diskdive/internal/model"
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

// Scan scans the filesystem starting at root
func (w *Walker) Scan(ctx context.Context, root string) (*model.Node, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, err
	}

	rootNode := &model.Node{
		Path:  absRoot,
		Name:  filepath.Base(absRoot),
		IsDir: info.IsDir(),
	}

	if !info.IsDir() {
		rootNode.Size = info.Size()
		return rootNode, nil
	}

	// Build tree recursively with parallelism
	w.scanDir(ctx, rootNode)

	close(w.progressCh)
	return rootNode, nil
}

func (w *Walker) scanDir(ctx context.Context, node *model.Node) {
	entries, err := os.ReadDir(node.Path)
	if err != nil {
		return // Skip directories we can't read
	}

	// Use semaphore for parallelism control
	sem := make(chan struct{}, w.workers)
	var wg sync.WaitGroup

	var children []*model.Node
	var childMu sync.Mutex

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return
		default:
		}

		childPath := filepath.Join(node.Path, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		child := &model.Node{
			Path:   childPath,
			Name:   entry.Name(),
			IsDir:  entry.IsDir(),
			Parent: node,
		}

		if !entry.IsDir() {
			child.Size = info.Size()
			atomic.AddInt64(&w.progress.FilesScanned, 1)
			atomic.AddInt64(&w.progress.BytesFound, info.Size())
		} else {
			atomic.AddInt64(&w.progress.DirsScanned, 1)

			wg.Add(1)
			sem <- struct{}{}
			go func(c *model.Node) {
				defer wg.Done()
				defer func() { <-sem }()
				w.scanDir(ctx, c)
			}(child)
		}

		childMu.Lock()
		children = append(children, child)
		childMu.Unlock()
	}

	wg.Wait()
	node.Children = children
}

// Ensure Walker implements Scanner
var _ Scanner = (*Walker)(nil)
