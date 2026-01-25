package ui

import (
	"fmt"
	"testing"

	"github.com/jeffwilliams/squarify"
	"github.com/samuli/diskdive/internal/model"
)

func TestSquarifyDirect(t *testing.T) {
	// Create simple tree structure for squarify
	root := &treemapItem{
		size: 300,
		children: []*treemapItem{
			{size: 100},
			{size: 100},
			{size: 100},
		},
	}

	rect := squarify.Rect{X: 0, Y: 0, W: 76, H: 22}

	blocks, metas := squarify.Squarify(root, rect, squarify.Options{
		MaxDepth: 1,
		Sort:     true,
	})

	t.Logf("Squarify returned %d blocks", len(blocks))
	for i, b := range blocks {
		depth := -1
		if i < len(metas) {
			depth = metas[i].Depth
		}
		t.Logf("  Block[%d] depth=%d X=%.1f Y=%.1f W=%.1f H=%.1f",
			i, depth, b.X, b.Y, b.W, b.H)
	}

	// Count depth-0 blocks (squarify returns children at depth 0, not depth 1)
	depth0Count := 0
	for i := range blocks {
		if i < len(metas) && metas[i].Depth == 0 {
			depth0Count++
		}
	}
	t.Logf("Depth-0 blocks: %d", depth0Count)

	if depth0Count != 3 {
		t.Errorf("Expected 3 depth-0 blocks, got %d", depth0Count)
	}
}

func TestTreemapLayout(t *testing.T) {
	// Create test nodes with varying sizes
	root := &model.Node{
		Name:  "root",
		IsDir: true,
		Size:  0,
	}

	// Add children with different sizes (simulating real disk usage)
	children := []*model.Node{
		{Name: "big1", Size: 100 * 1024 * 1024, IsDir: true, Parent: root},   // 100MB
		{Name: "big2", Size: 80 * 1024 * 1024, IsDir: true, Parent: root},    // 80MB
		{Name: "medium1", Size: 50 * 1024 * 1024, IsDir: true, Parent: root}, // 50MB
		{Name: "medium2", Size: 30 * 1024 * 1024, IsDir: true, Parent: root}, // 30MB
		{Name: "small1", Size: 10 * 1024 * 1024, IsDir: true, Parent: root},  // 10MB
		{Name: "small2", Size: 5 * 1024 * 1024, IsDir: true, Parent: root},   // 5MB
		{Name: "tiny1", Size: 1 * 1024 * 1024, IsDir: true, Parent: root},    // 1MB
		{Name: "tiny2", Size: 500 * 1024, IsDir: true, Parent: root},         // 500KB
	}
	root.Children = children

	t.Logf("Root: %s, IsDir=%v, NumChildren=%d", root.Name, root.IsDir, len(root.Children))
	for i, c := range root.Children {
		t.Logf("  Child[%d]: %s, Size=%d, TotalSize=%d", i, c.Name, c.Size, c.TotalSize())
	}

	// Create panel and set up
	panel := NewTreemapPanel()
	panel.SetSize(80, 24) // Typical terminal size

	t.Logf("Panel size: %dx%d", panel.width, panel.height)

	panel.SetRoot(root)

	t.Logf("After SetRoot: focus=%v, root=%v, blocks=%d",
		panel.focus != nil, panel.root != nil, len(panel.blocks))

	// Check that blocks are generated
	if len(panel.blocks) == 0 {
		t.Fatal("Expected blocks to be generated")
	}

	t.Logf("Generated %d blocks", len(panel.blocks))

	// Check all blocks are within bounds
	// Must match layout(): contentW = width - treemapBorderH - treemapPadding (2 + 0)
	//                      contentH = height - treemapBorderV (0)
	contentW := panel.width - 2
	contentH := panel.height

	for i, block := range panel.blocks {
		name := "grouped"
		if block.Node != nil {
			name = block.Node.Name
		}

		t.Logf("Block[%d] %s: x=%d y=%d w=%d h=%d (end: x=%d y=%d)",
			i, name, block.X, block.Y, block.Width, block.Height,
			block.X+block.Width, block.Y+block.Height)

		// Validate bounds
		if block.X < 0 {
			t.Errorf("Block[%d] %s: X is negative: %d", i, name, block.X)
		}
		if block.Y < 0 {
			t.Errorf("Block[%d] %s: Y is negative: %d", i, name, block.Y)
		}
		if block.X+block.Width > contentW {
			t.Errorf("Block[%d] %s: exceeds width bounds: x=%d w=%d contentW=%d",
				i, name, block.X, block.Width, contentW)
		}
		if block.Y+block.Height > contentH {
			t.Errorf("Block[%d] %s: exceeds height bounds: y=%d h=%d contentH=%d",
				i, name, block.Y, block.Height, contentH)
		}
	}
}

func TestMaxVisibleAnalysis(t *testing.T) {
	// Directly test squarify with various maxVisible values
	root := &model.Node{Name: "C:", IsDir: true}
	children := []*model.Node{
		{Name: "Program Files (x86)", Size: 546700000000, IsDir: true, Parent: root},
		{Name: "Users", Size: 501700000000, IsDir: true, Parent: root},
		{Name: "XboxGames", Size: 283300000000, IsDir: true, Parent: root},
		{Name: "Program Files", Size: 180200000000, IsDir: true, Parent: root},
		{Name: "dev", Size: 138700000000, IsDir: true, Parent: root},
		{Name: "Windows", Size: 38200000000, IsDir: true, Parent: root},
		{Name: "ProgramData", Size: 13500000000, IsDir: true, Parent: root},
		{Name: "hiberfil.sys", Size: 12400000000, IsDir: false, Parent: root},
		{Name: "pagefile.sys", Size: 9000000000, IsDir: false, Parent: root},
		{Name: "$Recycle.Bin", Size: 3100000000, IsDir: true, Parent: root},
		{Name: "msys64", Size: 326600000, IsDir: true, Parent: root},
	}
	root.Children = children

	contentW, contentH := 86, 48

	for maxVisible := 5; maxVisible <= 11; maxVisible++ {
		// Build items
		items := make([]*treemapItem, 0, len(children))
		for _, c := range children {
			items = append(items, &treemapItem{node: c, size: float64(c.Size)})
		}

		var displayItems []*treemapItem
		if len(items) <= maxVisible {
			displayItems = items
		} else {
			displayItems = make([]*treemapItem, 0, maxVisible)
			for i := 0; i < maxVisible-1; i++ {
				displayItems = append(displayItems, items[i])
			}
			var groupSize int64
			for i := maxVisible - 1; i < len(items); i++ {
				groupSize += int64(items[i].size)
			}
			displayItems = append(displayItems, &treemapItem{
				isGrouped:  true,
				groupCount: len(items) - (maxVisible - 1),
				groupSize:  groupSize,
				size:       float64(groupSize),
			})
		}

		tmRoot := &treemapItem{children: displayItems}
		for _, c := range displayItems {
			tmRoot.size += c.size
		}

		rect := squarify.Rect{X: 0, Y: 0, W: float64(contentW), H: float64(contentH)}
		blocks, metas := squarify.Squarify(tmRoot, rect, squarify.Options{MaxDepth: 1, Sort: true})

		t.Logf("maxVisible=%d:", maxVisible)
		allFit := true
		for i, b := range blocks {
			if i >= len(metas) || metas[i].Depth != 0 {
				continue
			}
			w := int(b.X+b.W) - int(b.X)
			h := int(b.Y+b.H) - int(b.Y)
			item := b.TreeSizer.(*treemapItem)
			name := "grouped"
			if item.node != nil {
				name = item.node.Name
			}
			fits := w >= 8 && h >= 3
			if !fits {
				allFit = false
			}
			t.Logf("  %-20s w=%-2d h=%-2d fits=%v", name, w, h, fits)
		}
		t.Logf("  allFit=%v", allFit)
	}
}

func TestTreemapWithRealData(t *testing.T) {
	// Simulate the actual C: drive data from the screenshot
	root := &model.Node{
		Name:  "C:",
		IsDir: true,
	}

	// Real sizes from the screenshot
	children := []*model.Node{
		{Name: "Program Files (x86)", Size: 546700000000, IsDir: true, Parent: root},
		{Name: "Users", Size: 501700000000, IsDir: true, Parent: root},
		{Name: "XboxGames", Size: 283300000000, IsDir: true, Parent: root},
		{Name: "Program Files", Size: 180200000000, IsDir: true, Parent: root},
		{Name: "dev", Size: 138700000000, IsDir: true, Parent: root},
		{Name: "Windows", Size: 38200000000, IsDir: true, Parent: root},
		{Name: "ProgramData", Size: 13500000000, IsDir: true, Parent: root},
		{Name: "hiberfil.sys", Size: 12400000000, IsDir: false, Parent: root},
		{Name: "pagefile.sys", Size: 9000000000, IsDir: false, Parent: root},
		{Name: "$Recycle.Bin", Size: 3100000000, IsDir: true, Parent: root},
		{Name: "msys64", Size: 326600000, IsDir: true, Parent: root},
		{Name: "AMD", Size: 64600000, IsDir: true, Parent: root},
		{Name: "swapfile.sys", Size: 16000000, IsDir: false, Parent: root},
		{Name: "MSI", Size: 1200000, IsDir: true, Parent: root},
		{Name: "Recovery", Size: 1100, IsDir: true, Parent: root},
		{Name: "PerfLogs", Size: 0, IsDir: true, Parent: root},
	}
	root.Children = children

	// Simulate the treemap panel size (roughly half of 180 char wide terminal)
	panel := NewTreemapPanel()
	panel.SetSize(90, 50) // Approximate size from screenshot
	panel.SetRoot(root)

	t.Logf("Generated %d blocks", len(panel.blocks))

	contentW := panel.width - 2  // treemapBorderH + treemapPadding
	contentH := panel.height     // treemapBorderV = 0

	t.Logf("Content area: %dx%d = %d chars", contentW, contentH, contentW*contentH)

	// Check each block
	for i, block := range panel.blocks {
		name := "grouped"
		size := int64(0)
		if block.IsGrouped {
			name = fmt.Sprintf("%d more", block.GroupCount)
			size = block.GroupSize
		} else if block.Node != nil {
			name = block.Node.Name
			size = block.Node.Size
		}

		area := block.Width * block.Height
		t.Logf("Block[%d] %-20s: x=%-3d y=%-3d w=%-3d h=%-3d area=%-5d size=%s",
			i, name, block.X, block.Y, block.Width, block.Height, area, FormatSize(size))

		// Check if block is too small to show label
		if block.Width <= 4 || block.Height <= 2 {
			t.Logf("  WARNING: Block too small to show label!")
		}
	}
}

func TestTreemapBlocksTile(t *testing.T) {
	// Create test nodes
	root := &model.Node{
		Name:  "root",
		IsDir: true,
	}

	// Just 3 equal-sized children for simple tiling test
	children := []*model.Node{
		{Name: "a", Size: 100, IsDir: true, Parent: root},
		{Name: "b", Size: 100, IsDir: true, Parent: root},
		{Name: "c", Size: 100, IsDir: true, Parent: root},
	}
	root.Children = children

	panel := NewTreemapPanel()
	panel.SetSize(40, 12)
	panel.SetRoot(root)

	if len(panel.blocks) != 3 {
		t.Fatalf("Expected 3 blocks, got %d", len(panel.blocks))
	}

	contentW := panel.width - 2  // treemapBorderH + treemapPadding
	contentH := panel.height     // treemapBorderV = 0

	t.Logf("Content area: %dx%d", contentW, contentH)

	// Check that blocks cover most of the area (allow for rounding)
	totalArea := 0
	for i, block := range panel.blocks {
		name := "grouped"
		if block.Node != nil {
			name = block.Node.Name
		}
		t.Logf("Block[%d] %s: x=%d y=%d w=%d h=%d (area=%d)",
			i, name, block.X, block.Y, block.Width, block.Height,
			block.Width*block.Height)
		totalArea += block.Width * block.Height
	}

	expectedArea := contentW * contentH
	coverage := float64(totalArea) / float64(expectedArea)
	t.Logf("Total area: %d, Expected: %d, Coverage: %.1f%%", totalArea, expectedArea, coverage*100)

	// Should cover at least 90% of the area
	if coverage < 0.90 {
		t.Errorf("Blocks only cover %.1f%% of area, expected at least 90%%", coverage*100)
	}
}
