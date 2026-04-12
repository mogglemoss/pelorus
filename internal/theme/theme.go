package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme holds all lipgloss styles for Pelorus.
type Theme struct {
	ActiveBorder    lipgloss.Style
	InactiveBorder  lipgloss.Style
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

	// The raw background color used for the header bar — used when
	// concatenating sections without a wrapper Render call.
	HeaderBg string
}

// Get returns the named theme, defaulting to pelorus.
func Get(name string) Theme {
	switch strings.ToLower(name) {
	case "gruvbox":
		return GruvboxTheme()
	case "nord":
		return NordTheme()
	case "light":
		return LightTheme()
	default:
		return PelorusTheme()
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
		HeaderBg: hdrBg,

		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorderActive)).
			Background(lipgloss.Color(colorBgPane)),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
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
			Border(lipgloss.RoundedBorder()).
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
			Foreground(lipgloss.Color("#052424")),
	}
}

// ---------------------------------------------------------------------------
// gruvbox — warm retro
// ---------------------------------------------------------------------------

func GruvboxTheme() Theme {
	hdrBg := "#3c3836"
	paneBg := "#282828"
	return Theme{
		HeaderBg: hdrBg,

		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#b8bb26")).
			Background(lipgloss.Color(paneBg)),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#504945")).
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
			Border(lipgloss.RoundedBorder()).
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

		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#504945")),

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
			Foreground(lipgloss.Color("#504945")),
	}
}

// ---------------------------------------------------------------------------
// nord — cool arctic
// ---------------------------------------------------------------------------

func NordTheme() Theme {
	hdrBg := "#3b4252"
	paneBg := "#2e3440"
	return Theme{
		HeaderBg: hdrBg,

		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#88c0d0")).
			Background(lipgloss.Color(paneBg)),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
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
			Border(lipgloss.RoundedBorder()).
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
			Foreground(lipgloss.Color("#434c5e")),
	}
}

// ---------------------------------------------------------------------------
// light — for daylight use
// ---------------------------------------------------------------------------

func LightTheme() Theme {
	hdrBg := "#0e7c7b"
	paneBg := "#f5f5f5"
	return Theme{
		HeaderBg: hdrBg,

		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#0e7c7b")).
			Background(lipgloss.Color(paneBg)),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
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
			Border(lipgloss.RoundedBorder()).
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
			Foreground(lipgloss.Color("#052424")),
	}
}
