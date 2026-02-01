package tui

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeffwilliams/squarify"
	"github.com/lumipallolabs/diskdive/internal/model"
)

// Block represents a rectangle in the treemap
type Block struct {
	Node          *model.Node
	X, Y          int
	Width, Height int
	// For grouped items (when Node is nil)
	IsGrouped   bool
	GroupCount  int
	GroupSize   int64
}

// TreemapPanel displays a treemap visualization
type TreemapPanel struct {
	root     *model.Node
	focus    *model.Node
	selected *model.Node
	blocks   []Block
	width    int
	height   int
	focused  bool

	// Render cache
	cachedView     string
	cacheValid     bool
	cachedFocus    *model.Node
	cachedSelected *model.Node
	cachedFocused  bool
}

// NewTreemapPanel creates a new treemap panel
func NewTreemapPanel() TreemapPanel {
	return TreemapPanel{}
}

// SetRoot sets the root node
func (t *TreemapPanel) SetRoot(root *model.Node) {
	t.root = root
	t.focus = root
	t.selected = root
	t.layout()
}

// SetSize sets the panel dimensions
func (t *TreemapPanel) SetSize(w, h int) {
	if t.width != w || t.height != h {
		t.width = w
		t.height = h
		t.cacheValid = false
		t.layout()
	}
}

// SetFocused sets focus state
func (t *TreemapPanel) SetFocused(focused bool) {
	t.focused = focused
}

// InvalidateCache marks the render cache as invalid
func (t *TreemapPanel) InvalidateCache() {
	t.cacheValid = false
}

// SetFocus sets the focus node (what to display in treemap)
// If a file is selected, shows its parent directory contents instead
func (t *TreemapPanel) SetFocus(node *model.Node) {
	if node == nil {
		return
	}
	// Files: show parent directory so file appears among siblings
	if !node.IsDir && node.Parent != nil {
		t.focus = node.Parent
	} else {
		t.focus = node
	}
	t.layout()
}

// SetSelected sets the selected node (for sync from tree)
func (t *TreemapPanel) SetSelected(node *model.Node) {
	if node == nil {
		return
	}
	t.selected = node

	// Update focus if selected is outside current focus
	if t.focus != nil && !t.isDescendant(node, t.focus) {
		// Find the ancestor that is a child of root
		t.focus = t.findAncestorUnderRoot(node)
		t.layout()
	}
}

// Selected returns the currently selected node
func (t TreemapPanel) Selected() *model.Node {
	return t.selected
}

// SelectFirst selects the first non-grouped block
func (t *TreemapPanel) SelectFirst() {
	for i := range t.blocks {
		if !t.blocks[i].IsGrouped && t.blocks[i].Node != nil {
			t.selected = t.blocks[i].Node
			return
		}
	}
}

// ZoomIn focuses on the selected folder
func (t *TreemapPanel) ZoomIn() {
	if t.selected != nil && t.selected.IsDir && len(t.selected.Children) > 0 {
		t.focus = t.selected
		t.layout()
	}
}

// ZoomOut goes to parent folder
func (t *TreemapPanel) ZoomOut() {
	if t.focus != nil && t.focus.Parent != nil {
		t.focus = t.focus.Parent
		t.layout()
	}
}

// MoveToBlock moves selection to an adjacent block
func (t *TreemapPanel) MoveToBlock(dx, dy int) {
	if len(t.blocks) == 0 {
		return
	}

	// Find current block
	var currentBlock *Block
	for i := range t.blocks {
		if !t.blocks[i].IsGrouped && t.blocks[i].Node == t.selected {
			currentBlock = &t.blocks[i]
			break
		}
	}

	if currentBlock == nil {
		// Select first non-grouped block if no current selection in view
		for i := range t.blocks {
			if !t.blocks[i].IsGrouped && t.blocks[i].Node != nil {
				t.selected = t.blocks[i].Node
				return
			}
		}
		return
	}

	// Find center of current block
	cx := currentBlock.X + currentBlock.Width/2
	cy := currentBlock.Y + currentBlock.Height/2

	// Find best candidate in the requested direction
	var bestBlock *Block
	bestDist := -1

	for i := range t.blocks {
		block := &t.blocks[i]
		// Skip grouped blocks and current selection
		if block.IsGrouped || block.Node == nil || block.Node == t.selected {
			continue
		}

		// Calculate center of candidate block
		bx := block.X + block.Width/2
		by := block.Y + block.Height/2

		// Check if block is in the right direction
		if dx > 0 && bx <= cx {
			continue
		}
		if dx < 0 && bx >= cx {
			continue
		}
		if dy > 0 && by <= cy {
			continue
		}
		if dy < 0 && by >= cy {
			continue
		}

		// Calculate distance
		dist := abs(bx-cx) + abs(by-cy)

		if bestDist < 0 || dist < bestDist {
			bestDist = dist
			bestBlock = block
		}
	}

	if bestBlock != nil && bestBlock.Node != nil {
		t.selected = bestBlock.Node
	}
}

// treemapItem wraps a node for the squarify algorithm
type treemapItem struct {
	node *model.Node
	size float64
	// For grouped items
	isGrouped  bool
	groupCount int
	groupSize  int64
	// Children for TreeSizer interface
	children []*treemapItem
}

// Size implements squarify.TreeSizer
func (t *treemapItem) Size() float64 {
	return t.size
}

// NumChildren implements squarify.TreeSizer
func (t *treemapItem) NumChildren() int {
	return len(t.children)
}

// Child implements squarify.TreeSizer
func (t *treemapItem) Child(i int) squarify.TreeSizer {
	return t.children[i]
}

const (
	minBlockWidth   = 8  // minimum width for any block (fits short label)
	minBlockHeight  = 3  // minimum height for any block (border + 1 line text)
	maxVisibleItems = 15 // max items before grouping remainder into "N more"

	// Layout constants for treemap panel (no outer border - blocks have their own)
	treemapBorderH = 2 // margin for rightmost block borders
	treemapPadding = 0 // no padding
	treemapBorderV = 0 // no vertical margin needed
)

// layout calculates block positions using the squarify library
func (t *TreemapPanel) layout() {
	t.blocks = nil
	t.cacheValid = false // Invalidate render cache

	if t.focus == nil || t.width <= 2 || t.height <= 2 {
		return
	}

	// Get children to display
	var nodes []*model.Node
	if t.focus.IsDir && len(t.focus.Children) > 0 {
		nodes = make([]*model.Node, len(t.focus.Children))
		copy(nodes, t.focus.Children)
		model.SortBySize(nodes)
	} else {
		// Single file or empty dir - show as single block
		nodes = []*model.Node{t.focus}
	}

	// Available space (accounting for border and padding)
	contentW := t.width - treemapBorderH - treemapPadding
	contentH := t.height - treemapBorderV

	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}

	// Prepare items with their REAL sizes - no modifications
	items := make([]*treemapItem, 0, len(nodes))
	for _, n := range nodes {
		size := float64(n.TotalSize())
		if size < 1 {
			size = 1 // Prevent division by zero, but keep proportions
		}
		items = append(items, &treemapItem{node: n, size: size})
	}

	// Sort by size descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].size > items[j].size
	})

	rect := squarify.Rect{
		X: 0,
		Y: 0,
		W: float64(contentW),
		H: float64(contentH),
	}

	var blocks []squarify.Block
	var metas []squarify.Meta
	var displayItems []*treemapItem
	var root *treemapItem

	// Try to fit as many items as possible
	maxVisible := len(items)
	if maxVisible > maxVisibleItems {
		maxVisible = 15
	}

	// Find the maximum items that fit with minimum dimensions
	for maxVisible >= 2 {
		// Calculate rect for main items (reserve bottom strip for "N more" if grouping)
		mainRect := rect
		// Only group if there would be 2+ items to group (don't show "1 more")
		// Calculate how many items would be grouped with current maxVisible
		numVisible := maxVisible
		if numVisible > len(items) {
			numVisible = len(items)
		}
		remainingItems := len(items) - numVisible

		// If only 1 item would be grouped, try to show it instead
		// by not reserving space for the grouped block
		hasGroupedItems := remainingItems >= 2
		if hasGroupedItems {
			// Reserve bottom strip for "N more" block
			mainRect.H = float64(contentH - minBlockHeight)
			// We'll show maxVisible-1 items + grouped block
			numVisible = maxVisible - 1
			if numVisible > len(items) {
				numVisible = len(items)
			}
		}

		displayItems = make([]*treemapItem, 0, numVisible)
		for i := 0; i < numVisible; i++ {
			displayItems = append(displayItems, items[i])
		}

		// Create root for squarify (main items only)
		root = &treemapItem{
			size:     0,
			children: displayItems,
		}
		for _, child := range displayItems {
			root.size += child.size
		}

		// Run squarify on main items
		blocks, metas = squarify.Squarify(root, mainRect, squarify.Options{
			MaxDepth: 1,
			Sort:     true,
		})

		// Check if all main blocks meet minimum dimensions
		allFit := true
		for i, block := range blocks {
			if i >= len(metas) || metas[i].Depth != 0 {
				continue
			}
			w := int(math.Floor(block.X+block.W)) - int(math.Floor(block.X))
			h := int(math.Floor(block.Y+block.H)) - int(math.Floor(block.Y))
			if w < minBlockWidth || h < minBlockHeight {
				allFit = false
				break
			}
		}

		if allFit {
			// Add the grouped items block at the bottom if needed
			// Only group if there are 2+ items to group (don't show "1 more")
			remainingItems := len(items) - numVisible
			if hasGroupedItems && remainingItems >= 2 {
				var groupSize int64
				for i := numVisible; i < len(items); i++ {
					groupSize += int64(items[i].size)
				}
				// Manually add the "N more" block at the bottom
				t.blocks = append(t.blocks, Block{
					X:          0,
					Y:          contentH - minBlockHeight,
					Width:      contentW,
					Height:     minBlockHeight,
					IsGrouped:  true,
					GroupCount: remainingItems,
					GroupSize:  groupSize,
				})
			}
			break // Found a working configuration
		}
		maxVisible--
	}

	// Handle edge case: only 1 item fits
	if maxVisible < 2 && len(items) > 0 {
		// Show the largest item, with "N more" only if 2+ items would be grouped
		displayItems = items[:1]
		root = &treemapItem{
			size:     items[0].size,
			children: displayItems,
		}
		mainRect := rect

		// Only reserve space for grouped block if 2+ items to group
		needsGrouped := len(items) > 2 // 1 shown + 2+ grouped
		if needsGrouped {
			mainRect.H = float64(contentH - minBlockHeight)
		}

		blocks, metas = squarify.Squarify(root, mainRect, squarify.Options{
			MaxDepth: 1,
			Sort:     true,
		})

		// Add grouped block only if 2+ items
		if needsGrouped {
			var groupSize int64
			for i := 1; i < len(items); i++ {
				groupSize += int64(items[i].size)
			}
			t.blocks = append(t.blocks, Block{
				X:          0,
				Y:          contentH - minBlockHeight,
				Width:      contentW,
				Height:     minBlockHeight,
				IsGrouped:  true,
				GroupCount: len(items) - 1,
				GroupSize:  groupSize,
			})
		}
	}

	// Convert squarify blocks to our Block type
	// Track where main blocks actually end (for placing "N more" without gaps)
	maxMainBlockEndY := 0

	for i, block := range blocks {
		item, ok := block.TreeSizer.(*treemapItem)
		if !ok {
			continue
		}

		// Only process leaf nodes at depth 0 (immediate children of root)
		// The root itself is not returned by squarify, only its children
		// depth 0 = children we want to display
		if i >= len(metas) || metas[i].Depth != 0 {
			continue
		}

		// Convert float to int - use Round for all to get consistent boundaries
		// This prevents adjacent blocks from overlapping (one's border overwriting another's)
		x := int(math.Round(block.X))
		y := int(math.Round(block.Y))
		endX := int(math.Round(block.X + block.W))
		endY := int(math.Round(block.Y + block.H))
		w := endX - x
		h := endY - y

		// Clip to content bounds
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		if endX > contentW {
			w = contentW - x
		}
		if endY > contentH {
			h = contentH - y
		}

		// Skip blocks that are too small or outside bounds
		if w < 1 || h < 1 || x >= contentW || y >= contentH {
			continue
		}

		// Track where main blocks end
		if y+h > maxMainBlockEndY {
			maxMainBlockEndY = y + h
		}

		t.blocks = append(t.blocks, Block{
			Node:       item.node,
			X:          x,
			Y:          y,
			Width:      w,
			Height:     h,
			IsGrouped:  item.isGrouped,
			GroupCount: item.groupCount,
			GroupSize:  item.groupSize,
		})
	}

	// Adjust "N more" block position to start right after main blocks (no gap)
	// and extend to fill remaining space
	for i := range t.blocks {
		if t.blocks[i].IsGrouped {
			t.blocks[i].Y = maxMainBlockEndY
			t.blocks[i].Height = contentH - maxMainBlockEndY
			if t.blocks[i].Height < 1 {
				t.blocks[i].Height = 1
			}
			break
		}
	}
}

// View renders the treemap
func (t *TreemapPanel) View() string {
	if t.focus == nil {
		return TreemapPanelStyle.Render("No data")
	}

	// Check if cache is valid
	if t.cacheValid &&
		t.cachedFocus == t.focus &&
		t.cachedSelected == t.selected &&
		t.cachedFocused == t.focused {
		return t.cachedView
	}

	// Content dimensions
	contentW := t.width - treemapBorderH
	contentH := t.height - treemapBorderV
	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}

	// Render each block completely using lipgloss, then composite line by line
	type renderedBlock struct {
		block Block
		lines []string
	}

	var rendered []renderedBlock
	for _, block := range t.blocks {
		if block.Width < 1 || block.Height < 1 {
			continue
		}
		blockStr := t.renderBlock(block)
		lines := strings.Split(blockStr, "\n")
		rendered = append(rendered, renderedBlock{block, lines})
	}

	// Build output line by line
	var outputLines []string
	for y := 0; y < contentH; y++ {
		// Find all blocks that have content at this row and sort by X
		type blockSegment struct {
			x     int
			width int
			line  string
		}
		var segments []blockSegment

		for _, rb := range rendered {
			// Check if this block has content at row y
			lineIdx := y - rb.block.Y
			if lineIdx >= 0 && lineIdx < len(rb.lines) && lineIdx < rb.block.Height {
				segments = append(segments, blockSegment{
					x:     rb.block.X,
					width: rb.block.Width,
					line:  rb.lines[lineIdx],
				})
			}
		}

		// Sort segments by X position
		sort.Slice(segments, func(i, j int) bool {
			return segments[i].x < segments[j].x
		})

		// Build the line by placing segments with padding between them
		var lineBuilder strings.Builder
		currentX := 0
		for _, seg := range segments {
			// Add padding to reach this segment's X position
			if seg.x > currentX {
				lineBuilder.WriteString(strings.Repeat(" ", seg.x-currentX))
			}
			lineBuilder.WriteString(seg.line)
			currentX = seg.x + seg.width
		}
		outputLines = append(outputLines, lineBuilder.String())
	}

	content := strings.Join(outputLines, "\n")
	style := lipgloss.NewStyle().Height(t.height).MaxHeight(t.height)

	// Cache the result
	t.cachedView = style.Render(content)
	t.cacheValid = true
	t.cachedFocus = t.focus
	t.cachedSelected = t.selected
	t.cachedFocused = t.focused

	return t.cachedView
}

// renderBlock renders a complete block using lipgloss and returns the styled string
func (t TreemapPanel) renderBlock(block Block) string {
	// Determine colors - border color indicates type, no background fill
	var fgColor, borderColor lipgloss.Color

	if block.IsGrouped {
		fgColor = lipgloss.Color("#6B7280")
		borderColor = lipgloss.Color("#4B5563")
	} else if block.Node != nil && block.Node.IsDeleted {
		// Deleted items shown in muted gray
		fgColor = lipgloss.Color("#6B7280")
		borderColor = lipgloss.Color("#374151")
	} else {
		if block.Node != nil && block.Node.IsDir {
			// Directories: cyan border and text
			fgColor = ColorDir
			borderColor = ColorDir
		} else {
			// Files: muted gray border and text
			fgColor = ColorFile
			borderColor = lipgloss.Color("#6B7280")
		}
	}

	isSelected := block.Node == t.selected
	if isSelected && t.focused {
		// Bright violet border, white text when focused
		fgColor = lipgloss.Color("#FFFFFF")
		borderColor = ColorPrimary
	} else if isSelected {
		// Dimmer selection when unfocused - still visible but not as prominent
		fgColor = lipgloss.Color("#E0E0E0")
		borderColor = lipgloss.Color("#9D7CD8") // dimmer violet
	}

	// Build label
	var label, sizeStr string
	if block.IsGrouped {
		label = fmt.Sprintf("%d more", block.GroupCount)
		sizeStr = FormatSize(block.GroupSize)
	} else if block.Node != nil {
		label = block.Node.Name
		sizeStr = FormatSize(block.Node.TotalSize())
	}

	// Inner dimensions (excluding border)
	innerW := block.Width - 2
	innerH := block.Height - 2
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}

	// Build content text
	text := label
	if innerH > 1 && sizeStr != "" {
		text = label + "\n" + sizeStr
	}

	// Render the block with border using lipgloss
	blockStyle := lipgloss.NewStyle().
		Width(innerW).
		Height(innerH).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Foreground(fgColor)

	if isSelected {
		blockStyle = blockStyle.Bold(true)
	}

	return blockStyle.Render(text)
}

// isDescendant checks if node is a descendant of ancestor
func (t TreemapPanel) isDescendant(node, ancestor *model.Node) bool {
	if node == nil || ancestor == nil {
		return false
	}
	for n := node; n != nil; n = n.Parent {
		if n == ancestor {
			return true
		}
	}
	return false
}

// findAncestorUnderRoot finds the ancestor of node that is a direct child of root
func (t TreemapPanel) findAncestorUnderRoot(node *model.Node) *model.Node {
	if node == nil || t.root == nil {
		return t.root
	}

	// Walk up to find the node under root
	for n := node; n != nil; n = n.Parent {
		if n.Parent == t.root {
			return n
		}
		if n == t.root {
			return t.root
		}
	}
	return t.root
}

// abs returns the absolute value of x
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
