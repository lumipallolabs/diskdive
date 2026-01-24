package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWalkerScan(t *testing.T) {
	// Create temp directory structure
	tmp := t.TempDir()

	// Create test files
	os.MkdirAll(filepath.Join(tmp, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmp, "file1.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(tmp, "subdir", "file2.txt"), []byte("world!"), 0644)

	// Scan
	w := NewWalker(4)
	root, err := w.Scan(context.Background(), tmp)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	// Verify root
	if !root.IsDir {
		t.Error("root should be a directory")
	}

	// Verify total size (5 + 6 = 11 bytes)
	if root.TotalSize() != 11 {
		t.Errorf("expected total size 11, got %d", root.TotalSize())
	}

	// Verify children count
	if len(root.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(root.Children))
	}
}
