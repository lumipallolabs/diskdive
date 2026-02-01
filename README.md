# DiskDive

A fast, terminal-based disk space analyzer with treemap visualization.

![DiskDive Screenshot](https://placeholder-for-screenshot.png)

## Features

- **Fast parallel scanning** - Scans large drives quickly using concurrent workers
- **Treemap visualization** - See disk usage at a glance with proportional blocks
- **Real-time deletion tracking** - Monitors filesystem and shows freed space
- **Cross-platform** - Works on macOS, Windows, and Linux
- **Vim-style navigation** - Navigate with arrow keys or hjkl

## Installation

```bash
go install github.com/samuli/diskdive@latest
```

Or clone and build:

```bash
git clone https://github.com/samuli/diskdive.git
cd diskdive
go build .
```

## Usage

```bash
# Scan a drive (interactive drive selector)
diskdive

# Scan a specific directory
diskdive /path/to/directory
```

### macOS

**From Finder:** Double-click `DiskDive.app` - opens in Terminal automatically.

**From Terminal:** The binary is inside the app bundle:
```bash
# Run directly
/Applications/DiskDive.app/Contents/MacOS/diskdive

# Or create a symlink for convenience
ln -s /Applications/DiskDive.app/Contents/MacOS/diskdive /usr/local/bin/diskdive
```

### Keyboard Controls

#### Navigation
| Key | Action |
|-----|--------|
| `↑↓←→` or `hjkl` | Navigate |
| `PgUp/PgDn` | Scroll faster |
| `g/G` | Jump to top/bottom |
| `Tab` | Switch between tree and treemap panels |

#### Actions
| Key | Action |
|-----|--------|
| `Enter` | Expand/zoom into directory |
| `Esc` or `Backspace` | Go back / collapse |
| `Space` | Preview file (Quick Look on macOS) |
| `e` | Select different drive |
| `o` | Open in file manager |
| `r` | Rescan current drive |

#### Other
| Key | Action |
|-----|--------|
| `?` | Show help |
| `q` | Quit |

## Requirements

- macOS 12+ / Windows 10+ / Linux

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for build instructions, architecture overview, and contribution guidelines.
