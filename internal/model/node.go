package model

// Node represents a file or directory in the scanned tree
type Node struct {
	Path     string
	Name     string
	Size     int64 // size in bytes (cached total for dirs, direct size for files)
	IsDir    bool
	Children []*Node
	Parent   *Node

	// Change tracking
	PrevSize  int64 // from cache, 0 if new
	IsNew     bool  // didn't exist in previous scan
	IsDeleted bool  // only appears in diff view
}

// TotalSize returns the cached total size (call ComputeSizes first)
func (n *Node) TotalSize() int64 {
	return n.Size
}

// ComputeSizes calculates and caches sizes for the entire tree
// Call this once after building/loading the tree
func (n *Node) ComputeSizes() int64 {
	if !n.IsDir {
		return n.Size
	}
	var total int64
	for _, child := range n.Children {
		total += child.ComputeSizes()
	}
	n.Size = total
	return total
}

// SizeChange returns the difference between current and previous size
func (n *Node) SizeChange() int64 {
	return n.TotalSize() - n.PrevSize
}

// ChangePercent returns the percentage change from previous size
func (n *Node) ChangePercent() float64 {
	if n.PrevSize == 0 {
		return 0
	}
	return float64(n.SizeChange()) / float64(n.PrevSize) * 100
}
