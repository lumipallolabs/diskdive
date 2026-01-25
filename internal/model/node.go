package model

import "runtime"

// Node represents a file or directory in the scanned tree
type Node struct {
	Path     string  `json:"path"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"` // size in bytes (cached total for dirs, direct size for files)
	IsDir    bool    `json:"isDir"`
	Children []*Node `json:"children,omitempty"`
	Parent   *Node   `json:"-"` // skip to avoid circular reference

	// Change tracking (not persisted)
	PrevSize   int64 `json:"-"`
	IsNew      bool  `json:"-"`
	IsDeleted  bool  `json:"-"`
	HasGrew    bool  `json:"-"` // this node or descendant grew/is new
	HasShrunk  bool  `json:"-"` // this node or descendant shrunk/deleted
}

// TotalSize returns the cached total size (call ComputeSizes first)
func (n *Node) TotalSize() int64 {
	return n.Size
}

// ComputeSizes calculates and caches sizes for the entire tree
// Call this once after building/loading the tree
func (n *Node) ComputeSizes() int64 {
	var counter int64
	return n.computeSizesWithYield(&counter)
}

func (n *Node) computeSizesWithYield(counter *int64) int64 {
	*counter++
	if *counter%500 == 0 {
		runtime.Gosched()
	}
	if !n.IsDir {
		return n.Size
	}
	var total int64
	for _, child := range n.Children {
		total += child.computeSizesWithYield(counter)
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

// RebuildParentLinks restores Parent pointers after loading from cache
func (n *Node) RebuildParentLinks() {
	for _, child := range n.Children {
		child.Parent = n
		child.RebuildParentLinks()
	}
}

// CacheNode is a serializable version of Node (no Parent pointer)
type CacheNode struct {
	Path     string
	Name     string
	Size     int64
	IsDir    bool
	Children []*CacheNode
}

// ToCacheNode converts a Node tree to a CacheNode tree for serialization
func (n *Node) ToCacheNode() *CacheNode {
	cn := &CacheNode{
		Path:  n.Path,
		Name:  n.Name,
		Size:  n.Size,
		IsDir: n.IsDir,
	}
	for _, child := range n.Children {
		cn.Children = append(cn.Children, child.ToCacheNode())
	}
	return cn
}

// ToNode converts a CacheNode tree back to a Node tree
func (cn *CacheNode) ToNode(parent *Node) *Node {
	n := &Node{
		Path:   cn.Path,
		Name:   cn.Name,
		Size:   cn.Size,
		IsDir:  cn.IsDir,
		Parent: parent,
	}
	for _, child := range cn.Children {
		n.Children = append(n.Children, child.ToNode(n))
	}
	return n
}
