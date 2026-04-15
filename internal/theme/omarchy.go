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
// Omarchy is a Linux distro (basecamp/omarchy); `current/theme` is a symlink
// to the user's active theme directory, built-in or custom. The colors.toml
// inside is a flat key=value file with ANSI color names:
//
//	background, foreground, cursor, selection_background
//	color0–color15  (standard ANSI 16-color set)
//	accent          (optional; Omarchy-specific primary accent colour)
//
// Light themes are signalled by either a `light.mode` sentinel file in the
// theme directory or (as a fallback) a background that is perceptually
// brighter than the foreground.
//
// ANSI role conventions:
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

	// Light mode: sentinel file, or background brighter than foreground.
	_, statErr := os.Stat(filepath.Join(themeDir, "light.mode"))
	light := statErr == nil ||
		hexLuminance(c["background"]) > hexLuminance(c["foreground"])

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
		hintText       string
	)

	if light {
		// Light themes: surfaces must sit a touch darker than the pure
		// background so panels/header/status are visible against it, and all
		// foreground text must use the dark `foreground` colour, not anything
		// derived from background.
		dirColor = get("color4", "color12")
		symlinkColor = get("color5", "color13")
		selBg = get("selection_background", "color7")
		inactiveBorder = get("color8", "color7")
		// Prefer bright white (color15) for surfaces when it differs from
		// background; otherwise fall back to selection_background which is a
		// subtle tint by convention.
		statusBg = pickDifferent(paneBg, get("color15"), get("selection_background"), get("color7"), paneBg)
		paletteBg = pickDifferent(paneBg, get("color15"), get("selection_background"), get("color7"), paneBg)
		hdrBg = pickDifferent(paneBg, get("selection_background"), get("color7"), get("color15"), paneBg)
		markedColor = get("color3", "color11")
		// Dim variants for muted text must stay legible on light surfaces —
		// use a dark tone rather than color8 which is often near-black only
		// in dark palettes.
		accentDim = get("accent", "color4", "color12", "color6")
		dimText = darken(foreground, 0x55)
		hintText = darken(foreground, 0x33)
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
		// For dark themes, color7 (white-ish) is a good muted body text;
		// color8 is too close to the background for small text.
		dimText = get("color7", "color15", "foreground")
		hintText = get("color7", "color15", "foreground")
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
			Foreground(lipgloss.Color(hintText)),

		StatusBarAccent: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color(accent)),

		StatusBarMuted: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color(dimText)),

		FooterBg: statusBg,
		FooterKey: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color(accent)).
			Bold(true),
		FooterDesc: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color(dimText)),
		FooterHover: lipgloss.NewStyle().
			Background(lipgloss.Color(accent)).
			Foreground(lipgloss.Color(paneBg)).
			Bold(true),

		SectionLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color(accent)).
			Bold(true),

		MascotStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(accent)),
	}, true
}

// pickDifferent returns the first candidate whose hex value is different
// (normalised case-insensitively) from `base`; falls back to the final
// argument when all candidates equal base.
func pickDifferent(base string, candidates ...string) string {
	b := strings.ToLower(strings.TrimSpace(base))
	for _, v := range candidates[:len(candidates)-1] {
		if v == "" {
			continue
		}
		if strings.ToLower(strings.TrimSpace(v)) != b {
			return v
		}
	}
	return candidates[len(candidates)-1]
}

// darken shifts each channel of a #rrggbb colour toward black by `amt`
// (0-255). Used to derive readable muted foregrounds on light themes when
// the palette doesn't ship an explicit mid-grey.
func darken(hex string, amt int) string {
	h := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(h) != 6 {
		return hex
	}
	out := "#"
	for i := 0; i < 3; i++ {
		var v int
		for j := 0; j < 2; j++ {
			c := h[i*2+j]
			var d int
			switch {
			case c >= '0' && c <= '9':
				d = int(c - '0')
			case c >= 'a' && c <= 'f':
				d = int(c-'a') + 10
			case c >= 'A' && c <= 'F':
				d = int(c-'A') + 10
			default:
				return hex
			}
			v = v*16 + d
		}
		v -= amt
		if v < 0 {
			v = 0
		}
		out += string("0123456789abcdef"[v>>4]) + string("0123456789abcdef"[v&0xf])
	}
	return out
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
