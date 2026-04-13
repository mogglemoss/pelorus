# pelorus demo
# Run with: vhs pelorus.tape

Output assets/pelorus.gif

Set Shell "fish"
Set FontSize 14
Set FontFamily "JetBrainsMono Nerd Font"
Set Width 1400
Set Height 800
Set Theme "Dark+"
Set PlaybackSpeed 1.0
Set TypingSpeed 80ms

# Launch pelorus in the repo directory
Hide
Type "cd /Users/scott/Documents/development/pelorus && /Users/scott/go/bin/pelorus ."
Enter
Sleep 4s
Show

# Browse around — navigate into internal
Sleep 500ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "l"
Sleep 1s

# Toggle preview pane
Type "p"
Sleep 1.5s

# Navigate into app directory
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "l"
Sleep 1s

# Browse files — preview highlights code
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms

# Scroll preview
Type "]"
Sleep 300ms
Type "]"
Sleep 300ms
Type "]"
Sleep 300ms

# Go back up
Type "h"
Sleep 800ms
Type "h"
Sleep 800ms

# Open command palette
Type ":"
Sleep 1.5s

# Search for sort
Type "sort"
Sleep 1s
Enter
Sleep 1s

# Cycle sort again to show size
Type "s"
Sleep 800ms
Type "s"
Sleep 800ms
Type "s"
Sleep 800ms

# Open jump list
Type "g"
Sleep 1.5s
Escape
Sleep 500ms

# Show hidden files
Type "."
Sleep 1s
Type "."
Sleep 500ms

# Switch to other pane
Tab
Sleep 800ms

# Navigate other pane
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "l"
Sleep 1s

# Switch back
Tab
Sleep 500ms

# Quit
Type "q"
Sleep 500ms
