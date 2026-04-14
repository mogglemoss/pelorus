// Package palette implements the command palette overlay: a fuzzy-searchable
// modal list of all registered actions. The palette is designed as a
// two-column picker — action name and optional description on the left, key
// chip(s) on the right — grouped by category with accented section rules.
package palette

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/mogglemoss/pelorus/internal/actions"
	"github.com/mogglemoss/pelorus/internal/theme"
)

// ClosePaletteMsg is sent when the palette is closed without a selection.
type ClosePaletteMsg struct{}

// RunActionMsg is sent when an action is selected from the palette.
type RunActionMsg struct {
	ActionID string
}

// categoryOrder defines the display order for known categories.
var categoryOrder = []string{
	"Navigation", "File", "View", "App", "Custom",
}

// categoryGlyph returns a single-rune icon for a category, used as a prefix
// on category headers for quick visual scanning.
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

func categoryPriority(cat string) int {
	for i, c := range categoryOrder {
		if strings.EqualFold(c, cat) {
			return i
		}
	}
	return len(categoryOrder) // unknown categories go last
}

// sortActions sorts actions by category priority then name.
func sortActions(acts []actions.Action) {
	sort.SliceStable(acts, func(i, j int) bool {
		pi := categoryPriority(acts[i].Category)
		pj := categoryPriority(acts[j].Category)
		if pi != pj {
			return pi < pj
		}
		return acts[i].Name < acts[j].Name
	})
}

// displayItem is either a category header or an action row.
type displayItem struct {
	isHeader  bool
	category  string
	action    actions.Action
	actionIdx int // index in Filtered (only valid when !isHeader)
}

// buildDisplayItems groups actions by category, inserting header rows.
func buildDisplayItems(acts []actions.Action) []displayItem {
	var items []displayItem
	prevCat := ""
	for i, a := range acts {
		cat := a.Category
		if cat == "" {
			cat = "General"
		}
		if cat != prevCat {
			items = append(items, displayItem{isHeader: true, category: cat})
			prevCat = cat
		}
		items = append(items, displayItem{action: a, actionIdx: i})
	}
	return items
}

// Model is the Bubbletea model for the command palette overlay.
type Model struct {
	Input    textinput.Model
	Actions  []actions.Action
	Filtered []actions.Action
	Cursor   int
	Theme    *theme.Theme
	Width    int
	Height   int
}

// New creates a new palette model.
func New(reg *actions.Registry, t *theme.Theme) *Model {
	ti := textinput.New()
	ti.Placeholder = "Search actions…"
	ti.Prompt = ""
	ti.Focus()
	ti.CharLimit = 128

	allActions := reg.All()
	sortActions(allActions)

	return &Model{
		Input:    ti,
		Actions:  allActions,
		Filtered: allActions,
		Theme:    t,
	}
}

// Reset prepares the palette for a fresh open.
func (m *Model) Reset() {
	m.Input.SetValue("")
	m.Input.Focus()
	m.Filtered = m.Actions
	m.Cursor = 0
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles key events for the palette.
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.Cursor > 0 {
				m.Cursor--
			}
		case tea.MouseButtonWheelDown:
			if m.Cursor < len(m.Filtered)-1 {
				m.Cursor++
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			return m, func() tea.Msg { return ClosePaletteMsg{} }

		case tea.KeyEnter:
			if len(m.Filtered) > 0 {
				sel := m.Filtered[m.Cursor]
				return m, func() tea.Msg { return RunActionMsg{ActionID: sel.ID} }
			}
			return m, func() tea.Msg { return ClosePaletteMsg{} }

		case tea.KeyUp, tea.KeyCtrlK:
			if m.Cursor > 0 {
				m.Cursor--
			}

		case tea.KeyDown, tea.KeyCtrlJ:
			if m.Cursor < len(m.Filtered)-1 {
				m.Cursor++
			}

		default:
			m.Input, cmd = m.Input.Update(msg)
			m.applyFilter()
		}
	}

	return m, cmd
}

func (m *Model) applyFilter() {
	query := m.Input.Value()
	if query == "" {
		m.Filtered = m.Actions
		m.Cursor = 0
		return
	}

	targets := make([]string, len(m.Actions))
	for i, a := range m.Actions {
		targets[i] = a.Name + " " + a.Description + " " + a.Category
	}

	matches := fuzzy.Find(query, targets)
	m.Filtered = make([]actions.Action, 0, len(matches))
	for _, match := range matches {
		m.Filtered = append(m.Filtered, m.Actions[match.Index])
	}
	m.Cursor = 0
}

// View renders the palette as a centered overlay string.
func (m *Model) View() string {
	boxW := 72
	if m.Width > 0 && boxW > m.Width-4 {
		boxW = m.Width - 4
	}
	innerW := boxW - 4
	maxDisplayLines := 14

	t := m.Theme
	bg := t.PaletteBox.GetBackground()
	accentBg := lipgloss.NewStyle().Background(bg)

	var sb strings.Builder

	// --- Input row: accent prompt glyph + textinput, on the palette bg.
	promptStyle := t.PaletteInput.Copy().Bold(true)
	prompt := promptStyle.Render(" ❯ ")
	promptW := lipgloss.Width(prompt)
	inputField := t.PaletteInput.Copy().Width(innerW - promptW).Render(m.Input.View())
	sb.WriteString(prompt + inputField)
	sb.WriteString("\n")

	// Thin rule below the input.
	ruleStyle := lipgloss.NewStyle().Background(bg).Foreground(t.Divider.GetForeground())
	sb.WriteString(ruleStyle.Render(strings.Repeat("─", innerW)))
	sb.WriteString("\n")

	query := m.Input.Value()

	if query == "" {
		m.renderGrouped(&sb, innerW, maxDisplayLines)
	} else {
		m.renderFlat(&sb, innerW, maxDisplayLines)
	}

	// --- Footer: count pill on the right, hint on the left.
	sb.WriteString("\n")
	hint := " ↑↓ navigate  ·  enter run  ·  esc close"
	hintStyle := t.PaletteItem.Copy().Faint(true)
	countStr := ""
	if len(m.Filtered) > 0 {
		countStr = fmt.Sprintf("%d/%d ", m.Cursor+1, len(m.Filtered))
	}
	countStyle := t.PaletteCategoryHeader.Copy()
	countRender := countStyle.Render(countStr)
	countW := lipgloss.Width(countRender)
	pad := innerW - lipgloss.Width(hint) - countW
	if pad < 1 {
		pad = 1
	}
	sb.WriteString(hintStyle.Render(hint))
	sb.WriteString(accentBg.Width(pad).Render(""))
	sb.WriteString(countRender)

	content := strings.TrimRight(sb.String(), "\n")
	return t.PaletteBox.Width(boxW).Render(content)
}

// renderGrouped renders actions grouped by category with headers.
func (m *Model) renderGrouped(sb *strings.Builder, innerW, maxLines int) {
	if len(m.Filtered) == 0 {
		empty := m.Theme.PaletteItem.Width(innerW).Render("  no actions")
		sb.WriteString(empty)
		sb.WriteString("\n")
		return
	}

	displayItems := buildDisplayItems(m.Filtered)

	// Find the display-line index of the cursor action.
	cursorLine := 0
	for i, di := range displayItems {
		if !di.isHeader && di.actionIdx == m.Cursor {
			cursorLine = i
			break
		}
	}

	// Compute scroll window: keep cursor visible within maxLines.
	winStart := 0
	if cursorLine >= maxLines {
		winStart = cursorLine - maxLines + 1
	}
	winEnd := winStart + maxLines
	if winEnd > len(displayItems) {
		winEnd = len(displayItems)
	}

	for i, di := range displayItems[winStart:winEnd] {
		if di.isHeader {
			// Add a blank spacer row before any non-first header.
			if winStart+i > 0 {
				sb.WriteString(m.Theme.PaletteItem.Width(innerW).Render(""))
				sb.WriteString("\n")
			}
			sb.WriteString(m.renderCategoryHeader(di.category, innerW))
		} else {
			sb.WriteString(m.renderActionRow(di.action, di.actionIdx == m.Cursor, innerW))
		}
		sb.WriteString("\n")
	}
}

// renderFlat renders actions as a flat list (used when a query is active).
func (m *Model) renderFlat(sb *strings.Builder, innerW, maxItems int) {
	if len(m.Filtered) == 0 {
		empty := m.Theme.PaletteItem.Width(innerW).Render("  no matches")
		sb.WriteString(empty)
		sb.WriteString("\n")
		return
	}

	start := 0
	if m.Cursor >= maxItems {
		start = m.Cursor - maxItems + 1
	}
	end := start + maxItems
	if end > len(m.Filtered) {
		end = len(m.Filtered)
	}

	for i := start; i < end; i++ {
		sb.WriteString(m.renderActionRow(m.Filtered[i], i == m.Cursor, innerW))
		sb.WriteString("\n")
	}
}

// renderCategoryHeader renders an accented section header with a glyph and
// a trailing rule that fills to innerW.
//
//	 ⇄  NAVIGATION ─────────────────────────────
func (m *Model) renderCategoryHeader(category string, innerW int) string {
	t := m.Theme
	bg := t.PaletteBox.GetBackground()
	glyph := categoryGlyph(category)
	label := strings.ToUpper(category)
	head := " " + glyph + "  " + label + " "
	headRender := t.PaletteCategoryHeader.Render(head)
	rule := lipgloss.NewStyle().Foreground(t.Divider.GetForeground()).Background(bg)
	fill := innerW - lipgloss.Width(headRender)
	if fill < 0 {
		fill = 0
	}
	return headRender + rule.Render(strings.Repeat("─", fill))
}

// renderActionRow renders a single action row: left accent bar when selected,
// the action name + optional dim description on the left, right-aligned key
// chip(s) on the right.
//
//	▌  rename file              r
//	   copy to clipboard        y
func (m *Model) renderActionRow(a actions.Action, selected bool, innerW int) string {
	t := m.Theme
	bg := t.PaletteBox.GetBackground()

	// Gather keybindings for the right column.
	var kbs []string
	if a.Keybinding != "" {
		kb := a.Keybinding
		if kb == " " {
			kb = "space"
		}
		kbs = append(kbs, kb)
	}
	for _, k := range a.ExtraKeybindings {
		if k != "" {
			kbs = append(kbs, k)
		}
	}

	// Compose right-side key chips (each in FooterKey style on palette bg).
	var keyRender string
	var keyW int
	if len(kbs) > 0 {
		chipStyle := t.FooterKey.Copy().Background(bg).Bold(true)
		chips := make([]string, len(kbs))
		for i, k := range kbs {
			chips[i] = chipStyle.Render(" " + k + " ")
		}
		sepStyle := lipgloss.NewStyle().Background(bg).Foreground(t.StatusBarMuted.GetForeground())
		sep := sepStyle.Render(" ")
		keyRender = strings.Join(chips, sep) + " "
		keyW = lipgloss.Width(keyRender)
	}

	// Left gutter: selection bar, otherwise 2 spaces.
	var leftBar string
	var leftBarW int
	if selected {
		leftBar = t.PaletteSelected.Copy().Render("▌ ")
		leftBarW = lipgloss.Width(leftBar)
	} else {
		leftBar = lipgloss.NewStyle().Background(bg).Render("  ")
		leftBarW = 2
	}

	// Middle: name + optional description.
	midW := innerW - leftBarW - keyW
	if midW < 4 {
		midW = 4
	}

	var nameStyle, descStyle lipgloss.Style
	if selected {
		nameStyle = t.PaletteSelected.Copy()
		descStyle = t.PaletteSelected.Copy().Faint(true)
	} else {
		nameStyle = t.PaletteItem.Copy()
		descStyle = t.PaletteItem.Copy().Faint(true)
	}

	name := a.Name
	desc := a.Description
	// Compose name + "  " + desc, clipped to midW.
	middle := name
	if desc != "" && desc != name {
		middle = name + "  " + desc
	}
	if lipgloss.Width(middle) > midW {
		middle = middle[:midW-1] + "…"
	}
	// Split back for separate styling.
	var midRender string
	if desc != "" && desc != name && strings.HasPrefix(middle, name+"  ") {
		rest := strings.TrimPrefix(middle, name+"  ")
		midRender = nameStyle.Render(name) + nameStyle.Render("  ") + descStyle.Render(rest)
	} else {
		midRender = nameStyle.Render(middle)
	}
	// Pad middle to midW (background-aware).
	midActualW := lipgloss.Width(midRender)
	if midActualW < midW {
		pad := midW - midActualW
		bgStyle := lipgloss.NewStyle().Background(bg)
		if selected {
			bgStyle = t.PaletteSelected.Copy()
		}
		midRender += bgStyle.Render(strings.Repeat(" ", pad))
	}

	return leftBar + midRender + keyRender
}
