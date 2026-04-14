// Package help renders the full-screen keybinding help overlay.
//
// The layout is a responsive grid: on wide terminals the category groups
// flow into two columns side-by-side; on narrow terminals they stack into
// a single column. Each row renders the keybinding as a chip on the left
// and the action name to its right — reading order matches how a user
// actually thinks ("what does ^p do?" → scan keys, not names).
package help

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mogglemoss/pelorus/internal/actions"
	"github.com/mogglemoss/pelorus/internal/theme"
	"github.com/mogglemoss/pelorus/internal/version"
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

// categoryGlyph returns a single-rune icon for a category.
func categoryGlyph(cat string) string {
	switch strings.ToLower(cat) {
	case "navigation":
		return "⇄"
	case "file":
		return "▤"
	case "view":
		return "▦"
	case "app":
		return "⌘"
	case "custom":
		return "✦"
	default:
		return "·"
	}
}

// categoryOrder controls the display priority for known categories.
var categoryOrder = []string{"Navigation", "File", "View", "App", "Custom"}

func categoryPriority(cat string) int {
	for i, c := range categoryOrder {
		if strings.EqualFold(c, cat) {
			return i
		}
	}
	return len(categoryOrder)
}

// Open builds the help content from the registry and resets the viewport to the top.
func (m *Model) Open() {
	// Account for border (2), title line (1), rule (1), padding (2), footer (1) = 7.
	vpH := m.Height - 7
	if vpH < 1 {
		vpH = 1
	}
	vpW := m.Width - 4
	if vpW < 10 {
		vpW = 10
	}
	m.vp.Width = vpW
	m.vp.Height = vpH
	m.vp.SetContent(m.buildContent(vpW))
	m.vp.GotoTop()
}

// buildContent lays out category groups either side-by-side (wide) or stacked.
func (m *Model) buildContent(width int) string {
	allActions := m.reg.All()

	// Group by category, filtering to actions with a visible keybinding.
	catMap := make(map[string][]actions.Action)
	for _, a := range allActions {
		if a.Keybinding == "" {
			continue
		}
		catMap[a.Category] = append(catMap[a.Category], a)
	}

	// Category order: known priorities first, then alphabetical for the rest.
	categories := make([]string, 0, len(catMap))
	for cat := range catMap {
		categories = append(categories, cat)
	}
	sort.Slice(categories, func(i, j int) bool {
		pi, pj := categoryPriority(categories[i]), categoryPriority(categories[j])
		if pi != pj {
			return pi < pj
		}
		return categories[i] < categories[j]
	})

	// Sort actions inside each category by name.
	for _, cat := range categories {
		acts := catMap[cat]
		sort.Slice(acts, func(x, y int) bool { return acts[x].Name < acts[y].Name })
		catMap[cat] = acts
	}

	// Decide columns: 2 columns when at least ~64 cols available.
	cols := 1
	if width >= 64 {
		cols = 2
	}
	colW := width / cols
	if cols > 1 {
		colW = (width - 2) / cols // 2-char gutter between columns
	}

	// Render each category as a block of lines.
	blocks := make([][]string, len(categories))
	for i, cat := range categories {
		blocks[i] = m.renderCategoryBlock(cat, catMap[cat], colW)
	}

	// Greedy bin-pack categories into N columns, balancing by line count.
	columns := make([][]string, cols)
	heights := make([]int, cols)
	for i, block := range blocks {
		// Pick the shortest column.
		target := 0
		for c := 1; c < cols; c++ {
			if heights[c] < heights[target] {
				target = c
			}
		}
		if i > 0 && len(columns[target]) > 0 {
			columns[target] = append(columns[target], "")
			heights[target]++
		}
		columns[target] = append(columns[target], block...)
		heights[target] += len(block)
	}

	// Equalize column heights.
	maxH := 0
	for _, h := range heights {
		if h > maxH {
			maxH = h
		}
	}
	for c := range columns {
		for len(columns[c]) < maxH {
			columns[c] = append(columns[c], strings.Repeat(" ", colW))
		}
	}

	// Zip columns into rows.
	gutter := "  "
	var sb strings.Builder
	for row := 0; row < maxH; row++ {
		for c := 0; c < cols; c++ {
			line := ""
			if row < len(columns[c]) {
				line = columns[c][row]
			}
			// Pad each column cell to colW.
			w := lipgloss.Width(line)
			if w < colW {
				line += strings.Repeat(" ", colW-w)
			}
			if c > 0 {
				sb.WriteString(gutter)
			}
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// renderCategoryBlock produces the lines for one category: header + rule +
// rows. Each row is "  [key]  action name" with the key as a chip.
func (m *Model) renderCategoryBlock(cat string, acts []actions.Action, width int) []string {
	t := m.Theme

	// Header: glyph + uppercase label, trailing rule fills to width.
	glyph := categoryGlyph(cat)
	head := " " + glyph + "  " + strings.ToUpper(cat) + " "
	headRender := t.PaletteCategoryHeader.Render(head)
	ruleStyle := lipgloss.NewStyle().Foreground(t.Divider.GetForeground())
	fill := width - lipgloss.Width(headRender)
	if fill < 0 {
		fill = 0
	}
	header := headRender + ruleStyle.Render(strings.Repeat("─", fill))

	// Compute the widest key chip so names align in a clean second column.
	keyW := 0
	for _, a := range acts {
		k := a.Keybinding
		if k == " " {
			k = "space"
		}
		w := lipgloss.Width(" " + k + " ")
		if w > keyW {
			keyW = w
		}
	}
	if keyW < 5 {
		keyW = 5
	}

	keyStyle := t.FooterKey.Copy().Bold(true)
	nameStyle := t.PaletteItem.Copy()

	lines := []string{header}
	for _, a := range acts {
		k := a.Keybinding
		if k == " " {
			k = "space"
		}
		chip := keyStyle.Render(" " + k + " ")
		chipW := lipgloss.Width(chip)
		chipPad := ""
		if chipW < keyW {
			chipPad = strings.Repeat(" ", keyW-chipW)
		}

		name := a.Name
		// Max name width = total width - 2 (left pad) - keyW - 2 (gap).
		maxName := width - 2 - keyW - 2
		if maxName < 4 {
			maxName = 4
		}
		if lipgloss.Width(name) > maxName {
			name = name[:maxName-1] + "…"
		}
		line := "  " + chip + chipPad + "  " + nameStyle.Render(name)
		lines = append(lines, line)
	}
	return lines
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
		case "d", "pgdown":
			m.vp.HalfViewDown()
		case "u", "pgup":
			m.vp.HalfViewUp()
		case "g", "home":
			m.vp.GotoTop()
		case "G", "end":
			m.vp.GotoBottom()
		}
	}
	return m, nil
}

// View renders the full-screen help overlay.
func (m *Model) View() string {
	if m.Width == 0 || m.Height == 0 {
		return ""
	}

	t := m.Theme
	bg := t.PaletteBox.GetBackground()
	boxStyle := t.PaletteBox.Copy().Padding(0, 1)

	innerW := m.Width - 4 // border (2) + padding (2)
	if innerW < 10 {
		innerW = 10
	}

	// Title: accented glyph + title, centered.
	titleGlyph := t.PaletteCategoryHeader.Render(" ⌘ ")
	titleStyle := t.HeaderTitle.Copy().Background(bg).Bold(true)
	titleText := titleGlyph + titleStyle.Render("  "+version.Title()+" — Keybindings  ") + titleGlyph
	titleLine := lipgloss.PlaceHorizontal(innerW, lipgloss.Center, titleText,
		lipgloss.WithWhitespaceBackground(bg))

	// Rule under the title, full-width.
	ruleStyle := lipgloss.NewStyle().Foreground(t.Divider.GetForeground()).Background(bg)
	rule := ruleStyle.Render(strings.Repeat("─", innerW))

	// Footer: left-aligned key hints with accent chips.
	footer := m.renderFooter(innerW)

	// Body (viewport content).
	vpView := m.vp.View()

	inner := strings.Join([]string{
		titleLine,
		rule,
		vpView,
		rule,
		footer,
	}, "\n")

	return boxStyle.Width(innerW).Render(inner)
}

// renderFooter builds the navigation hint strip at the bottom of the overlay.
func (m *Model) renderFooter(innerW int) string {
	t := m.Theme
	bg := t.PaletteBox.GetBackground()
	chip := t.FooterKey.Copy().Background(bg).Bold(true)
	desc := t.StatusBarMuted.Copy().Background(bg)
	bgStyle := lipgloss.NewStyle().Background(bg)

	type pair struct{ key, label string }
	pairs := []pair{
		{"↑↓", "scroll"},
		{"d/u", "half-page"},
		{"g/G", "top/end"},
		{"esc", "close"},
	}
	var sb strings.Builder
	for i, p := range pairs {
		if i > 0 {
			sb.WriteString(bgStyle.Render("   "))
		}
		sb.WriteString(chip.Render(" " + p.key + " "))
		sb.WriteString(bgStyle.Render(" "))
		sb.WriteString(desc.Render(p.label))
	}
	line := sb.String()
	pad := innerW - lipgloss.Width(line)
	if pad > 0 {
		line += bgStyle.Render(strings.Repeat(" ", pad))
	}
	return line
}
