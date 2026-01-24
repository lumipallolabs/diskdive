package cache

import (
	"testing"

	"github.com/samuli/diskdive/internal/model"
)

func TestApplyDiff(t *testing.T) {
	// Previous scan
	prev := &model.Node{
		Path:  "C:\\",
		Name:  "C:",
		IsDir: true,
		Children: []*model.Node{
			{Path: "C:\\old", Name: "old", Size: 100},
			{Path: "C:\\same", Name: "same", Size: 200},
		},
	}

	// Current scan
	curr := &model.Node{
		Path:  "C:\\",
		Name:  "C:",
		IsDir: true,
		Children: []*model.Node{
			{Path: "C:\\same", Name: "same", Size: 250}, // grew
			{Path: "C:\\new", Name: "new", Size: 300},   // new
		},
	}

	ApplyDiff(curr, prev)

	// Check same folder has previous size
	var same *model.Node
	for _, c := range curr.Children {
		if c.Name == "same" {
			same = c
			break
		}
	}

	if same == nil {
		t.Fatal("same folder not found")
	}

	if same.PrevSize != 200 {
		t.Errorf("expected PrevSize 200, got %d", same.PrevSize)
	}

	// Check new folder is marked
	var newNode *model.Node
	for _, c := range curr.Children {
		if c.Name == "new" {
			newNode = c
			break
		}
	}

	if newNode == nil {
		t.Fatal("new folder not found")
	}

	if !newNode.IsNew {
		t.Error("expected new folder to be marked IsNew")
	}
}
