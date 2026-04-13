# Pelorus — Todo

Captured after the session that landed: multi-select, F-key aliases, trash, sort modes, clipboard, git status glyphs, status bar redesign, Huh forms (delete + bulk rename), Dracula + Catppuccin + Omarchy themes, `--theme` flag, animated demo GIF.

---

## High priority

~~**Rename / new file / new dir → Huh forms**~~ ✓ done
~~**Filesystem watcher (auto-refresh)**~~ ✓ done (was already implemented with fsnotify + debounce)
~~**Preview search**~~ ✓ done (`/` opens inline search bar, `n`/`N` cycle matches, `Escape` closes)

---

## Medium priority

**Dirty directory glyphs**
Git status glyphs appear on files (`M`, `A`, `?`). Directories that contain modified or untracked children should show a subtle aggregate glyph (e.g. `~` or `·`). Computed from the existing `StatusMap` — any entry whose path is prefixed by the directory path.
Files: `internal/pane/pane.go` (`renderEntry`), `internal/gitstatus/gitstatus.go`

**Job queue → Bubbles progress bars**
The ops job queue renders hand-rolled progress bars. Replace with the Bubbles `progress` component for smooth animated fills and cleaner ETA display.
Files: `internal/ops/ops.go`, `internal/app/app.go` (job view render)

**Paginator for jump list / palette**
When the jump list or command palette has more results than fit vertically, the list clips with no indication. Add a Bubbles `paginator` or scroll indicator.
Files: `internal/palette/palette.go`, `internal/jump/jump.go`

---

## Low priority / polish

**Open in Finder / Reveal**
`ctrl+r` or palette action: reveal the item under cursor in macOS Finder. Local panes only — no-op gracefully on SFTP. `exec.Command("open", "-R", path)`.
Files: `internal/actions/builtin.go`, `internal/app/app.go`

**`pelorus://` URL scheme**
Register a custom URL scheme so Raycast, Spotlight scripts, Automator, and shell aliases can deep-link into Pelorus at a specific path. One-time system registration via a plist or `lsregister`. Unlocks integrations without any changes to other tools.
Files: `extras/` (installer script or plist), `cmd/root.go` (handle URL arg if needed)

**`extras/` macOS Quick Action**
A small `.workflow` file (plist + shell script) that adds "Open in Pelorus" to Finder's right-click Services menu. No Pelorus Go code changes. Ship as a downloadable file in `extras/open-in-pelorus.workflow`.

**Ambient directory awareness**
Subtle visual shift based on directory type: git repos get a slightly different accent hue in the border, recently-modified directories pulse briefly on entry, archives get a compressed visual texture. Complements the subaquatic aesthetic.

---

## Out there (backlog)

- **Temporal navigation** — browse filesystem as it was at a point in time via git history. `ctrl+t` time scrubber. Diffs highlighted inline.
- **Sonar map** — third panel mode showing frecency heat map as a sonar-ping visualization.
- **Collaborative pane** — live-mirror a pane from another Tailscale node.
- **Natural language command bar** — `:` palette accepts `find all jpegs modified this week` resolved via local LLM.
- **Directory pair suggestions** — learns frequent copy A→B pairs and pre-fills the other pane.
- **Undo log** — every destructive operation logged as an invertible event. `ctrl+z` undoes the last file operation.
- **Raycast extension** — surfaces Pelorus jump list directly in Raycast search; separate repo.

---

## Done

- Multi-select with `space` (select in place, no cursor move)
- F-key aliases: F4 (editor), F5 (copy), F6 (move), F7 (new dir), ⇧F7 (new file), F8 (trash), ⇧F8 (permanent delete)
- Trash action (macOS: AppleScript Finder; Linux: XDG spec; fallback: RemoveAll)
- Sort modes per pane (`s` cycles name → size → date → ext), indicator in path header
- Copy path / filename to clipboard (`y` / `Y`)
- Git status glyphs on files (`M` `A` `?` `D`), branch indicator in status bar
- Status bar redesign: breadcrumb · git branch · remote badge · perms + size
- Huh overlay for delete confirmation and bulk rename (`R`)
- Dracula theme
- Catppuccin Mocha theme (was: omarchy static)
- Omarchy dynamic theme (reads `~/.config/omarchy/current/theme/colors.toml`, auto-detects light/dark)
- `--theme` / `-t` CLI flag
- Animated demo GIF (`assets/pelorus.gif`) recorded with VHS, embedded in README
- Header/footer deduplication: path breadcrumb lives in footer only; header shows branding + hints
