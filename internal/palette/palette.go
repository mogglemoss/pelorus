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
	boxW := 64
	if m.Width > 0 && boxW > m.Width-4 {
		boxW = m.Width - 4
	}
	innerW := boxW - 4
	maxDisplayLines := 14

	var sb strings.Builder

	// Input field.
	inputLine := m.Theme.PaletteInput.Width(innerW).Render(m.Input.View())
	sb.WriteString(inputLine)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", innerW))
	sb.WriteString("\n")

	query := m.Input.Value()

	if query == "" {
		m.renderGrouped(&sb, innerW, maxDisplayLines)
	} else {
		m.renderFlat(&sb, innerW, maxDisplayLines)
	}

	// Footer.
	sb.WriteString("\n")
	footerStyle := m.Theme.PaletteItem.Copy().Faint(true)
	countStr := ""
	if len(m.Filtered) > 0 {
		countStr = fmt.Sprintf(" · %d/%d", m.Cursor+1, len(m.Filtered))
	}
	sb.WriteString(footerStyle.Width(innerW).Render("  ↑↓ navigate · enter select · esc close" + countStr))

	content := strings.TrimRight(sb.String(), "\n")
	return m.Theme.PaletteBox.Width(boxW).Render(content)
}

// renderGrouped renders actions grouped by category with headers.
func (m *Model) renderGrouped(sb *strings.Builder, innerW, maxLines int) {
	if len(m.Filtered) == 0 {
		empty := m.Theme.PaletteItem.Width(innerW).Render("No actions found")
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

	headerStyle := m.Theme.PaletteItem.Copy().
		Foreground(lipgloss.Color("#00a896")).
		Bold(true).
		Width(innerW)

	for _, di := range displayItems[winStart:winEnd] {
		if di.isHeader {
			sb.WriteString(headerStyle.Render("  " + strings.ToUpper(di.category)))
		} else {
			label := formatActionLabel(di.action, innerW)
			var style lipgloss.Style
			if di.actionIdx == m.Cursor {
				style = m.Theme.PaletteSelected.Width(innerW)
			} else {
				style = m.Theme.PaletteItem.Width(innerW)
			}
			sb.WriteString(style.Render(label))
		}
		sb.WriteString("\n")
	}
}

// renderFlat renders actions as a flat list (used when a query is active).
func (m *Model) renderFlat(sb *strings.Builder, innerW, maxItems int) {
	if len(m.Filtered) == 0 {
		empty := m.Theme.PaletteItem.Width(innerW).Render("No actions found")
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
		a := m.Filtered[i]
		label := formatActionLabel(a, innerW)
		var style lipgloss.Style
		if i == m.Cursor {
			style = m.Theme.PaletteSelected.Width(innerW)
		} else {
			style = m.Theme.PaletteItem.Width(innerW)
		}
		sb.WriteString(style.Render(label))
		sb.WriteString("\n")
	}
}

// formatActionLabel builds the display label for an action row.
func formatActionLabel(a actions.Action, maxW int) string {
	kb := a.Keybinding
	if kb == " " {
		kb = "space"
	}

	var label string
	if kb != "" {
		label = "  " + a.Name + " [" + kb + "]"
	} else {
		label = "  " + a.Name
	}
	if lipgloss.Width(label) > maxW {
		label = label[:maxW-1] + "…"
	}
	return label
}
