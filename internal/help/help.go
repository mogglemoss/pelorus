package help

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mogglemoss/pelorus/internal/actions"
	"github.com/mogglemoss/pelorus/internal/theme"
)

// CloseHelpMsg is emitted when the help overlay is dismissed.
type CloseHelpMsg struct{}


// Model is the Bubbletea model for the keybinding help overlay.
type Model struct {
	Width  int
	Height int
	Theme  *theme.Theme
	vp     viewport.Model
	reg    *actions.Registry
}

// New creates a new help overlay model.
func New(reg *actions.Registry, t *theme.Theme) *Model {
	vp := viewport.New(0, 0)
	return &Model{
		vp:    vp,
		reg:   reg,
		Theme: t,
	}
}

// Open builds the help content from the registry and resets the viewport to the top.
func (m *Model) Open() {
	content := m.buildContent()
	// Account for border (2), title line (1), separator (1), footer (1) = 5 overhead lines.
	// Plus 2 for top/bottom padding.
	vpH := m.Height - 7
	if vpH < 1 {
		vpH = 1
	}
	vpW := m.Width - 4 // 2 for border + 2 for padding
	if vpW < 10 {
		vpW = 10
	}
	m.vp.Width = vpW
	m.vp.Height = vpH
	m.vp.SetContent(content)
	m.vp.GotoTop()
}

// buildContent assembles the full help text from registry actions.
func (m *Model) buildContent() string {
	allActions := m.reg.All()

	// Group by category.
	catMap := make(map[string][]actions.Action)
	for _, a := range allActions {
		if a.Keybinding == "" {
			continue
		}
		catMap[a.Category] = append(catMap[a.Category], a)
	}

	// Sort categories alphabetically.
	categories := make([]string, 0, len(catMap))
	for cat := range catMap {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	t := m.Theme
	bg := t.PaletteBox.GetBackground()
	catStyle := t.SectionLabel.Copy()
	sepStyle := lipgloss.NewStyle().Background(bg).Foreground(t.StatusBarMuted.GetForeground())
	keyStyle := t.FooterKey.Copy().Background(bg)
	nameStyle := t.PaletteItem.Copy()

	var sb strings.Builder

	for i, cat := range categories {
		acts := catMap[cat]
		// Sort actions within category alphabetically by Name.
		sort.Slice(acts, func(x, y int) bool {
			return acts[x].Name < acts[y].Name
		})

		if i > 0 {
			sb.WriteString("\n")
		}

		// Category header.
		sb.WriteString("  ")
		sb.WriteString(catStyle.Render(cat))
		sb.WriteString("\n")
		sb.WriteString("  ")
		sb.WriteString(sepStyle.Render(strings.Repeat("─", 38)))
		sb.WriteString("\n")

		for _, a := range acts {
			// Left-pad key to 12 chars.
			key := a.Keybinding
			padded := fmt.Sprintf("%-12s", key)
			sb.WriteString("  ")
			sb.WriteString(keyStyle.Render(padded))
			sb.WriteString("  ")
			sb.WriteString(nameStyle.Render(a.Name))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// Update handles key events for the help overlay.
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "?":
			return m, func() tea.Msg { return CloseHelpMsg{} }
		case "j", "down":
			m.vp.LineDown(1)
		case "k", "up":
			m.vp.LineUp(1)
		case "d":
			m.vp.HalfViewDown()
		case "u":
			m.vp.HalfViewUp()
		}
	}
	return m, nil
}

// View renders the full-screen help overlay.
func (m *Model) View() string {
	if m.Width == 0 || m.Height == 0 {
		return ""
	}

	// Styles — all derived from the active theme.
	t := m.Theme
	bg := t.PaletteBox.GetBackground()
	baseBg := lipgloss.NewStyle().Background(bg)
	_ = baseBg

	boxStyle := t.PaletteBox.Copy().Padding(0, 1)

	titleStyle := t.HeaderTitle.Copy().Background(bg)

	footerStyle := t.StatusBarMuted.Copy().Background(bg)

	sepStyle := lipgloss.NewStyle().
		Foreground(t.StatusBarMuted.GetForeground()).
		Background(bg)

	innerW := m.Width - 4 // border (2) + padding (2)
	if innerW < 10 {
		innerW = 10
	}

	// Title line centered.
	titleText := "Pelorus — Keybindings"
	titleRendered := titleStyle.Render(titleText)
	titleLine := lipgloss.PlaceHorizontal(innerW, lipgloss.Center, titleRendered)

	// Separator below title.
	sep := sepStyle.Render(strings.Repeat("─", innerW))

	// Footer.
	footerText := "j/k scroll · esc close"
	footerRendered := footerStyle.Render(footerText)
	footerLine := lipgloss.PlaceHorizontal(innerW, lipgloss.Left, footerRendered)

	// Viewport content.
	vpView := m.vp.View()

	// Compose inner content.
	inner := strings.Join([]string{
		titleLine,
		sep,
		vpView,
		sep,
		footerLine,
	}, "\n")

	box := boxStyle.Width(innerW).Render(inner)
	return box
}
