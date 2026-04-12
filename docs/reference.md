# Pelorus — Project Reference Document
> For use with Claude Code. This document defines the vision, architecture, constraints, and roadmap for Pelorus, an opinionated TUI file manager built with Go and the Charm/Bubbletea ecosystem.

---

## 1. Project Identity

**Name:** Pelorus  
**Codename:** Pelorus (final)  
**Binary:** `pelorus`  
**Module path:** `github.com/mogglemoss/pelorus`  
**Tagline:** *A file manager with opinions.*  
**Aesthetic:** Retrofuture subaquatic deadpan — cohesive with Deep Sea Sleeper, lazytailscale, fathom, Possum  
**Author:** (`mogglemoss` on GitHub)

---

## 2. Core Philosophy

Pelorus is inspired by Marta (macOS) but lives in the terminal. The guiding principles:

- **Zero config to useful.** Works great out of the box. Config exists to tune, not to bootstrap.
- **Everything is an action.** Keybindings are shortcuts to named actions. The palette surfaces all of them.
- **Fuzzy everywhere.** Any list — files, actions, hosts, bookmarks — is fuzzy-searchable.
- **Dual pane always.** Not a mode. The default.
- **Providers, not assumptions.** The filesystem is abstracted. Local, SFTP, and future providers all look the same to the UI.
- **Async always.** File operations never block the UI. Ever.

---

## 3. Technology Stack

| Concern | Library |
|---|---|
| TUI framework | `charm.land/bubbletea/v2` |
| Layout & styling | `github.com/charmbracelet/lipgloss` |
| Components | `github.com/charmbracelet/bubbles` |
| Syntax highlighting | `github.com/alecthomas/chroma/v2` |
| Markdown rendering | `github.com/charmbracelet/glamour` |
| Image rendering | `github.com/disintegration/imaging` |
| Fuzzy matching | `github.com/sahilm/fuzzy` (pure Go, no binary dep) |
| SFTP/SSH | `github.com/pkg/sftp` + `golang.org/x/crypto/ssh` |
| Config | TOML via `github.com/BurntSushi/toml` |
| CLI entry | `github.com/spf13/cobra` |
| Clipboard | `github.com/atotto/clipboard` |
| Tailscale API | `tailscale.com/client/tailscale` (local socket) |
| Archive support | `github.com/mholt/archiver/v3` |
| XDG paths | `github.com/adrg/xdg` |

**Do NOT shell out to fzf.** Fuzzy matching is handled in-process via `sahilm/fuzzy`.  
**Do NOT shell out to external tools for core ops.** File operations are native Go.  
**Optionally shell out to `rg` (ripgrep) for content search** — degrade gracefully if absent.

---

## 4. Platform Support

### Tier 1 — macOS + Linux
Full feature set:
- All file operations including trash (platform-appropriate)
- Symlinks: displayed with indicator, navigated transparently
- Unix permissions display
- Image preview (sixel / chafa / fallback ASCII)
- Tailscale integration
- Full theme support
- Tested in CI (GitHub Actions)

### Tier 2 — Windows (native)
Best effort:
- Core navigation and file operations
- No image preview (documented, graceful degradation — no crash)
- Delete with confirmation instead of Recycle Bin (v1), Recycle Bin integration later
- Symlinks displayed, creation not supported
- Windows Terminal only — no guarantee on legacy console
- Community-supported, not primary dev target
- Path normalization handled automatically via `filepath` package

### Tier 2 — Windows as SSH/SFTP remote
Fully supported — this is just the SFTP provider. Path normalization at the provider boundary handles differences transparently. Works from macOS or Linux client connecting to Windows OpenSSH.

---

## 5. Project Structure

```
pelorus/
├── main.go
├── cmd/
│   └── root.go                  # Cobra entry, flags, startup
├── internal/
│   ├── app/
│   │   └── app.go               # Root Bubbletea model — owns layout, routes focus
│   ├── pane/
│   │   └── pane.go              # Single pane model (reused for left + right)
│   ├── nav/
│   │   └── nav.go               # Directory reading, sorting, filtering, history
│   ├── actions/
│   │   ├── registry.go          # Action registry — all ops registered here
│   │   └── builtin.go           # Built-in actions (copy, move, delete, etc.)
│   ├── palette/
│   │   └── palette.go           # Command palette Bubbletea model
│   ├── ops/
│   │   ├── ops.go               # File operation orchestration
│   │   └── queue.go             # Background job queue, progress tracking
│   ├── provider/
│   │   ├── provider.go          # Provider interface definition
│   │   ├── local/
│   │   │   └── local.go         # Local filesystem provider
│   │   └── sftp/
│   │       └── sftp.go          # SFTP provider (SSH remotes, homelab, Windows)
│   ├── config/
│   │   ├── config.go            # Config loading, defaults, validation
│   │   └── defaults.go          # Sensible out-of-box defaults
│   ├── preview/
│   │   └── preview.go           # Preview pane: Chroma, Glamour, image, archive
│   ├── theme/
│   │   └── theme.go             # Lipgloss styles, theme registry
│   ├── jump/
│   │   └── jump.go              # Bookmark + auto-ranked jump list (zoxide-style)
│   └── connect/
│       └── connect.go           # Connection palette: SSH config + Tailscale nodes
└── pkg/
    └── fileinfo/
        └── fileinfo.go          # File metadata, type detection, icon mapping
```

---

## 6. The Provider Interface

This is the load-bearing abstraction. All UI code operates on providers — never directly on the filesystem.

```go
type Provider interface {
    List(path string) ([]FileInfo, error)
    Stat(path string) (FileInfo, error)
    Read(path string) (io.ReadCloser, error)

    Copy(src, dst string) error
    Move(src, dst string) error
    Delete(path string) error
    MakeDir(path string) error
    Rename(src, dst string) error

    Capabilities() Caps
    String() string  // human-readable label e.g. "local" or "nautilus (SSH)"
}

type Caps struct {
    CanSetPermissions bool  // false on Windows, S3
    CanSymlink        bool  // false on Windows SFTP, S3
    CanPreview        bool  // may be false on slow remotes
    CanTrash          bool  // false on remotes
    IsRemote          bool
    SupportsArchive   bool
}
```

**Cross-provider ops:** Copy/move between providers (e.g. local left pane → SFTP right pane) is handled by the `ops` layer, which streams data rather than assuming same-filesystem semantics.

**Future providers** (v2+): S3-compatible (AWS, Backblaze B2, Cloudflare R2, MinIO), Google Drive. The interface is designed for them — don't implement them in v1.

---

## 7. The Action System

Everything flows through named actions. This is the spine of the application.

```go
type Action struct {
    ID          string          // e.g. "file.copy", "nav.jump", "view.toggle-hidden"
    Name        string          // Display name in palette
    Description string          // Shown in palette detail
    Category    string          // Groups in palette: "File", "Navigate", "View", etc.
    Handler     ActionFunc      // The actual implementation
    Context     ActionContext   // When this action is available
    Keybinding  string          // Default binding (overridable in config)
}

type ActionContext uint32
const (
    CtxAlways      ActionContext = iota
    CtxFileSelected
    CtxDirSelected
    CtxRemote
    CtxLocal
)
```

**Keybindings are aliases for action IDs.** In config:
```toml
[keybindings]
"p" = "file.copy-path"
"ctrl+n" = "file.new"
```

**Custom actions are shell commands registered as actions:**
```toml
[[actions.custom]]
id = "custom.open-in-zed"
name = "Open in Zed"
category = "Custom"
command = "zed {path}"
context = "always"
```

---

## 8. The Command Palette

Invoked with `:` or `ctrl+p` (configurable). Fuzzy-searches all registered actions. Context-aware — only shows actions valid for the current selection.

- Displays: action name, keybinding hint, category
- Fuzzy matching via `sahilm/fuzzy` — scored, ranked, typo-tolerant
- Remembers recently used actions (float to top)
- Custom actions appear alongside built-ins — no second-class citizens

---

## 9. Fuzzy Everywhere

`sahilm/fuzzy` is used in every list interaction:

| Surface | Trigger |
|---|---|
| File pane filtering | Type while in pane |
| Command palette | `:` or `ctrl+p` |
| Jump list | `g` |
| Connect palette | `c` |
| Action search | Within palette |
| Content search (v1.1) | `?` — shells to `rg` if present |

**No dependency on the `fzf` binary.** All fuzzy behavior is in-process.

---

## 10. Tailscale Integration

### Passive (v1 — zero effort)
SSH config hosts with Tailscale MagicDNS names appear in the connect palette automatically. `~/.ssh/config` is parsed on startup.

### Active (v1.1 — differentiating feature)
Pelorus queries the local Tailscale socket (`/var/run/tailscale/tailscaled.sock` on Linux, platform-appropriate on macOS) via `tailscale.com/client/tailscale` LocalClient API.

Connect palette shows two sections:
```
SSH Config Hosts          Tailscale Nodes
─────────────────         ────────────────────────────
  dev-server              ● nautilus    macOS   online
  backup-nas              ● mollusk     Linux   online
                          ○ mag-pi      Linux   offline
```

- Tailscale nodes connect via SFTP (not raw shell)
- Online/offline status shown with indicators
- Node OS shown (from Tailscale peer info)
- Degrade gracefully if `tailscaled` not running — SSH config section still works

**Prior art:** `lazytailscale` (`mogglemoss`) — reuse Tailscale API familiarity.

---

## 11. Symlinks

Symlinks are transparent. Navigation follows symlinks without requiring explicit user action.

Display:
- Symlink indicator in file listing (e.g. `→` or distinct icon/color)
- Shows target path in status bar / preview area
- Broken symlinks shown with distinct broken indicator (red, different icon)
- Symlink target resolved in status bar on hover/selection

Operations:
- Navigate into symlinked directories transparently
- File operations (copy/move/delete) operate on the link itself by default
- Config option to follow vs. operate on link for destructive ops

---

## 12. Archive Support

Archives are navigated as directories. Powered by `mholt/archiver`.

Supported formats: `.zip`, `.tar`, `.tar.gz`, `.tar.bz2`, `.tar.xz`, `.7z`, `.rar`  
Read: all formats  
Write: `.zip` and `.tar.gz` in v1, others in v1.1

Behavior:
- Entering an archive opens it as a virtual directory
- Status bar indicates depth: `~/Documents/archive.zip/subdir/`
- File ops within archives work (extract = copy out)
- Nested archives: navigate into, but no write ops on nested

---

## 13. Background Operations & Job Queue

File ops run as Bubbletea `Cmd` goroutines — never blocking the UI.

```
[ Jobs ]  ─────────────────────────────────────────
  ✓  Copy  report.pdf → /backup/         done
  ↑  Move  photos/ → /archive/           87%  ████████░░
  ⏸  Copy  video.mp4 → nautilus:/media/  paused
```

- Inspectable with `j` (configurable)
- Pauseable / resumeable
- Cross-provider transfers show speed and ETA
- Errors surface in the job queue, not modal dialogs

---

## 14. Jump List

Persistent, ranked directory bookmark system. Inspired by `zoxide` but native to Pelorus.

- Automatic: directories visited increment their score
- Manual: `B` to bookmark current directory explicitly
- Access: `g` opens fuzzy-searchable jump list
- Stored in XDG data dir (`~/.local/share/pelorus/jumps.db` on Linux)
- Named bookmarks supported: `bookmark-as "project root"`

---

## 15. Configuration

Location (XDG-compliant):
- Linux: `~/.config/pelorus/config.toml`
- macOS: `~/Library/Application Support/pelorus/config.toml`
- Windows: `%APPDATA%\pelorus\config.toml`

Generated on first run with all defaults commented in. Config is documentation.

```toml
[general]
start_dir = "."           # or "last" to restore previous session
show_hidden = false
confirm_delete = true
editor = ""               # falls back to $EDITOR

[layout]
ratio = "1:1"             # left:right pane ratio
show_preview = true
preview_width = 40        # percent when preview panel open

[theme]
name = "pelorus"          # built-in: pelorus, gruvbox, nord, light

[keybindings]
palette = "ctrl+p"
jump = "g"
connect = "c"
jobs = "j"
toggle_hidden = "."
# Full action ID overrides:
# "p" = "file.copy-path"

[[actions.custom]]
id = "custom.open-zed"
name = "Open in Zed"
command = "zed {path}"
category = "Custom"
```

---

## 16. Themes

Four built-in themes ship with Pelorus. All implemented via lipgloss.

| Theme | Description |
|---|---|
| `pelorus` | **Default.** Retrofuture subaquatic — deep teals, bioluminescent accents, dark background. This is the identity theme. |
| `gruvbox` | Warm retro palette |
| `nord` | Cool arctic palette |
| `light` | Light background for daylight use |

The default `pelorus` theme is designed to be screenshottable. It is the face of the project.

---

## 17. Default Keybindings

| Key | Action |
|---|---|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `h` / `←` | Go to parent |
| `l` / `→` / `enter` | Enter directory / open file |
| `tab` | Switch active pane |
| `ctrl+p` or `:` | Open command palette |
| `g` | Open jump list |
| `c` | Open connect palette |
| `j` | Open job queue |
| `space` | Select/deselect item |
| `y` | Copy selected to clipboard (as path) |
| `d` | Delete (with confirmation) |
| `r` | Rename |
| `n` | New file |
| `N` | New directory |
| `.` | Toggle hidden files |
| `?` | Help / keybinding reference |
| `q` | Quit |

All keybindings overridable in config.

---

## 18. Walking Skeleton — v0.1 Goals

The first working version should demonstrate the core architecture cleanly. Scope:

- [ ] Dual pane layout, lipgloss-styled with `pelorus` theme
- [ ] Local filesystem provider working
- [ ] Keyboard navigation (j/k/h/l + arrow keys)
- [ ] Tab to switch panes
- [ ] Basic file ops: copy, move, delete, rename, new file, new dir
- [ ] Action registry wired up (even if only 10 actions)
- [ ] Command palette opening and fuzzy-filtering actions
- [ ] Config file loading with defaults
- [ ] Status bar showing path, file count, permissions
- [ ] `pelorus .` and `pelorus ~/path` as entry points

Everything else — SFTP, Tailscale, archives, jump list, preview — comes after the skeleton is solid.

---

## 19. v1 Roadmap

| Version | Focus |
|---|---|
| v0.1 | Walking skeleton: dual pane, local provider, palette, basic ops |
| v0.2 | Preview pane (Chroma syntax highlight, Glamour markdown, image) |
| v0.3 | Jump list + bookmarks |
| v0.4 | Archive-as-directory navigation |
| v0.5 | Background job queue with progress UI |
| v0.6 | SFTP provider + connect palette (SSH config) |
| v0.7 | Tailscale active integration |
| v0.8 | Full config DSL + custom actions |
| v0.9 | Windows tier 2 pass + CI |
| v1.0 | Polish, README, demo gif, AUR package |

---

## 20. What We're Stealing (Acknowledgments)

| Source | What we took |
|---|---|
| **Marta** | Action palette as spine, dual pane as default, archive-as-directory, opinionated themes, unified job queue |
| **lf** | Async IO pattern, modular architecture (nav/eval/ui separation), config philosophy |
| **fm (mistakenelf)** | Dependency choices: Chroma, Glamour, imaging, clipboard |
| **yazi** | Fuzzy-everywhere as interaction model, preview pane depth |
| **UploadThing** | Opinionated defaults attitude — zero config to useful |
| **lazytailscale** | Tailscale API familiarity (same author) |
| **zoxide** | Auto-ranked jump list model |

---

## 21. What We're NOT Building in v1

- Plugin API with compiled extensions (shell command actions cover this)
- S3 / Google Drive / Dropbox providers
- Content search (v1.1, shells to `rg`)
- Mouse-first interactions
- Built-in terminal emulator
- Git integration (action palette can call `git` commands)
- Tabs (dual pane + jump list covers the use case)

---

## 22. Repository & Publishing

- GitHub: `github.com/mogglemoss/pelorus`
- License: MIT
- AUR package: `pelorus-bin` (target for v1.0)
- Homebrew tap: `mogglemoss/tap/pelorus` (target for v1.0)
- Install: `go install github.com/mogglemoss/pelorus@latest`

---

*Document version: 1.0 — April 2026*  
*Next step: scaffold v0.1 walking skeleton*
