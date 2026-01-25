package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/samuli/diskdive/internal/model"
)

const treeSizeBarWidth = 4 // Width of size proportion bar [████]

// TreePanel displays the folder tree
type TreePanel struct {
	root     *model.Node
	cursor   int
	expanded map[string]bool
	visible  []*model.Node
	width    int
	height   int
	focused  bool
	showDiff bool
	offset   int // scroll offset
}

// NewTreePanel creates a new tree panel
func NewTreePanel() TreePanel {
	return TreePanel{
		expanded: make(map[string]bool),
	}
}

// SetRoot sets the root node
func (t *TreePanel) SetRoot(root *model.Node) {
	t.root = root
	t.cursor = 0
	t.offset = 0
	t.expanded = make(map[string]bool)
	if root != nil {
		t.expanded[root.Path] = true
	}
	t.updateVisible()
}

// SetSize sets the panel dimensions
func (t *TreePanel) SetSize(w, h int) {
	t.width = w
	t.height = h
}

// SetFocused sets focus state
func (t *TreePanel) SetFocused(focused bool) {
	t.focused = focused
}

// SetShowDiff enables/disables diff display
func (t *TreePanel) SetShowDiff(show bool) {
	t.showDiff = show
}

// Selected returns the currently selected node
func (t TreePanel) Selected() *model.Node {
	if t.cursor >= 0 && t.cursor < len(t.visible) {
		return t.visible[t.cursor]
	}
	return nil
}

// Update handles messages
func (t TreePanel) Update(msg tea.Msg) (TreePanel, tea.Cmd) {
	return t, nil
}

// MoveUp moves cursor up
func (t *TreePanel) MoveUp() {
	if t.cursor > 0 {
		t.cursor--
		t.ensureVisible()
	}
}

// MoveDown moves cursor down
func (t *TreePanel) MoveDown() {
	if t.cursor < len(t.visible)-1 {
		t.cursor++
		t.ensureVisible()
	}
}

// PageUp moves cursor up by quarter page
func (t *TreePanel) PageUp() {
	pageSize := (t.height - 4) / 4
	if pageSize < 1 {
		pageSize = 1
	}
	t.cursor -= pageSize
	if t.cursor < 0 {
		t.cursor = 0
	}
	t.ensureVisible()
}

// PageDown moves cursor down by quarter page
func (t *TreePanel) PageDown() {
	pageSize := (t.height - 4) / 4
	if pageSize < 1 {
		pageSize = 1
	}
	t.cursor += pageSize
	if t.cursor >= len(t.visible) {
		t.cursor = len(t.visible) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
	t.ensureVisible()
}

// Collapse collapses current folder
func (t *TreePanel) Collapse() {
	if node := t.Selected(); node != nil && node.IsDir {
		delete(t.expanded, node.Path)
		t.updateVisible()
	}
}

// Expand expands current folder
func (t *TreePanel) Expand() {
	if node := t.Selected(); node != nil && node.IsDir {
		t.expanded[node.Path] = true
		t.updateVisible()
	}
}

// Toggle toggles expand/collapse of current folder
func (t *TreePanel) Toggle() {
	if node := t.Selected(); node != nil && node.IsDir {
		if t.expanded[node.Path] {
			delete(t.expanded, node.Path)
		} else {
			t.expanded[node.Path] = true
		}
		t.updateVisible()
	}
}

// GoToTop moves to first item
func (t *TreePanel) GoToTop() {
	t.cursor = 0
	t.offset = 0
}

// GoToBottom moves to last item
func (t *TreePanel) GoToBottom() {
	t.cursor = len(t.visible) - 1
	t.ensureVisible()
}

// ExpandTo expands the tree to show and select a specific node
func (t *TreePanel) ExpandTo(node *model.Node) {
	if node == nil {
		return
	}

	// Build path from root to node
	var path []*model.Node
	for n := node; n != nil; n = n.Parent {
		path = append([]*model.Node{n}, path...)
	}

	// Expand each ancestor
	for _, n := range path {
		if n.IsDir {
			t.expanded[n.Path] = true
		}
	}

	// Update visible list
	t.updateVisible()

	// Find and select the node
	for i, n := range t.visible {
		if n == node {
			t.cursor = i
			t.ensureVisible()
			break
		}
	}
}

func (t *TreePanel) ensureVisible() {
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	maxVisible := t.height - 2 // account for borders
	if maxVisible < 1 {
		maxVisible = 1
	}
	if t.cursor >= t.offset+maxVisible {
		t.offset = t.cursor - maxVisible + 1
	}
}

func (t *TreePanel) updateVisible() {
	t.visible = nil
	if t.root == nil {
		return
	}
	t.collectVisible(t.root, 0)
}

func (t *TreePanel) collectVisible(node *model.Node, depth int) {
	t.visible = append(t.visible, node)

	if node.IsDir && t.expanded[node.Path] {
		// Sort children by size
		children := make([]*model.Node, len(node.Children))
		copy(children, node.Children)
		model.SortBySize(children)

		for _, child := range children {
			t.collectVisible(child, depth+1)
		}
	}
}

func (t TreePanel) getDepth(node *model.Node) int {
	depth := 0
	for p := node.Parent; p != nil; p = p.Parent {
		depth++
	}
	return depth
}

// RequiredWidth calculates the minimum width needed to display all visible content
func (t TreePanel) RequiredWidth() int {
	if t.root == nil || len(t.visible) == 0 {
		return 30
	}

	maxWidth := 0
	for _, node := range t.visible {
		// Build the line exactly as View() does, then measure display width
		line := t.buildLine(node)
		width := lipgloss.Width(line)
		if width > maxWidth {
			maxWidth = width
		}
	}

	// Add border width (2 for left+right)
	return maxWidth + 2
}

// lineContent holds the components of a tree line for rendering
type lineContent struct {
	prefix      string
	name        string
	deletedBadge string
	sizeBar     string
	size        string
	changeStr   string
}

// buildLineContent extracts the common line building logic
func (t TreePanel) buildLineContent(node *model.Node) lineContent {
	depth := t.getDepth(node)

	prefix := strings.Repeat("  ", depth)
	if node.IsDir {
		if t.expanded[node.Path] {
			prefix += "\u25bc " // down triangle
		} else {
			prefix += "\u25b6 " // right triangle
		}
	} else {
		prefix += "  "
	}

	name := node.Name
	size := FormatSize(node.TotalSize())

	// For deleted items, skip size (will show as delta)
	var deletedBadge string
	if node.IsDeleted {
		deletedBadge = " DEL"
		size = ""
	}

	// Size bar for directories
	var sizeBar string
	if node.IsDir && node.Parent != nil && node.Parent.TotalSize() > 0 {
		pct := float64(node.TotalSize()) / float64(node.Parent.TotalSize())
		barW := treeSizeBarWidth
		filledFloat := pct * float64(barW)
		filled := int(filledFloat)
		var bar strings.Builder
		for j := 0; j < barW; j++ {
			if j < filled {
				bar.WriteRune('█')
			} else if float64(j) < filledFloat+0.5 && filled < barW {
				bar.WriteRune('▓')
			} else {
				bar.WriteRune('░')
			}
		}
		sizeBar = "[" + bar.String() + "]"
	}

	// Change indicator
	var changeStr string
	if t.showDiff {
		if node.IsDeleted {
			// Deleted item - show its full size as freed
			changeStr = fmt.Sprintf("-%s", FormatSize(node.TotalSize()))
		} else if node.DeletedSize > 0 {
			// Contains deleted children - show accumulated freed size
			changeStr = fmt.Sprintf("-%s", FormatSize(node.DeletedSize))
		}
	}

	return lineContent{prefix, name, deletedBadge, sizeBar, size, changeStr}
}

// buildLine creates the text content for a node (for width calculation)
// Must match the styling applied in View() for accurate width measurement
func (t TreePanel) buildLine(node *model.Node) string {
	c := t.buildLineContent(node)

	// Apply same styling as View() for accurate width measurement
	deletedBadge := c.deletedBadge
	if node.IsDeleted && deletedBadge != "" {
		deletedBadge = " " + DeletedBadge.Render("DEL")
	}

	changeStr := c.changeStr
	if t.showDiff && changeStr != "" && node.IsDeleted {
		// Deleted items show the freed size
	}

	return fmt.Sprintf("%s%s%s %s %s %s", c.prefix, c.name, deletedBadge, c.sizeBar, c.size, changeStr)
}

// View renders the tree
func (t TreePanel) View() string {
	if t.root == nil {
		return TreePanelStyle.Width(t.width).Height(t.height).Render("No data")
	}

	var lines []string
	maxVisible := t.height - 2
	if maxVisible < 1 {
		maxVisible = 1
	}

	for i := t.offset; i < len(t.visible) && len(lines) < maxVisible; i++ {
		node := t.visible[i]
		c := t.buildLineContent(node)

		// Apply styles to components
		deletedBadge := c.deletedBadge
		if node.IsDeleted && deletedBadge != "" {
			deletedBadge = " " + DeletedBadge.Render("DEL")
		}

		changeStr := c.changeStr
		if t.showDiff && changeStr != "" {
			// Style the size change indicator
			changeStr = ShrunkStyle.Render(changeStr)
		}

		// Compose line
		line := fmt.Sprintf("%s%s%s %s %s %s", c.prefix, c.name, deletedBadge, c.sizeBar, c.size, changeStr)

		// Determine color based on node type and deletion state
		var itemStyle lipgloss.Style
		maxW := t.width - 2
		if i == t.cursor && t.focused {
			itemStyle = TreeItemSelected.Width(maxW).MaxWidth(maxW)
		} else if i == t.cursor && !t.focused {
			// Show dimmer selection when unfocused
			itemStyle = TreeItemSelectedUnfocused.Width(maxW).MaxWidth(maxW)
		} else if t.showDiff && node.IsDeleted {
			// Deleted item - red
			itemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).MaxWidth(maxW)
		} else if t.showDiff && node.DeletedSize > 0 {
			// Contains deleted children - purple
			itemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A855F7")).MaxWidth(maxW)
		} else if node.IsDir {
			// Directory: neon cyan
			itemStyle = lipgloss.NewStyle().Foreground(ColorDir).MaxWidth(maxW)
		} else {
			// File: dimmer
			itemStyle = lipgloss.NewStyle().Foreground(ColorFile).MaxWidth(maxW)
		}
		line = itemStyle.Render(line)

		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	style := TreePanelStyle.Width(t.width).Height(t.height)
	if t.focused {
		style = style.BorderForeground(ColorPrimary)
	}

	return style.Render(content)
}
