package search

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/mogglemoss/pelorus/internal/theme"
)

// ResultMsg is sent when a search result is selected.
type ResultMsg struct{ Path string }

// CloseSearchMsg is sent when the overlay is dismissed without selection.
type CloseSearchMsg struct{}

// SearchDoneMsg is sent when the async search completes.
type SearchDoneMsg struct {
	Paths []string
	Err   error
}

// Model is the search overlay.
type Model struct {
	Width  int
	Height int
	Theme  *theme.Theme

	input     textinput.Model
	allPaths  []string // raw results from fd/find
	filtered  []string // after fuzzy filter of display names
	cursor    int
	baseDir   string
	searching bool // fd is running
}

func New(t *theme.Theme) *Model {
	ti := textinput.New()
	ti.Placeholder = "Search files…"
	ti.CharLimit = 128
	ti.Focus()
	return &Model{Theme: t, input: ti}
}

// Open resets the model for a new search in the given directory.
func (m *Model) Open(dir string) tea.Cmd {
	m.baseDir = dir
	m.allPaths = nil
	m.filtered = nil
	m.cursor = 0
	m.searching = false
	m.input.SetValue("")
	m.input.Focus()
	return nil
}

// RunSearch executes fd or find asynchronously.
func RunSearch(dir string) tea.Cmd {
	return func() tea.Msg {
		// Prefer fd.
		var out []byte
		var err error
		if _, ferr := exec.LookPath("fd"); ferr == nil {
			cmd := exec.Command("fd", "--hidden", "--type", "f", "--type", "d", ".", dir)
			out, err = cmd.Output()
		} else {
			cmd := exec.Command("find", dir, "-not", "-path", "*/\\.git/*", "-not", "-name", ".DS_Store")
			out, err = cmd.Output()
		}
		if err != nil {
			// Partial output is OK.
		}
		var paths []string
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				paths = append(paths, line)
			}
		}
		return SearchDoneMsg{Paths: paths}
	}
}

func (m *Model) SetResults(paths []string) {
	m.allPaths = paths
	m.searching = false
	m.applyFilter()
}

func (m *Model) applyFilter() {
	query := m.input.Value()
	if query == "" {
		m.filtered = m.allPaths
		m.cursor = 0
		return
	}
	// Build display names for fuzzy matching (relative path).
	targets := make([]string, len(m.allPaths))
	for i, p := range m.allPaths {
		rel, err := filepath.Rel(m.baseDir, p)
		if err != nil {
			rel = p
		}
		targets[i] = rel
	}
	matches := fuzzy.Find(query, targets)
	m.filtered = make([]string, 0, len(matches))
	for _, match := range matches {
		m.filtered = append(m.filtered, m.allPaths[match.Index])
	}
	m.cursor = 0
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.MouseButtonWheelDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			return m, func() tea.Msg { return CloseSearchMsg{} }
		case tea.KeyEnter:
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				sel := m.filtered[m.cursor]
				return m, func() tea.Msg { return ResultMsg{Path: sel} }
			}
			return m, func() tea.Msg { return CloseSearchMsg{} }
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		default:
			var cmd tea.Cmd
			prev := m.input.Value()
			m.input, cmd = m.input.Update(msg)
			if m.input.Value() != prev {
				m.applyFilter()
			}
			return m, cmd
		}
	case SearchDoneMsg:
		m.SetResults(msg.Paths)
	}
	return m, nil
}

func (m *Model) View() string {
	boxW := 72
	if m.Width > 0 && boxW > m.Width-4 {
		boxW = m.Width - 4
	}
	innerW := boxW - 4
	maxItems := 16

	var sb strings.Builder

	inputLine := m.Theme.PaletteInput.Width(innerW).Render(m.input.View())
	sb.WriteString(inputLine)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", innerW))
	sb.WriteString("\n")

	if m.searching {
		sb.WriteString(m.Theme.PaletteItem.Width(innerW).Render("  Searching…"))
		sb.WriteString("\n")
	} else if len(m.filtered) == 0 && m.input.Value() != "" {
		sb.WriteString(m.Theme.PaletteItem.Width(innerW).Render("  No results"))
		sb.WriteString("\n")
	} else if len(m.allPaths) == 0 {
		sb.WriteString(m.Theme.PaletteItem.Width(innerW).Render("  Press ctrl+f again to search, or type to filter"))
		sb.WriteString("\n")
	} else {
		start := 0
		if m.cursor >= maxItems {
			start = m.cursor - maxItems + 1
		}
		end := start + maxItems
		if end > len(m.filtered) {
			end = len(m.filtered)
		}
		for i := start; i < end; i++ {
			p := m.filtered[i]
			rel, err := filepath.Rel(m.baseDir, p)
			if err != nil {
				rel = p
			}
			label := "  " + rel
			if lipgloss.Width(label) > innerW {
				label = "  …" + rel[len(rel)-(innerW-4):]
			}
			var style lipgloss.Style
			if i == m.cursor {
				style = m.Theme.PaletteSelected.Width(innerW)
			} else {
				style = m.Theme.PaletteItem.Width(innerW)
			}
			sb.WriteString(style.Render(label))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	countStr := ""
	if len(m.filtered) > 0 {
		countStr = fmt.Sprintf(" · %d/%d", m.cursor+1, len(m.filtered))
	}
	footer := m.Theme.PaletteItem.Copy().Faint(true).Width(innerW).Render("  ↑↓ navigate · enter open · esc close" + countStr)
	sb.WriteString(footer)

	content := strings.TrimRight(sb.String(), "\n")
	return m.Theme.PaletteBox.Width(boxW).Render(content)
}
