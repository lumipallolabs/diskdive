package cache

import "github.com/samuli/diskdive/internal/model"

// ApplyDiff compares current scan against previous and populates diff fields
func ApplyDiff(current, previous *model.Node) {
	if previous == nil {
		markAllNew(current)
		propagateChanges(current)
		return
	}

	// Build lookup map of previous nodes by path
	prevMap := make(map[string]*model.Node)
	buildPathMap(previous, prevMap)

	// Apply diff info to current tree
	applyDiffRecursive(current, prevMap)

	// Propagate HasGrew/HasShrunk flags up the tree
	propagateChanges(current)
}

func buildPathMap(node *model.Node, m map[string]*model.Node) {
	m[node.Path] = node
	for _, child := range node.Children {
		buildPathMap(child, m)
	}
}

func applyDiffRecursive(node *model.Node, prevMap map[string]*model.Node) {
	prev, exists := prevMap[node.Path]
	if exists {
		node.PrevSize = prev.TotalSize()
		node.IsNew = false
	} else {
		node.IsNew = true
		node.PrevSize = 0
	}

	for _, child := range node.Children {
		applyDiffRecursive(child, prevMap)
	}
}

func markAllNew(node *model.Node) {
	node.IsNew = true
	for _, child := range node.Children {
		markAllNew(child)
	}
}

// propagateChanges sets HasGrew/HasShrunk on nodes based on their own state
// and their descendants' states. Returns (hasGrew, hasShrunk).
func propagateChanges(node *model.Node) (bool, bool) {
	// Check if this node itself grew or shrunk
	ownGrew := node.IsNew || node.SizeChange() > 0
	ownShrunk := node.IsDeleted || node.SizeChange() < 0

	// Aggregate from children
	childGrew := false
	childShrunk := false
	for _, child := range node.Children {
		g, s := propagateChanges(child)
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
