package cache

import (
	"path/filepath"
	"testing"

	"github.com/samuli/diskdive/internal/model"
)

func TestSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	c := New(tmp)

	// Create test tree
	root := &model.Node{
		Path:  "C:\\",
		Name:  "C:",
		IsDir: true,
		Children: []*model.Node{
			{Path: "C:\\file.txt", Name: "file.txt", Size: 100},
		},
	}

	// Save
	err := c.Save("C", root)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	files, _ := filepath.Glob(filepath.Join(tmp, "C_*.json"))
	if len(files) == 0 {
		t.Fatal("no cache file created")
	}

	// Load
	loaded, err := c.LoadLatest("C")
	if err != nil {
		t.Fatalf("LoadLatest failed: %v", err)
	}

	if loaded.Name != root.Name {
		t.Errorf("expected name %s, got %s", root.Name, loaded.Name)
	}

	if len(loaded.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(loaded.Children))
	}
}

func TestLoadLatestNoCache(t *testing.T) {
	tmp := t.TempDir()
	c := New(tmp)

	_, err := c.LoadLatest("X")
	if err == nil {
		t.Error("expected error for missing cache")
	}
}
