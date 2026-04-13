# pelorus demo
# Run with: vhs pelorus.tape

Output assets/pelorus.gif

Set Shell "bash"
Set FontSize 14
Set FontFamily "JetBrainsMono Nerd Font"
Set Width 1400
Set Height 800
Set Theme "GruvboxDark"
Set PlaybackSpeed 1.0
Set TypingSpeed 80ms

# Launch in demo mode with gruvbox theme
Hide
Type "/Users/scott/go/bin/pelorus --demo --theme gruvbox"
Enter
Sleep 5s
Show

# Pane opens on axiom/ — git status glyphs visible (M on server.go, user.go; ? on websocket.go)
Sleep 1s

# Toggle preview pane on
Type "p"
Sleep 1.5s

# Navigate down to README.md — preview renders markdown
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 1.5s

# Scroll through the rendered README
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms
Type "]"
Sleep 1s

# Navigate to internal/ — enter it
Type "k"
Sleep 150ms
Type "k"
Sleep 150ms
Type "k"
Sleep 150ms
Type "k"
Sleep 150ms
Type "k"
Sleep 150ms
Type "k"
Sleep 500ms
Type "l"
Sleep 1.2s

# Preview shows api/ directory listing — enter api/
Type "l"
Sleep 1.2s

# Preview shows server.go with syntax highlighting — navigate to it
Type "l"
Sleep 1.5s

# Scroll through server.go
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms
Type "]"
Sleep 800ms

# Search within the file preview
Type "/"
Sleep 600ms
Type "ListenAndServe"
Sleep 1.5s

# Close search, back out to root
Escape
Sleep 500ms
Type "h"
Sleep 300ms
Type "h"
Sleep 300ms
Type "h"
Sleep 800ms

# Demonstrate sort cycling
Type "s"
Sleep 900ms
Type "s"
Sleep 900ms
Type "s"
Sleep 900ms
Type "s"
Sleep 800ms

# Set a mark on this directory, then navigate away and jump back
Type "ma"
Sleep 700ms

# Enter web/src/components/ using fuzzy filter
Type "l"
Sleep 500ms
Type "web"
Sleep 800ms
Escape
Sleep 300ms
Type "l"
Sleep 500ms
Type "src"
Sleep 800ms
Escape
Sleep 300ms
Type "l"
Sleep 1s

# Jump back to axiom root via mark
Type "'a"
Sleep 1.2s

# Switch to right pane, navigate to docs/
Tab
Sleep 600ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 500ms
Type "l"
Sleep 1.2s

# Navigate to api.md — preview shows rendered markdown
Type "j"
Sleep 1.5s

# Resize split: grow left pane a bit
Tab
Sleep 400ms
Type ">"
Sleep 300ms
Type ">"
Sleep 300ms
Type ">"
Sleep 300ms
Type ">"
Sleep 800ms

# Shrink back
Type "<"
Sleep 300ms
Type "<"
Sleep 300ms
Type "<"
Sleep 300ms
Type "<"
Sleep 800ms

# Open recursive search overlay
Ctrl+f
Sleep 2s
Type "postgres"
Sleep 1.5s
Escape
Sleep 800ms

# Open command palette
Type ":"
Sleep 1.5s
Type "copy"
Sleep 1.2s
Escape
Sleep 800ms

# Quit
Type "q"
Sleep 500ms
