package model

import "sort"

// SortBySize sorts nodes by total size descending, then by name ascending
func SortBySize(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		si, sj := nodes[i].TotalSize(), nodes[j].TotalSize()
		if si != sj {
			return si > sj
		}
		return nodes[i].Name < nodes[j].Name
	})
}

