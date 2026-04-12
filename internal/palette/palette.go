package palette

import (
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

	// Build strings to search: "Name - Description"
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
// The caller must position/overlay this onto the main view.
func (m *Model) View() string {
	boxW := 60
	if m.Width > 0 && boxW > m.Width-4 {
		boxW = m.Width - 4
	}
	maxItems := 10

	var sb strings.Builder

	// Input field.
	inputLine := m.Theme.PaletteInput.Width(boxW - 4).Render(m.Input.View())
	sb.WriteString(inputLine)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", boxW-4))
	sb.WriteString("\n")

	// Items.
	shown := m.Filtered
	start := 0
	if m.Cursor >= maxItems {
		start = m.Cursor - maxItems + 1
	}
	end := start + maxItems
	if end > len(shown) {
		end = len(shown)
	}

	for i := start; i < end; i++ {
		a := shown[i]
		label := a.Name
		if a.Keybinding != "" {
			label = a.Name + " [" + a.Keybinding + "]"
		}
		if len(label) > boxW-6 {
			label = label[:boxW-9] + "…"
		}

		var style lipgloss.Style
		if i == m.Cursor {
			style = m.Theme.PaletteSelected.Width(boxW - 4)
		} else {
			style = m.Theme.PaletteItem.Width(boxW - 4)
		}
		sb.WriteString(style.Render(label))
		sb.WriteString("\n")
	}

	if len(m.Filtered) == 0 {
		empty := m.Theme.PaletteItem.Width(boxW - 4).Render("No actions found")
		sb.WriteString(empty)
		sb.WriteString("\n")
	}

	// Footer hint line.
	sb.WriteString("\n")
	footerStyle := m.Theme.PaletteItem.Copy().Faint(true)
	sb.WriteString(footerStyle.Width(boxW - 4).Render("  enter select · esc close"))

	content := strings.TrimRight(sb.String(), "\n")
	return m.Theme.PaletteBox.Width(boxW).Render(content)
}
