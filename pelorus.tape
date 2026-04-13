# pelorus demo
# Run with: vhs pelorus.tape
#
# Navigation uses arrow keys so j/k/h/l can't fall into typeahead filter.
# Cursor positions are always reset to known state before counting moves.

Output assets/pelorus.gif

Set Shell "bash"
Set FontSize 14
Set FontFamily "JetBrainsMono Nerd Font"
Set Width 1400
Set Height 800
Set Theme "GruvboxDark"
Set PlaybackSpeed 1.0
Set TypingSpeed 80ms

# ── Launch ───────────────────────────────────────────────────────────────────
Hide
Type "/Users/scott/go/bin/pelorus --demo --theme gruvbox"
Enter
Sleep 8s
Show

# ── Preview pane ─────────────────────────────────────────────────────────────
# Toggle preview on; cursor is at cmd/ (top of dirs-first list)
Type "p"
Sleep 1.5s

# Navigate to README.md
# dirs-first name sort: cmd(0) docs(1) internal(2) scripts(3) web(4) Makefile(5) README.md(6)
Down
Sleep 120ms
Down
Sleep 120ms
Down
Sleep 120ms
Down
Sleep 120ms
Down
Sleep 120ms
Down
Sleep 1.2s

# Preview shows rendered markdown — scroll through it
Type "]"
Sleep 500ms
Type "]"
Sleep 500ms
Type "]"
Sleep 500ms
Type "]"
Sleep 1s

# ── Navigate into internal/api/server.go ─────────────────────────────────────
# Go up to internal/ (README.md is index 6, internal is index 2: 4 ups)
Up
Sleep 120ms
Up
Sleep 120ms
Up
Sleep 120ms
Up
Sleep 600ms

# Enter internal/ — cursor lands on api/ (first item)
Right
Sleep 1.2s

# Enter api/ — cursor lands on routes.go (first item)
Right
Sleep 800ms

# Move down to server.go (routes.go=0, server.go=1)
Down
Sleep 500ms

# Preview shows syntax-highlighted Go
Sleep 1.5s

# Scroll the file
Type "]"
Sleep 500ms
Type "]"
Sleep 500ms
Type "]"
Sleep 800ms

# Search within preview
Type "/"
Sleep 600ms
Type "ListenAndServe"
Sleep 1.5s
Escape
Sleep 800ms

# ── Back to axiom root ────────────────────────────────────────────────────────
Left
Sleep 400ms
Left
Sleep 800ms

# ── Marks ────────────────────────────────────────────────────────────────────
# Cursor is at internal/ after coming back up. Set mark 'a' here at axiom root.
Type "ma"
Sleep 700ms

# Navigate one level into internal/
Right
Sleep 1s

# Jump back to axiom root via mark
Type "'a"
Sleep 1.2s

# ── Sort cycling ─────────────────────────────────────────────────────────────
# Cursor is at internal/ (where it landed after 'a jump resets to top=0... or
# clampCursor keeps it). Cycle sort — we only care about the visual, not position.
Type "s"
Sleep 900ms
Type "s"
Sleep 900ms
Type "s"
Sleep 900ms
Type "s"
Sleep 800ms

# ── Right pane: docs/ ────────────────────────────────────────────────────────
# Tab to right pane (cursor at cmd/ = 0, never moved right pane)
Tab
Sleep 600ms

# Down once → docs/ (cmd=0, docs=1)
Down
Sleep 400ms

# Enter docs/
Right
Sleep 1.2s

# Cursor lands on api.md (first item) — preview shows rendered markdown
Sleep 1.5s

# ── Pane resize ──────────────────────────────────────────────────────────────
# Tab back to left pane to resize it
Tab
Sleep 400ms
Type ">"
Sleep 250ms
Type ">"
Sleep 250ms
Type ">"
Sleep 250ms
Type ">"
Sleep 800ms

Type "<"
Sleep 250ms
Type "<"
Sleep 250ms
Type "<"
Sleep 250ms
Type "<"
Sleep 800ms

# ── Recursive file search ─────────────────────────────────────────────────────
Ctrl+f
Sleep 2s
Type "postgres"
Sleep 1.5s
Escape
Sleep 800ms

# ── Command palette ───────────────────────────────────────────────────────────
Type ":"
Sleep 1.5s
Type "copy"
Sleep 1.2s
Escape
Sleep 800ms

# ── Quit ─────────────────────────────────────────────────────────────────────
Type "q"
Sleep 500ms
