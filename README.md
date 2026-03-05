# REST Helper

A terminal-based REST API client built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- **URL bar** with method selector (GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS)
- **Request body** editor with JSON and YAML support (auto-converts YAML to JSON on send)
- **Request headers** editor with key-value pair input
- **Auth** tab with Bearer / Basic token support and visibility toggle
- **Response viewer** with syntax-highlighted JSON/YAML display, wrap/scroll modes, and horizontal scrolling
- **Field picker** for copying individual JSON fields from the response
- **History sidebar** with selection, batch delete, deduplication, and one-click restore
- **Full keyboard and mouse support** including clickable tabs, scrollbar dragging, and mouse wheel
- **Persistent storage** via SQLite (pure Go, no CGo required)
- **Minimum terminal size**: 80x24

## Installation

Requires Go 1.25+.

```sh
make build
```

This produces a `rest-helper` binary with the version embedded from `git describe`.

## Usage

```sh
./rest-helper
```

### Keyboard Shortcuts

#### General

| Key | Action |
|-----|--------|
| Ctrl+S | Send request |
| Tab / Shift+Tab | Switch panel focus |
| Alt+U | URL bar |
| Alt+B | Request Body |
| Alt+E | Request Headers |
| Alt+A | Request Auth |
| Alt+H | History |
| Alt+R | Response Body |
| Alt+D | Response Headers |
| ? / F1 | Toggle help |
| Ctrl+C / Ctrl+Q | Quit |

#### URL Bar

| Key | Action |
|-----|--------|
| Ctrl+P | Open method selector |
| Up/Down, Enter | Select method |

#### Request Body

| Key | Action |
|-----|--------|
| Ctrl+T | Toggle JSON/YAML format |

#### Request Headers

| Key | Action |
|-----|--------|
| Up/Down | Navigate rows |
| Left/Right | Move cursor / switch column |
| Enter | New row |
| Ctrl+D | Delete row |

#### Auth

| Key | Action |
|-----|--------|
| Ctrl+E | Toggle token visibility |

#### History Sidebar

| Key | Action |
|-----|--------|
| j/k or Up/Down | Navigate |
| Enter | Load entry |
| Space | Toggle select |
| d | Delete selected/single |
| D | Delete older |
| Ctrl+D | Clear all |
| Ctrl+X | Remove duplicates |
| Esc | Clear selection |

#### Response

| Key | Action |
|-----|--------|
| Tab | Switch Body/Headers |
| Up/Down | Scroll |
| PgUp/PgDn | Page scroll |
| Left/Right | Horizontal scroll (in scroll mode) |
| Ctrl+T | Toggle JSON/YAML view |
| Ctrl+W | Toggle wrap/scroll |
| y | Open field picker (copy) |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `REST_HELPER_DATA_DIR` | Override the database storage directory (default: `~/.local/share/rest-helper/`) |
| `HTTP_PROXY` / `HTTPS_PROXY` | Proxy server for HTTP/HTTPS requests (Go standard behavior) |
| `NO_PROXY` | Comma-separated list of hosts to exclude from proxying |

## Tech Stack

- [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Bubbles v2](https://github.com/charmbracelet/bubbles) - TUI components
- [Lip Gloss v2](https://github.com/charmbracelet/lipgloss) - Layout and styling with Compositor/Layer
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) - Pure Go SQLite driver

## License

MIT License - see [LICENSE](LICENSE) for details.
