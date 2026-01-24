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

// buildLine creates the text content for a node (same logic as View)
func (t TreePanel) buildLine(node *model.Node) string {
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

	var sizeBar string
	if node.IsDir && node.Parent != nil && node.Parent.TotalSize() > 0 {
		pct := float64(node.TotalSize()) / float64(node.Parent.TotalSize())
		barW := treeSizeBarWidth
		filled := int(pct * float64(barW))
		sizeBar = "[" + strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", barW-filled) + "]"
	}

	var changeStr string
	if t.showDiff {
		if node.IsNew {
			changeStr = "NEW"
		} else if change := node.SizeChange(); change != 0 {
			if change > 0 {
				changeStr = fmt.Sprintf("+%s", FormatSize(change))
			} else {
				changeStr = FormatSize(change)
			}
		}
	}

	return fmt.Sprintf("%s%s %s %s %s", prefix, name, sizeBar, size, changeStr)
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
		depth := t.getDepth(node)

		// Build prefix
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

		// Name and size
		name := node.Name
		size := FormatSize(node.TotalSize())

		// Size bar for directories
		var sizeBar string
		if node.IsDir && node.Parent != nil && node.Parent.TotalSize() > 0 {
			pct := float64(node.TotalSize()) / float64(node.Parent.TotalSize())
			barW := treeSizeBarWidth
			filled := int(pct * float64(barW))
			sizeBar = "[" + strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", barW-filled) + "]"
		}

		// Change indicator
		var changeStr string
		if t.showDiff {
			if node.IsNew {
				changeStr = NewBadge.Render("NEW")
			} else if change := node.SizeChange(); change != 0 {
				if change > 0 {
					changeStr = GrewStyle.Render(fmt.Sprintf("+%s", FormatSize(change)))
				} else {
					changeStr = ShrunkStyle.Render(FormatSize(change))
				}
			}
		}

		// Compose line
		line := fmt.Sprintf("%s%s %s %s %s", prefix, name, sizeBar, size, changeStr)

		// Determine color based on node type and diff state (matching treemap colors)
		var itemStyle lipgloss.Style
		if i == t.cursor && t.focused {
			itemStyle = TreeItemSelected.Width(t.width - 2)
		} else if t.showDiff && node.IsNew {
			itemStyle = lipgloss.NewStyle().Foreground(ColorNew).Width(t.width - 2)
		} else if t.showDiff && node.SizeChange() > 0 {
			itemStyle = lipgloss.NewStyle().Foreground(ColorGrew).Width(t.width - 2)
		} else if t.showDiff && node.SizeChange() < 0 {
			itemStyle = lipgloss.NewStyle().Foreground(ColorShrunk).Width(t.width - 2)
		} else if node.IsDir {
			// Directory: light blue (matches treemap)
			itemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7DD3FC")).Width(t.width - 2)
		} else {
			// File: light grey (matches treemap)
			itemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E4E4E7")).Width(t.width - 2)
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
