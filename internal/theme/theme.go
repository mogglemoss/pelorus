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
}

// Get returns the named theme, defaulting to pelorus.
//
// Special behaviour for Omarchy users: when no explicit theme is configured
// (name == "" or name == "pelorus"), Get attempts to read the active Omarchy
// system theme from ~/.config/omarchy/current/theme/colors.toml. If found,
// pelorus inherits that palette automatically — no config required. Any
// explicit theme name (gruvbox, dracula, nord, light) overrides this.
//
// Setting theme = "omarchy" in config forces the Omarchy loader without
// falling back to the pelorus default when omarchy is not detected.
func Get(name string) Theme {
	switch strings.ToLower(name) {
	case "gruvbox":
		return GruvboxTheme()
	case "nord":
		return NordTheme()
	case "light":
		return LightTheme()
	case "dracula":
		return DraculaTheme()
	case "catppuccin":
		return CatppuccinTheme()
	case "haruspex":
		return HaruspexTheme()
	case "omarchy":
		// Explicit opt-in: dynamic if available, static Catppuccin Mocha otherwise.
		if t, ok := LoadOmarchyTheme(); ok {
			return t
		}
		return CatppuccinTheme()
	default:
		// Auto-detect: if running inside Omarchy, inherit the system palette.
		if t, ok := LoadOmarchyTheme(); ok {
			return t
		}
		return HaruspexTheme()
	}
}

// ---------------------------------------------------------------------------
// pelorus — retrofuture subaquatic
// ---------------------------------------------------------------------------

const (
	colorBg             = "#0a0f14"
	colorBgPane         = "#0d1520"
	colorPrimary        = "#0e7c7b"
	colorAccent         = "#00ffd0"
	colorAccentDim      = "#00a896"
	colorText           = "#c8d8e8"
	colorTextDim        = "#4a6070"
	colorDir            = "#4fc3f7"
	colorSymlink        = "#b39ddb"
	colorBorderActive   = "#00ffd0"
	colorBorderInactive = "#1a3040"
	colorCursorBg       = "#0e4060"
	colorCursorFg       = "#00ffd0"
	colorStatus         = "#081018"
	colorPaletteBg      = "#0d1a26"
	colorSelected       = "#00ffd0"
	colorSelectedBg     = "#0a3050"
)

func PelorusTheme() Theme {
	hdrBg := colorPrimary
	return Theme{
		HeaderBg:  hdrBg,


		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color(colorBorderActive)).
			Background(lipgloss.Color(colorBgPane)),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(colorBorderInactive)).
			Background(lipgloss.Color(colorBgPane)),

		PreviewBorder: lipgloss.NewStyle().
			Border(lipgloss.Border{
				Top: "╌", Bottom: "╌",
				Left: "┊", Right: "┊",
				TopLeft: "╭", TopRight: "╮",
				BottomLeft: "╰", BottomRight: "╯",
			}).
			BorderForeground(lipgloss.Color(colorBorderInactive)).
			Background(lipgloss.Color(colorBgPane)),

		Cursor: lipgloss.NewStyle().
			Background(lipgloss.Color(colorCursorBg)).
			Foreground(lipgloss.Color(colorCursorFg)).
			Bold(true),

		DirName: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDir)).
			Bold(true),

		FileName: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorText)),

		SymlinkName: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSymlink)).
			Italic(true),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color(colorStatus)).
			Foreground(lipgloss.Color(colorAccentDim)).
			Padding(0, 1),

		PaletteBox: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color(colorAccent)).
			Background(lipgloss.Color(colorPaletteBg)).
			Padding(1, 2),

		PaletteInput: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent)).
			Background(lipgloss.Color(colorPaletteBg)),

		PaletteItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorText)).
			Background(lipgloss.Color(colorPaletteBg)),

		PaletteSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSelected)).
			Background(lipgloss.Color(colorSelectedBg)).
			Bold(true),

		PathHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccentDim)).
			Bold(true),

		MarkedEntry: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffd700")).
			Bold(true),

		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBorderInactive)),

		Header: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)),

		HeaderTitle: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color(colorAccent)).
			Bold(true),

		HeaderPath: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#b2d8d8")),

		HeaderHint: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#b0d8d8")),

		StatusBarAccent: lipgloss.NewStyle().
			Background(lipgloss.Color(colorStatus)).
			Foreground(lipgloss.Color(colorAccent)),

		StatusBarMuted: lipgloss.NewStyle().
			Background(lipgloss.Color(colorStatus)).
			Foreground(lipgloss.Color(colorAccentDim)),

		FooterBg: colorStatus,
		FooterKey: lipgloss.NewStyle().
			Background(lipgloss.Color(colorStatus)).
			Foreground(lipgloss.Color(colorAccent)).
			Bold(true),
		FooterDesc: lipgloss.NewStyle().
			Background(lipgloss.Color(colorStatus)).
			Foreground(lipgloss.Color(colorAccentDim)),
		FooterHover: lipgloss.NewStyle().
			Background(lipgloss.Color(colorAccent)).
			Foreground(lipgloss.Color(colorStatus)).
			Bold(true),

		SectionLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent)).
			Bold(true),

		MascotStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent)),
	}
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
			Foreground(lipgloss.Color("#b2d8d8")),

		HeaderHint: lipgloss.NewStyle().
			Background(lipgloss.Color(hdrBg)).
			Foreground(lipgloss.Color("#b0d8d8")),

		StatusBarAccent: lipgloss.NewStyle().
			Background(lipgloss.Color("#e0e0e0")).
			Foreground(lipgloss.Color("#0e7c7b")),

		StatusBarMuted: lipgloss.NewStyle().
			Background(lipgloss.Color("#e0e0e0")).
			Foreground(lipgloss.Color("#777777")),

		FooterBg: "#e0e0e0",
		FooterKey: lipgloss.NewStyle().
			Background(lipgloss.Color("#e0e0e0")).
			Foreground(lipgloss.Color("#0e7c7b")).
			Bold(true),
		FooterDesc: lipgloss.NewStyle().
			Background(lipgloss.Color("#e0e0e0")).
			Foreground(lipgloss.Color("#555555")),
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
