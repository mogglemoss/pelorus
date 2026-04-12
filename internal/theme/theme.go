package theme

import "github.com/charmbracelet/lipgloss"

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
}

// Color palette for the pelorus "retrofuture subaquatic" aesthetic.
const (
	colorBg          = "#0a0f14" // near-black deep ocean
	colorBgPane      = "#0d1520" // slightly lighter pane bg
	colorPrimary     = "#0e7c7b" // deep teal
	colorAccent      = "#00ffd0" // bioluminescent cyan-green
	colorAccentDim   = "#00a896" // dimmer accent
	colorText        = "#c8d8e8" // cool light blue-white
	colorTextDim     = "#4a6070" // dim blue-grey
	colorDir         = "#4fc3f7" // sky cyan for dirs
	colorSymlink     = "#b39ddb" // purple for symlinks
	colorBorderActive  = "#00ffd0" // bright accent border
	colorBorderInactive = "#1a3040" // dark dim border
	colorCursorBg    = "#0e4060" // deep teal highlight
	colorCursorFg    = "#00ffd0" // bright accent text
	colorStatus      = "#081018" // very dark status bar bg
	colorPaletteBg   = "#0d1a26" // palette box bg
	colorSelected    = "#00ffd0" // selected item fg
	colorSelectedBg  = "#0a3050" // selected item bg
)

// PelorusTheme returns the default pelorus theme.
func PelorusTheme() Theme {
	return Theme{
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
			Background(lipgloss.Color(colorPrimary)),

		HeaderTitle: lipgloss.NewStyle().
			Background(lipgloss.Color(colorPrimary)).
			Foreground(lipgloss.Color(colorAccent)).
			Bold(true).
			Padding(0, 1),

		HeaderPath: lipgloss.NewStyle().
			Background(lipgloss.Color(colorPrimary)).
			Foreground(lipgloss.Color("#b2d8d8")).
			Faint(true),

		HeaderHint: lipgloss.NewStyle().
			Background(lipgloss.Color(colorPrimary)).
			Foreground(lipgloss.Color("#052424")).
			Faint(true),
	}
}
