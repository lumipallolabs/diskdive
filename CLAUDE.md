# Development Guidelines

Guidelines for contributing to DiskDive.

## Platform Support

All features must work on macOS 12+, Windows 10+, and modern Linux distributions. Use build tags (`_darwin.go`, `_windows.go`, `_other.go`) for platform-specific implementations.

## Architecture

```
internal/
  core/       # Pure business logic - no UI dependencies
    controller.go   # Main application controller
    state.go        # State types (ScanState, FreedState)
    events.go       # Event types for UI communication

  ui/tui/     # Terminal UI (Bubble Tea + Lipgloss)
    app.go          # Main TUI application
    tree.go         # Tree panel component
    treemap.go      # Treemap visualization
    styles.go       # Colors and styles

  scanner/    # Filesystem scanning
  model/      # Data structures (Node, Drive)
  watcher/    # Filesystem change monitoring
  stats/      # Usage statistics persistence
```

The `core` package contains all business logic and can be used by alternative frontends (GUI, web, etc.).

## Code Quality

### Principles

- **Keep it simple.** Write the minimum code that solves the problem.
- **No duplication.** Extract shared logic into functions. DRY.
- **Single responsibility.** Each file/struct/function has one clear purpose.
- **Readable over clever.** Clarity beats brevity.
- **Fix root causes.** Don't add workarounds that mask bugs.

### Go Conventions

- Use existing solutions before writing custom code (stdlib, lipgloss, bubbletea)
- Prefer specific parameters over passing entire structs
- Use pointer receivers for methods that modify state
- Value receivers for read-only methods

### TUI Guidelines

- Use lipgloss for all styling and layout
- Use `lipgloss.Width()` for measuring rendered text width
- Use `lipgloss.Place()`/`JoinHorizontal()`/`JoinVertical()` for layout
- Only use `strings.Repeat()` for visual elements (progress bars), not layout spacing

### Treemap Rules

- All blocks must be large enough to show a label (minimum 8×3 characters)
- The "N more items" block must always be visible when items are grouped
- Show as many items as possible while maintaining readability

## Development

### Building

```bash
go build .
go test ./...
```

### Running

```bash
# Scan current directory
go run . ./

# Scan specific path
go run . /path/to/scan

# With debug logging
DEBUG=1 go run . ./
```

### Testing Changes

After making changes, verify:
1. `go build .` succeeds
2. `go test ./...` passes
3. Manual testing with both small directories and full drives
