# pelorus

![License: MIT](https://img.shields.io/badge/license-MIT-pink.svg)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8.svg)
[![Built with Charm](https://img.shields.io/badge/built_with-Charm-ff69b4.svg)](https://charm.sh)

*A file manager with opinions.*

Dual-pane TUI file manager. Local filesystem. SFTP remotes. Tailscale nodes. Archives as directories. Fuzzy everywhere. Zero configuration required to be useful.

---

![pelorus](./assets/pelorus.gif)

---

## Features

**Dual pane**
- Left and right panes, always visible — not a mode, not a toggle, just there
- `tab` to switch; `C` to copy across; `m` to move across
- Background job queue (`J`) with animated progress bars and speed/ETA — operations never block the UI

**Navigation**
- `j` / `k` / `h` / `l` — you know what these do
- Type in any pane to fuzzy-filter its contents live
- `g` opens the jump list — frecency-ranked, fuzzy-searchable, persistent
- `B` to pin the current directory; `~` to go home; `ctrl+l` to type a path directly

**Preview pane**
- `p` toggles a third panel: syntax-highlighted code via Chroma, rendered Markdown via Glamour, file metadata for everything else
- Scrollable with `]` / `[`
- Loads asynchronously — the pane spinner tells you it's working

**Command palette**
- `ctrl+p` or `:` — fuzzy-searches every action, built-in and custom
- Context-aware: only shows what's valid for the current selection
- Recently used actions float to the top

**Archives as directories**
- Press `l` on a `.zip`, `.tar.gz`, `.tar.bz2`, `.tar.xz`, or `.tar` — it opens as a directory
- Navigate in, copy files out; `h` at the root returns to the real filesystem
- Status bar shows `[zip]` or `[tar.gz]` so you know where you are

**Remote connections**
- `c` opens the connect palette — parses `~/.ssh/config` automatically
- Tailscale nodes appear in a second section, fetched live from the local socket, with `●` online / `○` offline indicators
- Connecting replaces the inactive pane with an SFTP session; all file operations work the same

**Custom actions**
- Any shell command is a first-class action
- `{path}`, `{name}`, `{dir}` template variables
- Appears in the command palette, inherits context filtering, bindable to any key

**Theming**
- Built-in *retrofuture subaquatic* theme — bioluminescent teal, ocean dark, phosphor cyan
- Three additional themes: `gruvbox`, `nord`, `light`
- Set in config; switch without restarting

---

## Installation

### go install

```bash
go install github.com/mogglemoss/pelorus@latest
```

### From source

```bash
git clone https://github.com/mogglemoss/pelorus
cd pelorus
go build -o pelorus .
./pelorus .
```

No CGO. No runtime dependencies. One binary.

---

## Usage

```
pelorus [path] [flags]

  path              Directory to open (default: current directory)

  -f, --config      Config file path (default: XDG config dir)
      --version     Print version and exit
```

On first run, pelorus writes a fully-commented config file to the XDG config directory (`~/.config/pelorus/config.toml` on Linux, `~/Library/Application Support/pelorus/config.toml` on macOS). Every option is present and explained. Reading it is optional.

---

## Key Bindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `h` / `←` | Go to parent |
| `l` / `→` / `enter` | Enter directory / open archive |
| `tab` | Switch active pane |
| `ctrl+p` / `:` | Command palette |
| `g` | Jump list |
| `c` | Connect to SSH / Tailscale host |
| `p` | Toggle preview pane |
| `J` | Job queue |
| `?` | Keybinding reference |
| `~` | Go to home directory |
| `ctrl+l` | Go to path (type any path) |
| `.` | Toggle hidden files |
| `B` | Bookmark current directory |
| `C` | Copy selected to other pane |
| `m` | Move selected to other pane |
| `d` | Delete (with confirmation) |
| `r` | Rename |
| `n` | New file |
| `N` | New directory |
| `]` / `[` | Scroll preview down / up |
| `q` | Quit |

All keybindings are overridable in config.

---

## Custom Actions

```toml
[[actions.custom]]
id = "custom.open-zed"
name = "Open in Zed"
description = "Open selected file in Zed editor"
category = "Custom"
command = "zed {path}"
context = "always"

[[actions.custom]]
id = "custom.copy-path"
name = "Copy Path"
description = "Copy selected path to clipboard"
category = "Custom"
command = "echo {path} | pbcopy"
context = "file"
```

Template variables: `{path}` full path, `{name}` filename, `{dir}` containing directory. Commands run via `sh -c`. Custom actions appear in the palette and can be bound to any key.

---

## Technical Specifications

| Parameter | Value |
|-----------|-------|
| UI framework | [Bubbletea](https://github.com/charmbracelet/bubbletea) |
| Styling | [Lipgloss](https://github.com/charmbracelet/lipgloss) |
| Syntax highlighting | [Chroma](https://github.com/alecthomas/chroma) |
| Markdown rendering | [Glamour](https://github.com/charmbracelet/glamour) |
| Fuzzy matching | [sahilm/fuzzy](https://github.com/sahilm/fuzzy) — in-process, no fzf binary |
| SFTP | [pkg/sftp](https://github.com/pkg/sftp) + [x/crypto/ssh](https://golang.org/x/crypto) |
| Tailscale | [tailscale.com/client/tailscale](https://pkg.go.dev/tailscale.com/client/tailscale) local socket |
| Config | TOML via [BurntSushi/toml](https://github.com/BurntSushi/toml) |
| Archive formats | `.zip` · `.tar` · `.tar.gz` · `.tar.bz2` · `.tar.xz` — pure Go |
| Jump list | Frecency scoring · XDG data dir · JSON |
| CGO | Disabled. One static binary. |
| Platforms | macOS · Linux (Tier 1) · Windows (Tier 2) |

---

## Acknowledgments

Pelorus steals thoughtfully from:

| Source | What we took |
|--------|-------------|
| [Marta](https://marta.sh) | Action palette as spine, dual pane as default, archive-as-directory, job queue |
| [lf](https://github.com/gokcehan/lf) | Async IO pattern, nav/eval/ui separation |
| [yazi](https://github.com/sxyazi/yazi) | Fuzzy-everywhere as interaction model, preview depth |
| [zoxide](https://github.com/ajeetdsouza/zoxide) | Auto-ranked jump list |
| [UploadThing](https://uploadthing.com) | The attitude — opinionated defaults, zero config to useful |

---

## License

MIT. See [LICENSE](./LICENSE).
