package theme

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// LoadOmarchyTheme reads the active Omarchy theme from
// ~/.config/omarchy/current/theme/colors.toml and maps it to a pelorus Theme.
// Returns (theme, true) on success, (zero, false) if the file isn't found.
//
// Omarchy stores its palette as a flat key=value file with ANSI color names:
//
//	background, foreground, cursor, selection_background
//	color0–color15  (standard ANSI 16-color set)
//	accent          (optional; Omarchy-specific primary accent colour)
//
// ANSI role conventions used here:
//
//	color0  black (darkest surface)          color8  bright black (overlay/muted)
//	color1  red                              color9  bright red
//	color2  green                            color10 bright green
//	color3  yellow                           color11 bright yellow
//	color4  blue                             color12 bright blue
//	color5  magenta/purple                   color13 bright magenta
//	color6  cyan                             color14 bright cyan
//	color7  white (lightest surface)         color15 bright white
func LoadOmarchyTheme() (Theme, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Theme{}, false
	}

	themeDir := filepath.Join(home, ".config", "omarchy", "current", "theme")
	data, err := os.ReadFile(filepath.Join(themeDir, "colors.toml"))
	if err != nil {
		return Theme{}, false
	}

	c := parseOmarchyColors(data)
	if c["foreground"] == "" || c["background"] == "" {
		return Theme{}, false
	}

	// Light mode: signalled by presence of a light.mode sentinel file.
	_, statErr := os.Stat(filepath.Join(themeDir, "light.mode"))
	light := statErr == nil

	// get returns the first non-empty value among the given keys.
	get := func(keys ...string) string {
		for _, k := range keys {
			if v := c[k]; v != "" {
				return v
			}
		}
		return "#888888" // safe neutral fallback
	}

	paneBg := get("background", "color0")
	foreground := get("foreground", "color15")
	accent := get("accent", "color6", "color14") // cyan family as accent

	var (
		dirColor       string
		symlinkColor   string
		selBg          string
		inactiveBorder string
		statusBg       string
		paletteBg      string
		markedColor    string
		hdrBg          string
		accentDim      string
		dimText        string
	)

	if light {
		dirColor = get("color4", "color12")
		symlinkColor = get("color5", "color13")
		selBg = get("selection_background", "color7")
		inactiveBorder = get("color8", "color7")
		statusBg = get("color7", "foreground")
		paletteBg = get("color7", "background")
		markedColor = get("color3", "color11")
		hdrBg = get("color7", "color8")
		accentDim = get("color6", "color14", "accent")
		dimText = get("color8", "color0")
	} else {
		dirColor = get("color12", "color4")     // bright blue preferred for dirs
		symlinkColor = get("color13", "color5") // bright magenta for symlinks
		selBg = get("selection_background", "color8")
		inactiveBorder = get("color8", "color0")
		statusBg = get("color0", "background")
		paletteBg = get("color0", "background")
		markedColor = get("color11", "color3") // bright yellow for marked items
		hdrBg = get("color8", "color0")
		accentDim = get("color14", "color6", "accent")
		dimText = get("color8", "color7")
	}

	return Theme{
		HeaderBg: hdrBg,

		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(accent)).
			Background(lipgloss.Color(paneBg)),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(inactiveBorder)).
			Background(lipgloss.Color(paneBg)),

		PreviewBorder: lipgloss.NewStyle().
			Border(lipgloss.Border{
				Top: "╌", Bottom: "╌",
				Left: "┊", Right: "┊",
				TopLeft: "╭", TopRight: "╮",
				BottomLeft: "╰", BottomRight: "╯",
			}).
			BorderForeground(lipgloss.Color(inactiveBorder)).
			Background(lipgloss.Color(paneBg)),

		Cursor: lipgloss.NewStyle().
			Background(lipgloss.Color(selBg)).
			Foreground(lipgloss.Color(accent)).
			Bold(true),

		DirName: lipgloss.NewStyle().
			Foreground(lipgloss.Color(dirColor)).
			Bold(true),

		FileName: lipgloss.NewStyle().
			Foreground(lipgloss.Color(foreground)),

		SymlinkName: lipgloss.NewStyle().
			Foreground(lipgloss.Color(symlinkColor)).
			Italic(true),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color(dimText)).
			Padding(0, 1),

		PaletteBox: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color(accent)).
			Background(lipgloss.Color(paletteBg)).
			Padding(1, 2),

		PaletteInput: lipgloss.NewStyle().
			Foreground(lipgloss.Color(accent)).
			Background(lipgloss.Color(paletteBg)),

		PaletteItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color(foreground)).
			Background(lipgloss.Color(paletteBg)),

		PaletteSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(paneBg)).
			Background(lipgloss.Color(accent)).
			Bold(true),

		PathHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color(accentDim)).
			Bold(true),

		MarkedEntry: lipgloss.NewStyle().
			Foreground(lipgloss.Color(markedColor)).
			Bold(true),

		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color(inactiveBorder)),

		Header: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)),

		HeaderTitle: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color(accent)).
			Bold(true),

		HeaderPath: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color(foreground)),

		HeaderHint: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color(paneBg)),

		StatusBarAccent: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color(accent)),

		StatusBarMuted: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color(dimText)),
	}, true
}

// parseOmarchyColors parses the flat key=value color file used by Omarchy.
// Lines beginning with # are comments. Values may be quoted.
func parseOmarchyColors(data []byte) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if key != "" && val != "" {
			result[key] = val
		}
	}
	return result
}
