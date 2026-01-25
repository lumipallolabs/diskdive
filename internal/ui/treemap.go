package ui

import (
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeffwilliams/squarify"
	"github.com/samuli/diskdive/internal/model"
)

var treemapDebugLog *log.Logger

func init() {
	f, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	treemapDebugLog = log.New(f, "", log.Lmicroseconds)
}

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
	showDiff bool

	// Render cache
	cachedView     string
	cacheValid     bool
	cachedFocus    *model.Node
	cachedSelected *model.Node
	cachedFocused  bool
	cachedShowDiff bool
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

// SetShowDiff enables/disables diff display
func (t *TreemapPanel) SetShowDiff(show bool) {
	t.showDiff = show
}

// SetFocus sets the focus node (what to display in treemap)
func (t *TreemapPanel) SetFocus(node *model.Node) {
	if node == nil {
		return
	}
	t.focus = node
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
	if len(t.blocks) == 0 || t.selected == nil {
		return
	}

	// Find current block
	var currentBlock *Block
	for i := range t.blocks {
		if t.blocks[i].Node == t.selected {
			currentBlock = &t.blocks[i]
			break
		}
	}

	if currentBlock == nil {
		// Select first block if no current selection in view
		if len(t.blocks) > 0 {
			t.selected = t.blocks[0].Node
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
		if block.Node == t.selected {
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

	if bestBlock != nil {
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

	// Layout constants for treemap panel (no left border - shares with tree panel)
	treemapBorderH = 1 // right border only (left shared with tree)
	treemapPadding = 2 // 1 char padding on each side
	treemapBorderV = 2 // top + bottom border
)

// layout calculates block positions using the squarify library
func (t *TreemapPanel) layout() {
	start := time.Now()
	defer func() {
		if treemapDebugLog != nil {
			treemapDebugLog.Printf("TreemapPanel.layout: %v", time.Since(start))
		}
	}()

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

		// Convert float to int using floor for start, round for end Y (to prevent vertical gaps)
		x := int(math.Floor(block.X))
		y := int(math.Floor(block.Y))
		endX := int(math.Floor(block.X + block.W))
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
	start := time.Now()
	defer func() {
		if treemapDebugLog != nil {
			treemapDebugLog.Printf("TreemapPanel.View: %v (cached=%v)", time.Since(start), t.cacheValid)
		}
	}()

	if t.focus == nil {
		return TreemapPanelStyle.Render("No data")
	}

	// Check if cache is valid
	if t.cacheValid &&
		t.cachedFocus == t.focus &&
		t.cachedSelected == t.selected &&
		t.cachedFocused == t.focused &&
		t.cachedShowDiff == t.showDiff {
		return t.cachedView
	}

	// Content dimensions (accounting for border and padding)
	contentW := t.width - treemapBorderH - treemapPadding
	contentH := t.height - treemapBorderV
	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}

	// Create a 2D grid with style indices instead of full styles
	grid := make([][]rune, contentH)
	styleIdx := make([][]int, contentH)
	for i := range grid {
		grid[i] = make([]rune, contentW)
		styleIdx[i] = make([]int, contentW)
		for j := range grid[i] {
			grid[i][j] = ' '
			styleIdx[i][j] = 0 // 0 = empty/default
		}
	}

	// Collect styles used - index 0 is reserved for empty
	styles := []lipgloss.Style{lipgloss.NewStyle()}

	// Draw blocks (with bounds validation)
	for _, block := range t.blocks {
		// Ensure block is within bounds (defensive)
		if block.X < 0 || block.Y < 0 ||
			block.X+block.Width > contentW ||
			block.Y+block.Height > contentH {
			// Skip invalid blocks
			continue
		}
		t.drawBlockIndexed(grid, styleIdx, &styles, block, contentW, contentH)
	}

	// Render grid to string - batch by style index
	var lines []string
	for y := 0; y < contentH; y++ {
		var line strings.Builder
		x := 0
		for x < contentW {
			// Find run of cells with same style index
			currentIdx := styleIdx[y][x]
			var run strings.Builder
			for x < contentW && styleIdx[y][x] == currentIdx {
				run.WriteRune(grid[y][x])
				x++
			}
			line.WriteString(styles[currentIdx].Render(run.String()))
		}
		lines = append(lines, line.String())
	}

	content := strings.Join(lines, "\n")

	// Apply border - no left border (shares with tree panel)
	// Set Height to match tree panel (which uses Height(t.height))
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		BorderLeft(false).
		Height(t.height)
	if t.focused {
		style = style.BorderForeground(ColorPrimary)
	}

	// Cache the result
	t.cachedView = style.Render(content)
	t.cacheValid = true
	t.cachedFocus = t.focus
	t.cachedSelected = t.selected
	t.cachedFocused = t.focused
	t.cachedShowDiff = t.showDiff

	return t.cachedView
}

// drawBlock draws a single block onto the grid
func (t TreemapPanel) drawBlock(grid [][]rune, colors [][]lipgloss.Style, block Block, gridW, gridH int) {
	if block.Width < 1 || block.Height < 1 {
		return
	}

	// Determine block color
	var bgColor lipgloss.Color
	var fgColor lipgloss.Color

	if block.IsGrouped {
		// Grouped items get a distinct color
		bgColor = lipgloss.Color("#3D3D3D")
		fgColor = lipgloss.Color("#9CA3AF")
	} else if t.showDiff && block.Node != nil {
		if block.Node.IsNew {
			// Entirely new item (yellow)
			bgColor = ColorNew
			fgColor = lipgloss.Color("#000000")
		} else if block.Node.IsDir && block.Node.HasGrew && block.Node.HasShrunk {
			// Folder has both additions and removals (purple/mixed)
			bgColor = ColorMixedBg
			fgColor = ColorMixed
		} else if block.Node.HasGrew {
			// Item or folder grew / contains only additions (orange)
			bgColor = ColorGrewBg
			fgColor = ColorGrew
		} else if block.Node.HasShrunk {
			// Item or folder shrunk / contains only removals (cyan)
			bgColor = ColorShrunkBg
			fgColor = ColorShrunk
		} else {
			bgColor = lipgloss.Color("#2D2D2D")
			fgColor = lipgloss.Color("#9CA3AF")
		}
	} else {
		// Default coloring based on depth or type
		if block.Node != nil && block.Node.IsDir {
			bgColor = lipgloss.Color("#1E3A5F")
			fgColor = ColorDir
		} else {
			bgColor = lipgloss.Color("#2D2D2D")
			fgColor = ColorFile
		}
	}

	// Selection highlight
	isSelected := block.Node == t.selected && t.focused

	blockStyle := lipgloss.NewStyle().Background(bgColor).Foreground(fgColor)
	if isSelected {
		blockStyle = blockStyle.Background(ColorPrimary).Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	}

	// Fill block area
	for y := block.Y; y < block.Y+block.Height && y < gridH; y++ {
		for x := block.X; x < block.X+block.Width && x < gridW; x++ {
			if y >= 0 && x >= 0 {
				grid[y][x] = ' '
				colors[y][x] = blockStyle
			}
		}
	}

	// Draw border
	borderStyle := lipgloss.NewStyle().Background(bgColor).Foreground(lipgloss.Color("#4B5563"))
	if isSelected {
		borderStyle = borderStyle.Background(ColorPrimary).Foreground(lipgloss.Color("#FFFFFF"))
	}

	// Top and bottom borders
	for x := block.X; x < block.X+block.Width && x < gridW; x++ {
		if x >= 0 {
			if block.Y >= 0 && block.Y < gridH {
				grid[block.Y][x] = '\u2500' // horizontal line
				colors[block.Y][x] = borderStyle
			}
			if block.Y+block.Height-1 >= 0 && block.Y+block.Height-1 < gridH {
				grid[block.Y+block.Height-1][x] = '\u2500'
				colors[block.Y+block.Height-1][x] = borderStyle
			}
		}
	}

	// Left and right borders
	for y := block.Y; y < block.Y+block.Height && y < gridH; y++ {
		if y >= 0 {
			if block.X >= 0 && block.X < gridW {
				grid[y][block.X] = '\u2502' // vertical line
				colors[y][block.X] = borderStyle
			}
			if block.X+block.Width-1 >= 0 && block.X+block.Width-1 < gridW {
				grid[y][block.X+block.Width-1] = '\u2502'
				colors[y][block.X+block.Width-1] = borderStyle
			}
		}
	}

	// Corners
	if block.Y >= 0 && block.Y < gridH && block.X >= 0 && block.X < gridW {
		grid[block.Y][block.X] = '\u250C' // top-left
		colors[block.Y][block.X] = borderStyle
	}
	if block.Y >= 0 && block.Y < gridH && block.X+block.Width-1 >= 0 && block.X+block.Width-1 < gridW {
		grid[block.Y][block.X+block.Width-1] = '\u2510' // top-right
		colors[block.Y][block.X+block.Width-1] = borderStyle
	}
	if block.Y+block.Height-1 >= 0 && block.Y+block.Height-1 < gridH && block.X >= 0 && block.X < gridW {
		grid[block.Y+block.Height-1][block.X] = '\u2514' // bottom-left
		colors[block.Y+block.Height-1][block.X] = borderStyle
	}
	if block.Y+block.Height-1 >= 0 && block.Y+block.Height-1 < gridH && block.X+block.Width-1 >= 0 && block.X+block.Width-1 < gridW {
		grid[block.Y+block.Height-1][block.X+block.Width-1] = '\u2518' // bottom-right
		colors[block.Y+block.Height-1][block.X+block.Width-1] = borderStyle
	}

	// Draw label if space permits
	if block.Width > 4 && block.Height > 2 {
		var label string
		var sizeStr string

		if block.IsGrouped {
			label = fmt.Sprintf("%d more", block.GroupCount)
			sizeStr = FormatSize(block.GroupSize)
		} else if block.Node != nil {
			label = block.Node.Name
			sizeStr = FormatSize(block.Node.TotalSize())
		}

		innerW := block.Width - 4
		innerH := block.Height - 2
		if innerW < 1 || innerH < 1 {
			return
		}

		// Use lipgloss to wrap text (don't set Height - let text be natural size)
		text := label
		if innerH > 1 && sizeStr != "" {
			text = label + "\n" + sizeStr
		}

		wrapped := lipgloss.NewStyle().Width(innerW).Render(text)

		// Draw wrapped text into grid
		lines := strings.Split(wrapped, "\n")
		for dy, line := range lines {
			if dy >= innerH {
				break
			}
			for dx, ch := range line {
				if dx >= innerW {
					break
				}
				x, y := block.X+2+dx, block.Y+1+dy
				if x < gridW && y < gridH {
					grid[y][x] = ch
					colors[y][x] = blockStyle
				}
			}
		}
	}
}

// drawBlockIndexed draws a single block using style indices for efficient batching
func (t TreemapPanel) drawBlockIndexed(grid [][]rune, styleIdx [][]int, styles *[]lipgloss.Style, block Block, gridW, gridH int) {
	if block.Width < 1 || block.Height < 1 {
		return
	}

	// Determine block color
	var bgColor lipgloss.Color
	var fgColor lipgloss.Color

	if block.IsGrouped {
		bgColor = lipgloss.Color("#3D3D3D")
		fgColor = lipgloss.Color("#9CA3AF")
	} else if t.showDiff && block.Node != nil {
		if block.Node.IsNew {
			bgColor = ColorNew
			fgColor = lipgloss.Color("#000000")
		} else if block.Node.IsDir && block.Node.HasGrew && block.Node.HasShrunk {
			bgColor = ColorMixedBg
			fgColor = ColorMixed
		} else if block.Node.HasGrew {
			bgColor = ColorGrewBg
			fgColor = ColorGrew
		} else if block.Node.HasShrunk {
			bgColor = ColorShrunkBg
			fgColor = ColorShrunk
		} else {
			bgColor = lipgloss.Color("#2D2D2D")
			fgColor = lipgloss.Color("#9CA3AF")
		}
	} else {
		if block.Node != nil && block.Node.IsDir {
			bgColor = lipgloss.Color("#1E3A5F")
			fgColor = ColorDir
		} else {
			bgColor = lipgloss.Color("#2D2D2D")
			fgColor = ColorFile
		}
	}

	isSelected := block.Node == t.selected && t.focused

	// Create styles and get their indices
	blockStyle := lipgloss.NewStyle().Background(bgColor).Foreground(fgColor)
	if isSelected {
		blockStyle = blockStyle.Background(ColorPrimary).Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	}
	blockIdx := len(*styles)
	*styles = append(*styles, blockStyle)

	borderStyle := lipgloss.NewStyle().Background(bgColor).Foreground(lipgloss.Color("#4B5563"))
	if isSelected {
		borderStyle = borderStyle.Background(ColorPrimary).Foreground(lipgloss.Color("#FFFFFF"))
	}
	borderIdx := len(*styles)
	*styles = append(*styles, borderStyle)

	// Fill block area
	for y := block.Y; y < block.Y+block.Height && y < gridH; y++ {
		for x := block.X; x < block.X+block.Width && x < gridW; x++ {
			if y >= 0 && x >= 0 {
				grid[y][x] = ' '
				styleIdx[y][x] = blockIdx
			}
		}
	}

	// Draw border
	for x := block.X; x < block.X+block.Width && x < gridW; x++ {
		if x >= 0 {
			if block.Y >= 0 && block.Y < gridH {
				grid[block.Y][x] = '\u2500'
				styleIdx[block.Y][x] = borderIdx
			}
			if block.Y+block.Height-1 >= 0 && block.Y+block.Height-1 < gridH {
				grid[block.Y+block.Height-1][x] = '\u2500'
				styleIdx[block.Y+block.Height-1][x] = borderIdx
			}
		}
	}

	for y := block.Y; y < block.Y+block.Height && y < gridH; y++ {
		if y >= 0 {
			if block.X >= 0 && block.X < gridW {
				grid[y][block.X] = '\u2502'
				styleIdx[y][block.X] = borderIdx
			}
			if block.X+block.Width-1 >= 0 && block.X+block.Width-1 < gridW {
				grid[y][block.X+block.Width-1] = '\u2502'
				styleIdx[y][block.X+block.Width-1] = borderIdx
			}
		}
	}

	// Corners
	if block.Y >= 0 && block.Y < gridH && block.X >= 0 && block.X < gridW {
		grid[block.Y][block.X] = '\u250C'
		styleIdx[block.Y][block.X] = borderIdx
	}
	if block.Y >= 0 && block.Y < gridH && block.X+block.Width-1 >= 0 && block.X+block.Width-1 < gridW {
		grid[block.Y][block.X+block.Width-1] = '\u2510'
		styleIdx[block.Y][block.X+block.Width-1] = borderIdx
	}
	if block.Y+block.Height-1 >= 0 && block.Y+block.Height-1 < gridH && block.X >= 0 && block.X < gridW {
		grid[block.Y+block.Height-1][block.X] = '\u2514'
		styleIdx[block.Y+block.Height-1][block.X] = borderIdx
	}
	if block.Y+block.Height-1 >= 0 && block.Y+block.Height-1 < gridH && block.X+block.Width-1 >= 0 && block.X+block.Width-1 < gridW {
		grid[block.Y+block.Height-1][block.X+block.Width-1] = '\u2518'
		styleIdx[block.Y+block.Height-1][block.X+block.Width-1] = borderIdx
	}

	// Draw label if space permits
	if block.Width > 4 && block.Height > 2 {
		var label string
		var sizeStr string

		if block.IsGrouped {
			label = fmt.Sprintf("%d more", block.GroupCount)
			sizeStr = FormatSize(block.GroupSize)
		} else if block.Node != nil {
			label = block.Node.Name
			sizeStr = FormatSize(block.Node.TotalSize())
		}

		innerW := block.Width - 4
		innerH := block.Height - 2
		if innerW < 1 || innerH < 1 {
			return
		}

		text := label
		if innerH > 1 && sizeStr != "" {
			text = label + "\n" + sizeStr
		}

		wrapped := lipgloss.NewStyle().Width(innerW).Render(text)
		lines := strings.Split(wrapped, "\n")
		for dy, line := range lines {
			if dy >= innerH {
				break
			}
			for dx, ch := range line {
				if dx >= innerW {
					break
				}
				x, y := block.X+2+dx, block.Y+1+dy
				if x < gridW && y < gridH {
					grid[y][x] = ch
					styleIdx[y][x] = blockIdx
				}
			}
		}
	}
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
