package cache

import "github.com/samuli/diskdive/internal/model"

// ApplyDiff compares current scan against previous and populates diff fields
func ApplyDiff(current, previous *model.Node) {
	if previous == nil {
		markAllNew(current)
		return
	}

	// Build lookup map of previous nodes by path
	prevMap := make(map[string]*model.Node)
	buildPathMap(previous, prevMap)

	// Apply diff info to current tree
	applyDiffRecursive(current, prevMap)
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
