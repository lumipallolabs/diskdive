package ui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keyboard shortcuts
type KeyMap struct {
	Up          key.Binding
	Down        key.Binding
	Left        key.Binding
	Right       key.Binding
	Top         key.Binding
	Bottom      key.Binding
	Tab         key.Binding
	Enter       key.Binding
	Back        key.Binding
	Rescan      key.Binding
	ToggleDiff  key.Binding
	CycleSort   key.Binding
	Help        key.Binding
	Quit        key.Binding
	Drive1      key.Binding
	Drive2      key.Binding
	Drive3      key.Binding
	Drive4      key.Binding
	Drive5      key.Binding
	Drive6      key.Binding
	Drive7      key.Binding
	Drive8      key.Binding
	Drive9      key.Binding
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
		Top: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G", "bottom"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch panel"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "zoom in"),
		),
		Back: key.NewBinding(
			key.WithKeys("backspace"),
			key.WithHelp("backspace", "zoom out"),
		),
		Rescan: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "rescan"),
		),
		ToggleDiff: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "toggle diff"),
		),
		CycleSort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "cycle sort"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Drive1: key.NewBinding(key.WithKeys("1")),
		Drive2: key.NewBinding(key.WithKeys("2")),
		Drive3: key.NewBinding(key.WithKeys("3")),
		Drive4: key.NewBinding(key.WithKeys("4")),
		Drive5: key.NewBinding(key.WithKeys("5")),
		Drive6: key.NewBinding(key.WithKeys("6")),
		Drive7: key.NewBinding(key.WithKeys("7")),
		Drive8: key.NewBinding(key.WithKeys("8")),
		Drive9: key.NewBinding(key.WithKeys("9")),
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
		{k.Rescan, k.ToggleDiff, k.CycleSort},
		{k.Help, k.Quit},
	}
}
