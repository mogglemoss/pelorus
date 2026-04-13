# Pelorus — Todo

Captured after the session that landed: multi-select, F-key aliases, trash, sort modes, clipboard, git status glyphs, status bar redesign, Huh forms (delete + bulk rename), Dracula + Catppuccin + Omarchy themes, `--theme` flag, animated demo GIF.

---

## High priority

~~**Rename / new file / new dir → Huh forms**~~ ✓ done
~~**Filesystem watcher (auto-refresh)**~~ ✓ done (was already implemented with fsnotify + debounce)
~~**Preview search**~~ ✓ done (`/` opens inline search bar, `n`/`N` cycle matches, `Escape` closes)

---

## Medium priority

~~**Dirty directory glyphs**~~ ✓ done (`~` glyph on dirs with modified/untracked children)
~~**Job queue → Bubbles progress bars**~~ ✓ done (was already using Bubbles `progress` with gradient fills)
~~**Paginator for jump list / palette**~~ ✓ done (`N/M` counter in footer of both overlays)

---

## Low priority / polish

~~**Open in Finder / Reveal**~~ ✓ done (`ctrl+r` / palette; `open -R path`; no-op on remote panes)
~~**`pelorus://` URL scheme**~~ ✓ done (`extras/install-url-scheme.sh` registers AppleScript handler; `cmd/root.go` strips scheme prefix from CLI args)
~~**`extras/` macOS Quick Action**~~ ✓ done (`extras/Open in Pelorus.workflow` — copy to `~/Library/Services/` to install)
~~**Ambient directory awareness**~~ ✓ done (path header: amber in archives, soft green in git repos, default cyan elsewhere)

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
