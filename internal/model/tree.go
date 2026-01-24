package model

import "sort"

// SortBySize sorts nodes by total size descending
func SortBySize(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].TotalSize() > nodes[j].TotalSize()
	})
}

// SortByChange sorts nodes by size change descending
func SortByChange(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].SizeChange() > nodes[j].SizeChange()
	})
}

// SortByName sorts nodes alphabetically
func SortByName(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})
}
