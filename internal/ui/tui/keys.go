package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keyboard shortcuts
type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Left         key.Binding
	Right        key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	Top          key.Binding
	Bottom       key.Binding
	Tab          key.Binding
	Enter        key.Binding
	Back         key.Binding
	Rescan       key.Binding
	Help         key.Binding
	Quit         key.Binding
	SelectDrive  key.Binding
	OpenExplorer key.Binding
	Preview      key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "collapse"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "expand"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("PgUp", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("PgDn", "page down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G", "bottom"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab", "shift+tab"),
			key.WithHelp("tab", "switch panel"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "zoom in"),
		),
		Back: key.NewBinding(
			key.WithKeys("backspace", "esc"),
			key.WithHelp("esc/⌫", "back"),
		),
		Rescan: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "rescan"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		SelectDrive: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "select drive"),
		),
		OpenExplorer: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open in file manager"),
		),
		Preview: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("Space", "preview"),
		),
	}
}

// ShortHelp returns a brief help string
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Left, k.Right, k.Enter, k.Quit}
}

// FullHelp returns all help bindings
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Top, k.Bottom, k.Tab},
		{k.Enter, k.Back},
		{k.Rescan},
		{k.Help, k.Quit},
	}
}
