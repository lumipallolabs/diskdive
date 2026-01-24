package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/samuli/diskdive/internal/model"
)

// Block represents a rectangle in the treemap
type Block struct {
	Node          *model.Node
	X, Y          int
	Width, Height int
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
	t.width = w
	t.height = h
	t.layout()
}

// SetFocused sets focus state
func (t *TreemapPanel) SetFocused(focused bool) {
	t.focused = focused
}

// SetShowDiff enables/disables diff display
func (t *TreemapPanel) SetShowDiff(show bool) {
	t.showDiff = show
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

// layout calculates block positions using slice-and-dice algorithm
func (t *TreemapPanel) layout() {
	t.blocks = nil

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
	contentW := t.width - 4
	contentH := t.height - 2

	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}

	t.sliceDice(nodes, 0, 0, contentW, contentH, true)
}

// sliceDice recursively divides space using slice-and-dice algorithm
func (t *TreemapPanel) sliceDice(nodes []*model.Node, x, y, w, h int, horizontal bool) {
	if len(nodes) == 0 || w < 1 || h < 1 {
		return
	}

	// Calculate total size
	var totalSize int64
	for _, n := range nodes {
		totalSize += n.TotalSize()
	}

	if totalSize == 0 {
		// Equal division if all sizes are 0
		totalSize = int64(len(nodes))
		for i, n := range nodes {
			if horizontal {
				nodeW := w / len(nodes)
				nodeX := x + i*nodeW
				if i == len(nodes)-1 {
					nodeW = w - i*nodeW // Last one gets remainder
				}
				t.blocks = append(t.blocks, Block{
					Node:   n,
					X:      nodeX,
					Y:      y,
					Width:  nodeW,
					Height: h,
				})
			} else {
				nodeH := h / len(nodes)
				nodeY := y + i*nodeH
				if i == len(nodes)-1 {
					nodeH = h - i*nodeH
				}
				t.blocks = append(t.blocks, Block{
					Node:   n,
					X:      x,
					Y:      nodeY,
					Width:  w,
					Height: nodeH,
				})
			}
		}
		return
	}

	// Single node - create block
	if len(nodes) == 1 {
		t.blocks = append(t.blocks, Block{
			Node:   nodes[0],
			X:      x,
			Y:      y,
			Width:  w,
			Height: h,
		})
		return
	}

	// Slice and dice
	offset := 0
	for i, n := range nodes {
		ratio := float64(n.TotalSize()) / float64(totalSize)

		if horizontal {
			nodeW := int(float64(w) * ratio)
			if nodeW < 1 {
				nodeW = 1
			}
			// Last node gets the remainder
			if i == len(nodes)-1 {
				nodeW = w - offset
			}
			if offset+nodeW > w {
				nodeW = w - offset
			}
			if nodeW > 0 {
				t.blocks = append(t.blocks, Block{
					Node:   n,
					X:      x + offset,
					Y:      y,
					Width:  nodeW,
					Height: h,
				})
				offset += nodeW
			}
		} else {
			nodeH := int(float64(h) * ratio)
			if nodeH < 1 {
				nodeH = 1
			}
			if i == len(nodes)-1 {
				nodeH = h - offset
			}
			if offset+nodeH > h {
				nodeH = h - offset
			}
			if nodeH > 0 {
				t.blocks = append(t.blocks, Block{
					Node:   n,
					X:      x,
					Y:      y + offset,
					Width:  w,
					Height: nodeH,
				})
				offset += nodeH
			}
		}
	}
}

// View renders the treemap
func (t TreemapPanel) View() string {
	if t.focus == nil {
		return TreemapPanelStyle.Width(t.width).Height(t.height).Render("No data")
	}

	// Calculate content area
	contentW := t.width - 4
	contentH := t.height - 2

	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}

	// Create a 2D grid
	grid := make([][]rune, contentH)
	colors := make([][]lipgloss.Style, contentH)
	for i := range grid {
		grid[i] = make([]rune, contentW)
		colors[i] = make([]lipgloss.Style, contentW)
		for j := range grid[i] {
			grid[i][j] = ' '
			colors[i][j] = lipgloss.NewStyle()
		}
	}

	// Draw blocks
	for _, block := range t.blocks {
		t.drawBlock(grid, colors, block, contentW, contentH)
	}

	// Render grid to string
	var lines []string
	for y := 0; y < contentH; y++ {
		var line strings.Builder
		for x := 0; x < contentW; x++ {
			line.WriteString(colors[y][x].Render(string(grid[y][x])))
		}
		lines = append(lines, line.String())
	}

	content := strings.Join(lines, "\n")

	style := TreemapPanelStyle.Width(t.width).Height(t.height)
	if t.focused {
		style = style.BorderForeground(ColorPrimary)
	}

	return style.Render(content)
}

// drawBlock draws a single block onto the grid
func (t TreemapPanel) drawBlock(grid [][]rune, colors [][]lipgloss.Style, block Block, gridW, gridH int) {
	if block.Width < 1 || block.Height < 1 {
		return
	}

	// Determine block color
	var bgColor lipgloss.Color
	var fgColor lipgloss.Color

	if t.showDiff && block.Node != nil {
		if block.Node.IsNew {
			bgColor = ColorNew
			fgColor = lipgloss.Color("#000000")
		} else if change := block.Node.SizeChange(); change > 0 {
			bgColor = ColorGrewBg
			fgColor = ColorGrew
		} else if change < 0 {
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
			fgColor = lipgloss.Color("#7DD3FC")
		} else {
			bgColor = lipgloss.Color("#2D2D2D")
			fgColor = lipgloss.Color("#E4E4E7")
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
	if block.Node != nil && block.Width > 4 && block.Height > 2 {
		label := block.Node.Name
		maxLen := block.Width - 4
		if maxLen > 0 && len(label) > maxLen {
			label = label[:maxLen]
		}

		labelY := block.Y + 1
		labelX := block.X + 2

		if labelY < gridH && labelX < gridW && maxLen > 0 {
			labelStyle := blockStyle
			for i, ch := range label {
				x := labelX + i
				if x < gridW && x < block.X+block.Width-2 {
					grid[labelY][x] = ch
					colors[labelY][x] = labelStyle
				}
			}
		}

		// Show size on next line if space
		if block.Height > 3 && block.Width > 6 {
			sizeStr := FormatSize(block.Node.TotalSize())
			sizeY := block.Y + 2
			sizeX := block.X + 2

			if sizeY < gridH {
				for i, ch := range sizeStr {
					x := sizeX + i
					if x < gridW && x < block.X+block.Width-2 {
						grid[sizeY][x] = ch
						colors[sizeY][x] = blockStyle
					}
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
