# pelorus demo
# Run with: vhs pelorus.tape

Output assets/pelorus.gif

Set Shell "fish"
Set FontSize 14
Set FontFamily "JetBrainsMono Nerd Font"
Set Width 1400
Set Height 800
Set Theme "Dracula"
Set PlaybackSpeed 1.0
Set TypingSpeed 80ms

# Launch with gruvbox theme
Hide
Type "/Users/scott/go/bin/pelorus /Users/scott/Documents/development/pelorus --theme gruvbox"
Enter
Sleep 4s
Show

# Cursor starts on assets/ — navigate down to README.md
# Root order (dirs first): assets cmd docs internal pkg | go.mod go.sum LICENSE main.go pelorus pelorus.tape README.md
Sleep 500ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 1.5s

# Preview pane shows rendered README.md markdown

# Scroll through the README
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms
Type "]"
Sleep 1s

# Navigate back up to internal/ (8 k presses up from README.md)
Type "k"
Sleep 100ms
Type "k"
Sleep 100ms
Type "k"
Sleep 100ms
Type "k"
Sleep 100ms
Type "k"
Sleep 100ms
Type "k"
Sleep 100ms
Type "k"
Sleep 100ms
Type "k"
Sleep 500ms

# Enter internal/
Type "l"
Sleep 1s

# Navigate down to theme/ (13 j presses: app config connect gitstatus help jump nav ops palette pane preview provider theme)
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 800ms

# Enter theme/
Type "l"
Sleep 1s

# Preview shows theme.go with syntax highlighting — scroll through it
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms
Type "]"
Sleep 1s

# Navigate to omarchy.go (j once)
Type "j"
Sleep 1s

# Back out to root
Type "h"
Sleep 500ms
Type "h"
Sleep 800ms

# Show sort cycling on the root directory
Type "s"
Sleep 1s
Type "s"
Sleep 1s
Type "s"
Sleep 1s
Type "s"
Sleep 800ms

# Open command palette
Type ":"
Sleep 2s

# Close palette
Escape
Sleep 800ms

# Switch to right pane and navigate to docs/
Tab
Sleep 800ms
Type "j"
Sleep 100ms
Type "j"
Sleep 500ms

# Enter docs/
Type "l"
Sleep 1s

# Preview shows enhancements.md — scroll through it
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms

# Navigate to reference.md
Type "j"
Sleep 1.5s

# Switch back to left pane
Tab
Sleep 500ms

# Quit
Type "q"
Sleep 500ms
