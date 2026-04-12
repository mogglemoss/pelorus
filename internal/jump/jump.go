package jump

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/mogglemoss/pelorus/internal/theme"
)

// --- Message types ---

// JumpToMsg is emitted when the user selects an entry from the jump list.
type JumpToMsg struct{ Path string }

// CloseJumpMsg is emitted when the jump list is closed without a selection.
type CloseJumpMsg struct{}

// --- Store ---

// Entry represents a single directory in the jump store.
type Entry struct {
	Path      string    `json:"path"`
	Score     float64   `json:"score"`     // frecency score
	Name      string    `json:"name"`      // optional display name (empty = use path)
	Pinned    bool      `json:"pinned"`    // true = manually bookmarked
	LastVisit time.Time `json:"last_visit"`
}

// Store holds the full jump list and its backing file path.
type Store struct {
	Entries []*Entry `json:"entries"`
	path    string   // path to the JSON file
}

// NewStore returns an empty in-memory store with no backing file.
func NewStore() *Store {
	return &Store{}
}

// LoadStore loads the store from the XDG data directory.
// If the file does not exist it returns an empty store.
func LoadStore() (*Store, error) {
	p, err := xdg.DataFile("pelorus/jumps.json")
	if err != nil {
		return &Store{path: p}, err
	}

	s := &Store{path: p}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}

	if err := json.Unmarshal(data, s); err != nil {
		// Corrupt file — start fresh.
		return &Store{path: p}, nil
	}
	return s, nil
}

// Save writes the store to disk.
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// Visit increments the frecency score for dir, adding it if new.
func (s *Store) Visit(dir string) {
	for _, e := range s.Entries {
		if e.Path == dir {
			e.Score = e.Score*0.9 + 10
			e.LastVisit = time.Now()
			return
		}
	}
	// New entry.
	s.Entries = append(s.Entries, &Entry{
		Path:      dir,
		Score:     10,
		LastVisit: time.Now(),
	})
}

// Pin bookmarks dir with an optional display name.
// If dir already exists its Pinned flag and Name are updated.
func (s *Store) Pin(dir, name string) {
	for _, e := range s.Entries {
		if e.Path == dir {
			e.Pinned = true
			if name != "" {
				e.Name = name
			}
			return
		}
	}
	s.Entries = append(s.Entries, &Entry{
		Path:      dir,
		Score:     10,
		Name:      name,
		Pinned:    true,
		LastVisit: time.Now(),
	})
}

// All returns a sorted copy of entries: pinned first, then by score descending.
func (s *Store) All() []*Entry {
	out := make([]*Entry, len(s.Entries))
	copy(out, s.Entries)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned // pinned sorts first
		}
		return out[i].Score > out[j].Score
	})
	return out
}

// Remove deletes the entry for dir.
func (s *Store) Remove(dir string) {
	filtered := s.Entries[:0]
	for _, e := range s.Entries {
		if e.Path != dir {
			filtered = append(filtered, e)
		}
	}
	s.Entries = filtered
}

// --- Overlay model ---

// Model is the Bubbletea model for the jump list overlay.
type Model struct {
	Width  int
	Height int
	Theme  *theme.Theme

	store    *Store
	input    textinput.Model
	filtered []*Entry
	cursor   int
	active   bool
}

// NewModel creates a new jump overlay model.
func NewModel(store *Store, t *theme.Theme) *Model {
	ti := textinput.New()
	ti.Placeholder = "Jump to…"
	ti.CharLimit = 128

	return &Model{
		store: store,
		input: ti,
		Theme: t,
	}
}

// Open resets the overlay and makes it visible.
func (m *Model) Open() {
	m.input.SetValue("")
	m.input.Focus()
	m.cursor = 0
	m.filtered = m.store.All()
	m.active = true
}

// IsActive reports whether the overlay is currently shown.
func (m *Model) IsActive() bool {
	return m.active
}

// Update handles key events while the jump list is open.
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.Type {
	case tea.KeyEsc:
		m.active = false
		return m, func() tea.Msg { return CloseJumpMsg{} }

	case tea.KeyEnter:
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			sel := m.filtered[m.cursor]
			m.active = false
			path := sel.Path
			return m, func() tea.Msg { return JumpToMsg{Path: path} }
		}
		m.active = false
		return m, func() tea.Msg { return CloseJumpMsg{} }

	case tea.KeyDown:
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		return m, nil

	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case tea.KeyRunes:
		ch := keyMsg.String()
		// j/k navigation when no query text (vim-style).
		if m.input.Value() == "" {
			if ch == "j" {
				if m.cursor < len(m.filtered)-1 {
					m.cursor++
				}
				return m, nil
			}
			if ch == "k" {
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			}
		}
		// Otherwise feed into the text input.
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.applyFilter()
		return m, cmd

	case tea.KeyBackspace:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.applyFilter()
		return m, cmd

	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.applyFilter()
		return m, cmd
	}
}

// applyFilter re-computes m.filtered based on the current input value.
func (m *Model) applyFilter() {
	query := m.input.Value()
	all := m.store.All()

	if query == "" {
		m.filtered = all
		m.cursor = 0
		return
	}

	// Build display strings for fuzzy matching.
	targets := make([]string, len(all))
	for i, e := range all {
		if e.Name != "" {
			targets[i] = e.Name + " " + e.Path
		} else {
			targets[i] = filepath.Base(e.Path) + " " + e.Path
		}
	}

	matches := fuzzy.Find(query, targets)
	m.filtered = make([]*Entry, 0, len(matches))
	for _, match := range matches {
		m.filtered = append(m.filtered, all[match.Index])
	}
	m.cursor = 0
}

// View renders the jump list as an overlay string.
func (m *Model) View() string {
	boxW := 64
	if m.Width > 0 && boxW > m.Width-4 {
		boxW = m.Width - 4
	}
	maxItems := 12

	var sb strings.Builder

	// Input field.
	inputLine := m.Theme.PaletteInput.Width(boxW - 4).Render(m.input.View())
	sb.WriteString(inputLine)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", boxW-4))
	sb.WriteString("\n")

	// Entry list.
	shown := m.filtered
	start := 0
	if m.cursor >= maxItems {
		start = m.cursor - maxItems + 1
	}
	end := start + maxItems
	if end > len(shown) {
		end = len(shown)
	}

	for i := start; i < end; i++ {
		e := shown[i]

		// Build label.
		var label string
		if e.Name != "" {
			label = e.Name + "  " + shortenPath(e.Path, boxW-10)
		} else {
			label = shortenPath(e.Path, boxW-6)
		}

		// Pinned indicator.
		pinIndicator := "  "
		if e.Pinned {
			pinIndicator = "★ "
		}

		full := pinIndicator + label
		if lipgloss.Width(full) > boxW-4 {
			full = full[:boxW-7] + "…"
		}

		var style lipgloss.Style
		if i == m.cursor {
			style = m.Theme.PaletteSelected.Width(boxW - 4)
		} else if e.Pinned {
			style = m.Theme.PaletteItem.
				Width(boxW - 4).
				Foreground(lipgloss.Color("#00ffd0"))
		} else {
			style = m.Theme.PaletteItem.Width(boxW - 4)
		}
		sb.WriteString(style.Render(full))
		sb.WriteString("\n")
	}

	if len(m.filtered) == 0 {
		empty := m.Theme.PaletteItem.Width(boxW - 4).Render("No entries found")
		sb.WriteString(empty)
		sb.WriteString("\n")
	}

	content := strings.TrimRight(sb.String(), "\n")
	return m.Theme.PaletteBox.Width(boxW).Render(content)
}

// shortenPath shortens a path to fit within maxWidth, prefixing with "…" if needed.
func shortenPath(p string, maxWidth int) string {
	if maxWidth <= 0 {
		return p
	}
	if len(p) <= maxWidth {
		return p
	}
	return "…" + p[len(p)-maxWidth+1:]
}
