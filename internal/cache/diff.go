package cache

import (
	"path/filepath"
	"runtime"

	"github.com/lumipallolabs/diskdive/internal/logging"
	"github.com/lumipallolabs/diskdive/internal/model"
)

// ApplyDiff compares current scan against previous and populates diff fields
func ApplyDiff(current, previous *model.Node) {
	var counter int64
	if previous == nil {
		markAllNew(current, &counter)
		propagateChanges(current, &counter)
		return
	}

	// Build lookup map of previous nodes by path
	prevMap := make(map[string]*model.Node)
	buildPathMap(previous, prevMap, &counter)

	// Build lookup map of current nodes by path
	currMap := make(map[string]*model.Node)
	buildPathMap(current, currMap, &counter)

	// Apply diff info to current tree
	applyDiffRecursive(current, prevMap, &counter)

	// Add deleted items from previous tree into current tree
	addDeletedItems(current, prevMap, currMap, &counter)

	// Verify deleted items were added by counting them
	deletedCount := countDeletedNodes(current, &counter)
	logging.Debug.Printf("[DIFF] ApplyDiff complete: found %d deleted nodes in final tree", deletedCount)

	// Propagate HasGrew/HasShrunk flags up the tree
	propagateChanges(current, &counter)
}

// countDeletedNodes recursively counts how many nodes have IsDeleted=true
func countDeletedNodes(node *model.Node, counter *int64) int {
	yieldIfNeeded(counter)
	count := 0
	if node.IsDeleted {
		count = 1
	}
	for _, child := range node.Children {
		count += countDeletedNodes(child, counter)
	}
	return count
}

func yieldIfNeeded(counter *int64) {
	*counter++
	if *counter%200 == 0 {
		runtime.Gosched()
	}
}

func buildPathMap(node *model.Node, m map[string]*model.Node, counter *int64) {
	yieldIfNeeded(counter)
	m[node.Path] = node
	for _, child := range node.Children {
		buildPathMap(child, m, counter)
	}
}

func applyDiffRecursive(node *model.Node, prevMap map[string]*model.Node, counter *int64) {
	yieldIfNeeded(counter)
	prev, exists := prevMap[node.Path]
	if exists {
		node.PrevSize = prev.TotalSize()
		node.IsNew = false
	} else {
		node.IsNew = true
		node.PrevSize = 0
	}

	for _, child := range node.Children {
		applyDiffRecursive(child, prevMap, counter)
	}
}

// addDeletedItems adds nodes from previous tree that don't exist in current tree
func addDeletedItems(current *model.Node, prevMap, currMap map[string]*model.Node, counter *int64) {
	yieldIfNeeded(counter)
	var deletedCount int
	// For each node in previous tree, check if it exists in current
	for prevPath, prevNode := range prevMap {
		if _, exists := currMap[prevPath]; !exists {
			// This node was deleted - add it to current tree
			parentPath := getParentPath(prevPath)
			if parentPath == "" {
				// This is root - shouldn't happen
				continue
			}

			// Find parent in current tree
			parent, parentExists := currMap[parentPath]
			if !parentExists {
				// Parent was also deleted, will be handled by its own entry
				continue
			}

			// Create a copy of the deleted node
			deletedNode := &model.Node{
				Path:      prevNode.Path,
				Name:      prevNode.Name,
				Size:      prevNode.TotalSize(),
				IsDir:     prevNode.IsDir,
				Parent:    parent,
				Children:  nil, // Don't include children of deleted items
				PrevSize:  prevNode.TotalSize(),
				IsDeleted: true,
				IsNew:     false,
			}

			// Add to parent's children
			parent.Children = append(parent.Children, deletedNode)

			// Add to current map so we can find it as a parent for its children
			currMap[prevPath] = deletedNode
			deletedCount++

			// Debug: log first few deletions
			if deletedCount <= 10 {
				logging.Debug.Printf("Added deleted item: %s (size=%d) to parent: %s", prevNode.Name, prevNode.TotalSize(), parent.Name)
			}
		}
	}
	if deletedCount > 0 {
		logging.Debug.Printf("Total deleted items added to tree: %d", deletedCount)
	}
}

// getParentPath returns the parent directory path
func getParentPath(path string) string {
	if path == "" || path == "/" {
		return ""
	}
	parent := filepath.Dir(path)
	if parent == "." {
		return ""
	}
	return parent
}

func markAllNew(node *model.Node, counter *int64) {
	yieldIfNeeded(counter)
	node.IsNew = true
	for _, child := range node.Children {
		markAllNew(child, counter)
	}
}

// propagateChanges sets HasGrew/HasShrunk on nodes based on their own state
// and their descendants' states. Returns (hasGrew, hasShrunk).
func propagateChanges(node *model.Node, counter *int64) (bool, bool) {
	yieldIfNeeded(counter)
	// Check if this node itself grew or shrunk
	ownGrew := node.IsNew || node.SizeChange() > 0
	ownShrunk := node.IsDeleted || node.SizeChange() < 0

	// Aggregate from children
	childGrew := false
	childShrunk := false
	for _, child := range node.Children {
		g, s := propagateChanges(child, counter)
		if g {
			childGrew = true
		}
		if s {
			childShrunk = true
		}
	}

	node.HasGrew = ownGrew || childGrew
	node.HasShrunk = ownShrunk || childShrunk
	return node.HasGrew, node.HasShrunk
}
