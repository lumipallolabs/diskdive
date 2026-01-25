package cache

import (
	"runtime"

	"github.com/samuli/diskdive/internal/model"
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

	// Apply diff info to current tree
	applyDiffRecursive(current, prevMap, &counter)

	// Propagate HasGrew/HasShrunk flags up the tree
	propagateChanges(current, &counter)
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
