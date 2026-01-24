package model

import "testing"

func TestNodeSize(t *testing.T) {
	child1 := &Node{Name: "file1.txt", Size: 100, IsDir: false}
	child2 := &Node{Name: "file2.txt", Size: 200, IsDir: false}
	parent := &Node{
		Name:     "folder",
		IsDir:    true,
		Children: []*Node{child1, child2},
	}

	// Must call ComputeSizes to cache totals
	parent.ComputeSizes()

	if parent.TotalSize() != 300 {
		t.Errorf("expected 300, got %d", parent.TotalSize())
	}
}

func TestNodeSizeChange(t *testing.T) {
	node := &Node{Name: "folder", Size: 0, PrevSize: 100, IsDir: true}
	node.Size = 150

	change := node.SizeChange()
	if change != 50 {
		t.Errorf("expected change of 50, got %d", change)
	}

	pct := node.ChangePercent()
	if pct != 50.0 {
		t.Errorf("expected 50%%, got %.1f%%", pct)
	}
}

func TestSortBySize(t *testing.T) {
	nodes := []*Node{
		{Name: "small", Size: 100},
		{Name: "large", Size: 1000},
		{Name: "medium", Size: 500},
	}

	SortBySize(nodes)

	if nodes[0].Name != "large" {
		t.Errorf("expected 'large' first, got %s", nodes[0].Name)
	}
	if nodes[2].Name != "small" {
		t.Errorf("expected 'small' last, got %s", nodes[2].Name)
	}
}

func TestSortByChange(t *testing.T) {
	nodes := []*Node{
		{Name: "shrunk", Size: 50, PrevSize: 100},
		{Name: "grew", Size: 200, PrevSize: 100},
		{Name: "same", Size: 100, PrevSize: 100},
	}

	SortByChange(nodes)

	if nodes[0].Name != "grew" {
		t.Errorf("expected 'grew' first, got %s", nodes[0].Name)
	}
	if nodes[2].Name != "shrunk" {
		t.Errorf("expected 'shrunk' last, got %s", nodes[2].Name)
	}
}
