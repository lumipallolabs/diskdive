package model

import "sort"

// SortBySize sorts nodes by total size descending
func SortBySize(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].TotalSize() > nodes[j].TotalSize()
	})
}

