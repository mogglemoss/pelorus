package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme holds all lipgloss styles for Pelorus.
type Theme struct {
	ActiveBorder    lipgloss.Style
	InactiveBorder  lipgloss.Style
	PreviewBorder   lipgloss.Style
	Cursor          lipgloss.Style
	DirName         lipgloss.Style
	FileName        lipgloss.Style
	SymlinkName     lipgloss.Style
	StatusBar       lipgloss.Style
	PaletteBox      lipgloss.Style
	PaletteInput    lipgloss.Style
	PaletteItem     lipgloss.Style
	PaletteSelected lipgloss.Style
	PathHeader      lipgloss.Style
	Divider         lipgloss.Style

	// Header bar styles.
	Header      lipgloss.Style
	HeaderTitle lipgloss.Style
	HeaderPath  lipgloss.Style
	HeaderHint  lipgloss.Style

	// MarkedEntry is the style for items marked for batch operations.
	MarkedEntry lipgloss.Style

	// The raw background color used for the header bar — used when
	// concatenating sections without a wrapper Render call.
	HeaderBg string

	// Status bar accent styles.
	StatusBarAccent lipgloss.Style // phosphor cyan for primary status bar text
	StatusBarMuted  lipgloss.Style // teal dim for secondary status bar text

	// Footer shortcut bar styles.
	FooterBg    string         // raw background color for the footer row
	FooterKey   lipgloss.Style // shortcut key chip (e.g. "^p")
	FooterDesc  lipgloss.Style // shortcut description text
	FooterHover lipgloss.Style // hovered shortcut highlight

	// SectionLabel is the HARUSPEX-style uppercase pane label ("LOCAL", "REMOTE").
	SectionLabel lipgloss.Style

	// MascotStyle is the foreground color applied to the mascot ASCII art.
	MascotStyle lipgloss.Style

	// Card semantic styles (used by internal/preview fallback cards). These
	// give themes control over card colour rather than hardcoded Catppuccin
	// hex. Any theme that leaves these unset will produce all-white cards —
	// every Theme constructor should populate them.
	CardLabel   lipgloss.Style // label column text (dim)
	CardDim     lipgloss.Style // absent perm bits, size-bar unfilled
	CardSuccess lipgloss.Style // green: exec bit, small size, fresh age
	CardWarning lipgloss.Style // yellow: write bit, medium size/age
	CardDanger  lipgloss.Style // red: large size, stale age, broken symlink
	CardInfo    lipgloss.Style // blue: read bit
	CardAccent  lipgloss.Style // theme accent for arrows & highlights

	// PaletteCategoryHeader styles the uppercase category dividers inside the
	// command palette and help overlay. Derived from the theme accent at load.
	PaletteCategoryHeader lipgloss.Style
}

// Package-level semantic colour styles for card rendering. These are the
// defaults applied by the darkCards() / lightCards() helpers below. Themes
// that want to customise any field may override after calling the helper.
var (
	darkCardLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7f8896"))
	darkCardDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("#3b4252"))
	darkCardSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("#a3be8c"))
	darkCardWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebcb8b"))
	darkCardDanger  = lipgloss.NewStyle().Foreground(lipgloss.Color("#bf616a"))
	darkCardInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("#81a1c1"))

	lightCardLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("#586e75"))
	lightCardDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("#b8c5cc"))
	lightCardSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("#237a23"))
	lightCardWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("#9a6700"))
	lightCardDanger  = lipgloss.NewStyle().Foreground(lipgloss.Color("#b0222f"))
	lightCardInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("#1c5488"))
)

// applyDarkCards sets the 7 Card fields on a dark theme using the shared
// dark palette + the theme's accent colour.
func applyDarkCards(t *Theme, accent string) {
	t.CardLabel = darkCardLabel
	t.CardDim = darkCardDim
	t.CardSuccess = darkCardSuccess
	t.CardWarning = darkCardWarning
	t.CardDanger = darkCardDanger
	t.CardInfo = darkCardInfo
	t.CardAccent = lipgloss.NewStyle().Foreground(lipgloss.Color(accent))
	t.PaletteCategoryHeader = lipgloss.NewStyle().
		Foreground(lipgloss.Color(accent)).
		Background(t.PaletteBox.GetBackground()).
		Bold(true)
}

// applyLightCards sets the 7 Card fields for light themes.
func applyLightCards(t *Theme, accent string) {
	t.CardLabel = lightCardLabel
	t.CardDim = lightCardDim
	t.CardSuccess = lightCardSuccess
	t.CardWarning = lightCardWarning
	t.CardDanger = lightCardDanger
	t.CardInfo = lightCardInfo
	t.CardAccent = lipgloss.NewStyle().Foreground(lipgloss.Color(accent))
	t.PaletteCategoryHeader = lipgloss.NewStyle().
		Foreground(lipgloss.Color(accent)).
		Background(t.PaletteBox.GetBackground()).
		Bold(true)
}

// Get returns the named theme, defaulting to haruspex.
//
// Omarchy (a Linux distro, not a theme) ships a per-user palette at
// ~/.config/omarchy/current/theme/colors.toml. When no theme is configured
// (name == "" or "auto"), Get inherits whatever Omarchy theme the user has
// active on their system — built-in or custom. If no Omarchy palette is
// present, Get falls back to the haruspex built-in.
//
// Setting theme = "omarchy" in config forces the Omarchy loader; if no
// Omarchy palette is detected it falls back to haruspex so the app remains
// usable on non-Omarchy systems.
//
// Any explicit built-in name (haruspex, gruvbox, nord, light, dracula,
// catppuccin) bypasses Omarchy entirely.
func Get(name string) Theme {
	var t Theme
	n := strings.ToLower(name)
	switch n {
	case "gruvbox":
		t = GruvboxTheme()
	case "nord":
		t = NordTheme()
	case "light":
		t = LightTheme()
	case "dracula":
		t = DraculaTheme()
	case "catppuccin":
		t = CatppuccinTheme()
	case "haruspex":
		t = HaruspexTheme()
	case "omarchy":
		if ot, ok := LoadOmarchyTheme(); ok {
			t = ot
		} else {
			t = HaruspexTheme()
		}
	default:
		if ot, ok := LoadOmarchyTheme(); ok {
			t = ot
		} else {
			t = HaruspexTheme()
		}
	}
	// Populate Card* semantic styles from the theme's accent. Use the theme's
	// own background luminance (not the user-supplied name) to decide whether
	// the palette is light — that way Omarchy light themes, custom themes,
	// etc. all get the right card styles automatically.
	accent := extractAccent(t)
	if isLightTheme(t) {
		applyLightCards(&t, accent)
	} else {
		applyDarkCards(&t, accent)
	}
	return t
}

// isLightTheme returns true when the theme's pane background is bright
// enough that light-mode contrast styling should apply.
func isLightTheme(t Theme) bool {
	bg, ok := t.PaletteItem.GetBackground().(lipgloss.Color)
	if !ok {
		return false
	}
	return hexLuminance(string(bg)) > 0.5
}

// hexLuminance returns a rough [0,1] perceived luminance for a #rrggbb
// string. Returns 0 for unparseable inputs.
func hexLuminance(hex string) float64 {
	h := strings.TrimPrefix(hex, "#")
	if len(h) != 6 {
		return 0
	}
	var r, g, b int
	for i, out := range []*int{&r, &g, &b} {
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
				return 0
			}
			v = v*16 + d
		}
		*out = v
	}
	return (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 255.0
}

// extractAccent returns the theme's primary accent colour as a hex string,
// derived from MascotStyle (which every theme sets to its accent). Falls back
// to a safe neutral if the style doesn't have a typed color.
func extractAccent(t Theme) string {
	if c, ok := t.MascotStyle.GetForeground().(lipgloss.Color); ok {
		return string(c)
	}
	return "#888888"
}


// ---------------------------------------------------------------------------
// gruvbox — warm retro
// ---------------------------------------------------------------------------

func GruvboxTheme() Theme {
	hdrBg := "#3c3836"
	paneBg := "#282828"
	return Theme{
		HeaderBg:  hdrBg,


		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("#b8bb26")).
			Background(lipgloss.Color(paneBg)),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#a89984")).
			Background(lipgloss.Color(paneBg)),

		PreviewBorder: lipgloss.NewStyle().
			Border(lipgloss.Border{
				Top: "╌", Bottom: "╌",
				Left: "┊", Right: "┊",
				TopLeft: "╭", TopRight: "╮",
				BottomLeft: "╰", BottomRight: "╯",
			}).
			BorderForeground(lipgloss.Color("#a89984")).
			Background(lipgloss.Color(paneBg)),

		Cursor: lipgloss.NewStyle().
			Background(lipgloss.Color("#fabd2f")).
			Foreground(lipgloss.Color("#282828")).
			Bold(true),

		DirName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#83a598")).
			Bold(true),

		FileName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ebdbb2")),

		SymlinkName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d3869b")).
			Italic(true),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("#1d2021")).
			Foreground(lipgloss.Color("#a89984")).
			Padding(0, 1),

		PaletteBox: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#fabd2f")).
			Background(lipgloss.Color("#32302f")).
			Padding(1, 2),

		PaletteInput: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fabd2f")).
			Background(lipgloss.Color("#32302f")),

		PaletteItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ebdbb2")).
			Background(lipgloss.Color("#32302f")),

		PaletteSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#282828")).
			Background(lipgloss.Color("#b8bb26")).
			Bold(true),

		PathHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d79921")).
			Bold(true),

		MarkedEntry: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fabd2f")).
			Bold(true),

		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a89984")),

		Header: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)),

		HeaderTitle: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#fabd2f")).
			Bold(true),

		HeaderPath: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#a89984")),

		HeaderHint: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#a89984")),

		StatusBarAccent: lipgloss.NewStyle().
			Background(lipgloss.Color("#1d2021")).
			Foreground(lipgloss.Color("#fabd2f")),

		StatusBarMuted: lipgloss.NewStyle().
			Background(lipgloss.Color("#1d2021")).
			Foreground(lipgloss.Color("#a89984")),

		FooterBg: "#1d2021",
		FooterKey: lipgloss.NewStyle().
			Background(lipgloss.Color("#1d2021")).
			Foreground(lipgloss.Color("#fabd2f")).
			Bold(true),
		FooterDesc: lipgloss.NewStyle().
			Background(lipgloss.Color("#1d2021")).
			Foreground(lipgloss.Color("#a89984")),
		FooterHover: lipgloss.NewStyle().
			Background(lipgloss.Color("#fabd2f")).
			Foreground(lipgloss.Color("#282828")).
			Bold(true),

		SectionLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fabd2f")).
			Bold(true),

		MascotStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fabd2f")),
	}
}

// ---------------------------------------------------------------------------
// nord — cool arctic
// ---------------------------------------------------------------------------

func NordTheme() Theme {
	hdrBg := "#3b4252"
	paneBg := "#2e3440"
	return Theme{
		HeaderBg:  hdrBg,


		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("#88c0d0")).
			Background(lipgloss.Color(paneBg)),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#434c5e")).
			Background(lipgloss.Color(paneBg)),

		PreviewBorder: lipgloss.NewStyle().
			Border(lipgloss.Border{
				Top: "╌", Bottom: "╌",
				Left: "┊", Right: "┊",
				TopLeft: "╭", TopRight: "╮",
				BottomLeft: "╰", BottomRight: "╯",
			}).
			BorderForeground(lipgloss.Color("#434c5e")).
			Background(lipgloss.Color(paneBg)),

		Cursor: lipgloss.NewStyle().
			Background(lipgloss.Color("#5e81ac")).
			Foreground(lipgloss.Color("#eceff4")).
			Bold(true),

		DirName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#81a1c1")).
			Bold(true),

		FileName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d8dee9")),

		SymlinkName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#b48ead")).
			Italic(true),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("#242933")).
			Foreground(lipgloss.Color("#4c566a")).
			Padding(0, 1),

		PaletteBox: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#88c0d0")).
			Background(lipgloss.Color("#3b4252")).
			Padding(1, 2),

		PaletteInput: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88c0d0")).
			Background(lipgloss.Color("#3b4252")),

		PaletteItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d8dee9")).
			Background(lipgloss.Color("#3b4252")),

		PaletteSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2e3440")).
			Background(lipgloss.Color("#88c0d0")).
			Bold(true),

		PathHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#81a1c1")).
			Bold(true),

		MarkedEntry: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ebcb8b")).
			Bold(true),

		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#434c5e")),

		Header: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)),

		HeaderTitle: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#88c0d0")).
			Bold(true),

		HeaderPath: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#d8dee9")),

		HeaderHint: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#7b8898")),

		StatusBarAccent: lipgloss.NewStyle().
			Background(lipgloss.Color("#242933")).
			Foreground(lipgloss.Color("#88c0d0")),

		StatusBarMuted: lipgloss.NewStyle().
			Background(lipgloss.Color("#242933")).
			Foreground(lipgloss.Color("#4c566a")),

		FooterBg: "#242933",
		FooterKey: lipgloss.NewStyle().
			Background(lipgloss.Color("#242933")).
			Foreground(lipgloss.Color("#88c0d0")).
			Bold(true),
		FooterDesc: lipgloss.NewStyle().
			Background(lipgloss.Color("#242933")).
			Foreground(lipgloss.Color("#4c566a")),
		FooterHover: lipgloss.NewStyle().
			Background(lipgloss.Color("#88c0d0")).
			Foreground(lipgloss.Color("#2e3440")).
			Bold(true),

		SectionLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88c0d0")).
			Bold(true),

		MascotStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88c0d0")),
	}
}

// ---------------------------------------------------------------------------
// light — for daylight use
// ---------------------------------------------------------------------------

func LightTheme() Theme {
	hdrBg := "#0e7c7b"
	paneBg := "#f5f5f5"
	return Theme{
		HeaderBg:  hdrBg,


		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("#0e7c7b")).
			Background(lipgloss.Color(paneBg)),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#cccccc")).
			Background(lipgloss.Color(paneBg)),

		PreviewBorder: lipgloss.NewStyle().
			Border(lipgloss.Border{
				Top: "╌", Bottom: "╌",
				Left: "┊", Right: "┊",
				TopLeft: "╭", TopRight: "╮",
				BottomLeft: "╰", BottomRight: "╯",
			}).
			BorderForeground(lipgloss.Color("#cccccc")).
			Background(lipgloss.Color(paneBg)),

		Cursor: lipgloss.NewStyle().
			Background(lipgloss.Color("#0e7c7b")).
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true),

		DirName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1565c0")).
			Bold(true),

		FileName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#333333")),

		SymlinkName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7b1fa2")).
			Italic(true),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("#e0e0e0")).
			Foreground(lipgloss.Color("#555555")).
			Padding(0, 1),

		PaletteBox: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#0e7c7b")).
			Background(lipgloss.Color("#ffffff")).
			Padding(1, 2),

		PaletteInput: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0e7c7b")).
			Background(lipgloss.Color("#ffffff")),

		PaletteItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#333333")).
			Background(lipgloss.Color("#ffffff")),

		PaletteSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#0e7c7b")).
			Bold(true),

		PathHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0e7c7b")).
			Bold(true),

		MarkedEntry: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e65100")).
			Bold(true),

		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cccccc")),

		Header: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)),

		HeaderTitle: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true),

		HeaderPath: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#ffffff")),

		HeaderHint: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#e8f4f4")).
			Bold(true),

		StatusBarAccent: lipgloss.NewStyle().
			Background(lipgloss.Color("#e0e0e0")).
			Foreground(lipgloss.Color("#0e7c7b")),

		StatusBarMuted: lipgloss.NewStyle().
			Background(lipgloss.Color("#e0e0e0")).
			Foreground(lipgloss.Color("#555555")),

		FooterBg: "#e0e0e0",
		FooterKey: lipgloss.NewStyle().
			Background(lipgloss.Color("#e0e0e0")).
			Foreground(lipgloss.Color("#0e7c7b")).
			Bold(true),
		FooterDesc: lipgloss.NewStyle().
			Background(lipgloss.Color("#e0e0e0")).
			Foreground(lipgloss.Color("#2a2a2a")),
		FooterHover: lipgloss.NewStyle().
			Background(lipgloss.Color("#0e7c7b")).
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true),

		SectionLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0e7c7b")).
			Bold(true),

		MascotStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0e7c7b")),
	}
}

// ---------------------------------------------------------------------------
// dracula — purple night
// ---------------------------------------------------------------------------

func DraculaTheme() Theme {
	hdrBg := "#44475A"
	paneBg := "#282A36"
	statusBg := "#21222C"
	paletteBg := "#21222C"
	return Theme{
		HeaderBg:  hdrBg,


		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("#BD93F9")).
			Background(lipgloss.Color(paneBg)),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#44475A")).
			Background(lipgloss.Color(paneBg)),

		PreviewBorder: lipgloss.NewStyle().
			Border(lipgloss.Border{
				Top: "╌", Bottom: "╌",
				Left: "┊", Right: "┊",
				TopLeft: "╭", TopRight: "╮",
				BottomLeft: "╰", BottomRight: "╯",
			}).
			BorderForeground(lipgloss.Color("#44475A")).
			Background(lipgloss.Color(paneBg)),

		Cursor: lipgloss.NewStyle().
			Background(lipgloss.Color("#44475A")).
			Foreground(lipgloss.Color("#BD93F9")).
			Bold(true),

		DirName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8BE9FD")).
			Bold(true),

		FileName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2")),

		SymlinkName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF79C6")).
			Italic(true),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color("#6272A4")).
			Padding(0, 1),

		PaletteBox: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#BD93F9")).
			Background(lipgloss.Color(paletteBg)).
			Padding(1, 2),

		PaletteInput: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BD93F9")).
			Background(lipgloss.Color(paletteBg)),

		PaletteItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2")).
			Background(lipgloss.Color(paletteBg)),

		PaletteSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#282A36")).
			Background(lipgloss.Color("#BD93F9")).
			Bold(true),

		PathHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BD93F9")).
			Bold(true),

		MarkedEntry: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F1FA8C")).
			Bold(true),

		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#44475A")),

		Header: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)),

		HeaderTitle: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#BD93F9")).
			Bold(true),

		HeaderPath: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#F8F8F2")),

		HeaderHint: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#6272A4")),

		StatusBarAccent: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color("#BD93F9")),

		StatusBarMuted: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color("#6272A4")),

		FooterBg: statusBg,
		FooterKey: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color("#BD93F9")).
			Bold(true),
		FooterDesc: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color("#6272A4")),
		FooterHover: lipgloss.NewStyle().
			Background(lipgloss.Color("#BD93F9")).
			Foreground(lipgloss.Color("#282A36")).
			Bold(true),

		SectionLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BD93F9")).
			Bold(true),

		MascotStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BD93F9")),
	}
}

// ---------------------------------------------------------------------------
// catppuccin — catppuccin mocha
// ---------------------------------------------------------------------------

func CatppuccinTheme() Theme {
	hdrBg := "#313244"  // surface0
	paneBg := "#1e1e2e" // base
	statusBg := "#11111b"  // crust
	paletteBg := "#181825" // mantle
	return Theme{
		HeaderBg:  hdrBg,


		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("#cba6f7")).
			Background(lipgloss.Color(paneBg)),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#45475a")).
			Background(lipgloss.Color(paneBg)),

		PreviewBorder: lipgloss.NewStyle().
			Border(lipgloss.Border{
				Top: "╌", Bottom: "╌",
				Left: "┊", Right: "┊",
				TopLeft: "╭", TopRight: "╮",
				BottomLeft: "╰", BottomRight: "╯",
			}).
			BorderForeground(lipgloss.Color("#45475a")).
			Background(lipgloss.Color(paneBg)),

		Cursor: lipgloss.NewStyle().
			Background(lipgloss.Color("#45475a")).
			Foreground(lipgloss.Color("#cba6f7")).
			Bold(true),

		DirName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#89b4fa")).
			Bold(true),

		FileName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cdd6f4")),

		SymlinkName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f5c2e7")).
			Italic(true),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color("#585b70")).
			Padding(0, 1),

		PaletteBox: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#cba6f7")).
			Background(lipgloss.Color(paletteBg)).
			Padding(1, 2),

		PaletteInput: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cba6f7")).
			Background(lipgloss.Color(paletteBg)),

		PaletteItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cdd6f4")).
			Background(lipgloss.Color(paletteBg)),

		PaletteSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1e1e2e")).
			Background(lipgloss.Color("#cba6f7")).
			Bold(true),

		PathHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cba6f7")).
			Bold(true),

		MarkedEntry: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f9e2af")).
			Bold(true),

		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#45475a")),

		Header: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)),

		HeaderTitle: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#cba6f7")).
			Bold(true),

		HeaderPath: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#bac2de")),

		HeaderHint: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#6c7086")),

		StatusBarAccent: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color("#cba6f7")),

		StatusBarMuted: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color("#585b70")),

		FooterBg: statusBg,
		FooterKey: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color("#cba6f7")).
			Bold(true),
		FooterDesc: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color("#585b70")),
		FooterHover: lipgloss.NewStyle().
			Background(lipgloss.Color("#cba6f7")).
			Foreground(lipgloss.Color("#1e1e2e")).
			Bold(true),

		SectionLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cba6f7")).
			Bold(true),

		MascotStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cba6f7")),
	}
}

// ---------------------------------------------------------------------------
// haruspex — warm rust (default)
// ---------------------------------------------------------------------------

const (
	hxBg      = "#1a1815"
	hxSurface = "#201d18"
	hxPanel   = "#252118"
	hxText    = "#e8e6e3"
	hxMuted   = "#7a756e"
	hxBorder  = "#3a3530"
	hxRust    = "#C15F3C"
	hxGold    = "#d4a017"
	hxSymlink = "#8a6e4a"
	hxFade    = "#8A3820"
	hxFlare   = "#e8a559"
)

func HaruspexTheme() Theme {
	hdrBg := hxPanel
	paneBg := hxBg
	statusBg := hxSurface
	paletteBg := hxSurface
	return Theme{
		HeaderBg:  hdrBg,


		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color(hxRust)).
			Background(lipgloss.Color(paneBg)),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(hxBorder)).
			Background(lipgloss.Color(paneBg)),

		PreviewBorder: lipgloss.NewStyle().
			Border(lipgloss.Border{
				Top: "╌", Bottom: "╌",
				Left: "┊", Right: "┊",
				TopLeft: "╭", TopRight: "╮",
				BottomLeft: "╰", BottomRight: "╯",
			}).
			BorderForeground(lipgloss.Color(hxBorder)).
			Background(lipgloss.Color(paneBg)),

		Cursor: lipgloss.NewStyle().
			Background(lipgloss.Color("#3a2a20")).
			Foreground(lipgloss.Color(hxRust)).
			Bold(true),

		DirName: lipgloss.NewStyle().
			Foreground(lipgloss.Color(hxGold)).
			Bold(true),

		FileName: lipgloss.NewStyle().
			Foreground(lipgloss.Color(hxText)),

		SymlinkName: lipgloss.NewStyle().
			Foreground(lipgloss.Color(hxSymlink)).
			Italic(true),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color(hxMuted)).
			Padding(0, 1),

		PaletteBox: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color(hxRust)).
			Background(lipgloss.Color(paletteBg)).
			Padding(1, 2),

		PaletteInput: lipgloss.NewStyle().
			Foreground(lipgloss.Color(hxRust)).
			Background(lipgloss.Color(paletteBg)),

		PaletteItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color(hxText)).
			Background(lipgloss.Color(paletteBg)),

		PaletteSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(hxBg)).
			Background(lipgloss.Color(hxRust)).
			Bold(true),

		PathHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color(hxRust)).
			Bold(true),

		MarkedEntry: lipgloss.NewStyle().
			Foreground(lipgloss.Color(hxGold)).
			Bold(true),

		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color(hxBorder)),

		Header: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)),

		HeaderTitle: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color(hxRust)).
			Bold(true),

		HeaderPath: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color(hxText)),

		HeaderHint: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color(hxMuted)),

		StatusBarAccent: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color(hxRust)),

		StatusBarMuted: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color(hxMuted)),

		FooterBg: statusBg,
		FooterKey: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color(hxRust)).
			Bold(true),
		FooterDesc: lipgloss.NewStyle().
			Background(lipgloss.Color(statusBg)).
			Foreground(lipgloss.Color(hxMuted)),
		FooterHover: lipgloss.NewStyle().
			Background(lipgloss.Color(hxRust)).
			Foreground(lipgloss.Color(hxBg)).
			Bold(true),

		SectionLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color(hxRust)).
			Bold(true),

		MascotStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(hxRust)),
	}
}
