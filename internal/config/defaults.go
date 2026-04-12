package config

// Defaults returns the default configuration struct.
func Defaults() *Config {
	return &Config{
		General: GeneralConfig{
			StartDir:      ".",
			ShowHidden:    false,
			ConfirmDelete: true,
			Editor:        "vi",
		},
		Layout: LayoutConfig{
			Ratio:        "50:50",
			ShowPreview:  true,
			PreviewWidth: 40,
		},
		Theme: ThemeConfig{
			Name: "pelorus",
		},
		Keybindings: map[string]string{},
		Actions:     ActionsConfig{},
	}
}

// GenerateDefaultConfig returns a fully-commented default config.toml as a string.
// This is written to disk on first run so the file itself is documentation.
func GenerateDefaultConfig() string {
	return `# Pelorus configuration
# All options are shown with their defaults.

[general]
# Starting directory. Use "." for cwd, "last" to restore previous session,
# or an absolute path like "/home/user/projects".
start_dir = "."

# Show hidden files (dotfiles) by default.
show_hidden = false

# Prompt before deleting files.
confirm_delete = true

# Editor to open files with. Falls back to $EDITOR then $VISUAL if empty.
editor = ""

[layout]
# Left:right pane width ratio.
ratio = "1:1"

# Show the preview panel on startup.
show_preview = true

# Width of the preview panel as a percentage of total terminal width.
preview_width = 40

[theme]
# Built-in themes: pelorus, gruvbox, nord, light
name = "pelorus"

[keybindings]
# Override any action's keybinding. Format: "action.id" = "key"
# Examples (uncomment and adjust to taste):
# "nav.up"       = "ctrl+k"
# "nav.down"     = "ctrl+j"
# "file.copy"    = "y"
# "app.quit"     = "ctrl+q"

# --- Custom shell actions ---
# Uncomment and edit to add your own actions. They appear in the command palette.
# Template variables: {path} = full path, {name} = filename, {dir} = parent directory

# [[actions.custom]]
# id          = "custom.open-zed"
# name        = "Open in Zed"
# description = "Open selected file in Zed editor"
# category    = "Custom"
# command     = "zed {path}"
# context     = "always"   # always | file | dir | remote | local

# [[actions.custom]]
# id          = "custom.copy-path"
# name        = "Copy Path"
# description = "Copy selected path to clipboard"
# category    = "Custom"
# command     = "echo {path} | pbcopy"
# context     = "file"
`
}
