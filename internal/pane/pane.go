package pane

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/mogglemoss/pelorus/internal/nav"
	"github.com/mogglemoss/pelorus/internal/provider"
	"github.com/mogglemoss/pelorus/internal/provider/archive"
	"github.com/mogglemoss/pelorus/internal/theme"
	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

// Mode describes what the pane is currently doing.
type Mode int

const (
	ModeNormal Mode = iota
	ModeRename
	ModeNewFile
	ModeNewDir
	ModeConfirmDelete
	ModeFilter
	ModeGotoPath
)

// archiveFrame records the pane state prior to entering an archive so we can
// restore it when the user navigates above the archive root.
type archiveFrame struct {
	prevPath     string
	prevProvider provider.Provider
}

// Model is the Bubbletea model for a single pane.
type Model struct {
	Path       string
	Provider   provider.Provider
	Entries    []fileinfo.FileInfo // all visible entries
	Filtered   []fileinfo.FileInfo // after fuzzy filter (== Entries when filter empty)
	Cursor     int
	IsActive   bool
	ShowHidden bool

	// Archive navigation stack.
	archiveStack []archiveFrame

	// Fuzzy filter state
	FilterStr string

	// Inline input (rename / new file / new dir)
	Mode      Mode
	Input     textinput.Model
	ConfirmTarget fileinfo.FileInfo // file being deleted

	// Dimensions
	Width  int
	Height int

	Theme *theme.Theme
}

// New creates a new pane model.
func New(path string, p provider.Provider, t *theme.Theme, showHidden bool) *Model {
	ti := textinput.New()
	ti.CharLimit = 256

	m := &Model{
		Path:       path,
		Provider:   p,
		Theme:      t,
		ShowHidden: showHidden,
		Input:      ti,
	}
	m.reload()
	return m
}

// reload refreshes entries from the provider.
func (m *Model) reload() {
	entries, err := nav.ReadDir(m.Path, m.Provider, m.ShowHidden)
	if err != nil {
		m.Entries = nil
	} else {
		m.Entries = entries
	}
	m.FilterStr = ""
	m.Filtered = m.Entries
	m.clampCursor()
}

// Selected returns the currently highlighted FileInfo, or nil.
func (m *Model) Selected() *fileinfo.FileInfo {
	if len(m.Filtered) == 0 {
		return nil
	}
	if m.Cursor < 0 || m.Cursor >= len(m.Filtered) {
		return nil
	}
	fi := m.Filtered[m.Cursor]
	return &fi
}

func (m *Model) clampCursor() {
	if len(m.Filtered) == 0 {
		m.Cursor = 0
		return
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
	if m.Cursor >= len(m.Filtered) {
		m.Cursor = len(m.Filtered) - 1
	}
}

// applyFilter recomputes Filtered using the current FilterStr.
func (m *Model) applyFilter() {
	if m.FilterStr == "" {
		m.Filtered = m.Entries
		m.clampCursor()
		return
	}
	names := make([]string, len(m.Entries))
	for i, e := range m.Entries {
		names[i] = e.Name
	}
	matches := fuzzy.Find(m.FilterStr, names)
	m.Filtered = make([]fileinfo.FileInfo, 0, len(matches))
	for _, match := range matches {
		m.Filtered = append(m.Filtered, m.Entries[match.Index])
	}
	m.Cursor = 0
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles messages directed at this pane.
// Returns whether the message was consumed.
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.Mode {
	case ModeRename, ModeNewFile, ModeNewDir, ModeGotoPath:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEnter:
				cmd = m.commitInput()
			case tea.KeyEsc:
				m.Mode = ModeNormal
				m.Input.Blur()
			default:
				m.Input, cmd = m.Input.Update(msg)
			}
		}
		return m, cmd

	case ModeConfirmDelete:
		if kMsg, ok := msg.(tea.KeyMsg); ok {
			switch strings.ToLower(kMsg.String()) {
			case "y", "enter":
				cmd = m.doDelete()
			default:
				m.Mode = ModeNormal
			}
		}
		return m, cmd

	case ModeFilter:
		if kMsg, ok := msg.(tea.KeyMsg); ok {
			switch kMsg.Type {
			case tea.KeyEsc:
				m.FilterStr = ""
				m.applyFilter()
				m.Mode = ModeNormal
			case tea.KeyBackspace:
				if len(m.FilterStr) > 0 {
					m.FilterStr = m.FilterStr[:len(m.FilterStr)-1]
					m.applyFilter()
				} else {
					m.Mode = ModeNormal
				}
			case tea.KeyRunes:
				m.FilterStr += kMsg.String()
				m.applyFilter()
			}
		}
		return m, nil
	}

	// ModeNormal — handled by app-level keybindings, not here.
	return m, nil
}

func (m *Model) commitInput() tea.Cmd {
	val := strings.TrimSpace(m.Input.Value())
	m.Input.Blur()

	if val == "" {
		m.Mode = ModeNormal
		return nil
	}

	var err error
	switch m.Mode {
	case ModeRename:
		sel := m.Selected()
		if sel != nil {
			dst := filepath.Join(m.Path, val)
			err = m.Provider.Rename(sel.Path, dst)
		}
	case ModeNewFile:
		target := filepath.Join(m.Path, val)
		f, ferr := os.Create(target)
		if ferr == nil {
			f.Close()
		}
		err = ferr
	case ModeNewDir:
		err = m.Provider.MakeDir(filepath.Join(m.Path, val))
	case ModeGotoPath:
		target := expandPath(val)
		info, serr := os.Stat(target)
		if serr != nil {
			err = serr
		} else if !info.IsDir() {
			err = fmt.Errorf("%q is not a directory", target)
		} else {
			m.Mode = ModeNormal
			m.Path = target
			m.archiveStack = nil // clear archive stack on explicit navigation
			m.reload()
			return nil
		}
	}

	m.Mode = ModeNormal
	m.reload()

	if err != nil {
		return func() tea.Msg { return ErrMsg{Err: err} }
	}
	return nil
}

// expandPath expands ~ and environment variables in a path.
func expandPath(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			p = home + p[1:]
		}
	}
	return os.ExpandEnv(p)
}

// DeleteConfirmedMsg is emitted when the user confirms deletion in ModeConfirmDelete.
// The app handles the actual deletion so it can enqueue it as a background job.
type DeleteConfirmedMsg struct {
	Path     string
	Provider provider.Provider
}

func (m *Model) doDelete() tea.Cmd {
	m.Mode = ModeNormal
	target := m.ConfirmTarget
	prov := m.Provider
	// Reload optimistically (the file will disappear when the job finishes).
	m.reload()
	return func() tea.Msg {
		return DeleteConfirmedMsg{Path: target.Path, Provider: prov}
	}
}

// ErrMsg carries an error to be shown in the status bar.
type ErrMsg struct{ Err error }

// --- Navigation helpers (called by app) ---

func (m *Model) CursorUp() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

func (m *Model) CursorDown() {
	if m.Cursor < len(m.Filtered)-1 {
		m.Cursor++
	}
}

func (m *Model) EnterSelected() tea.Cmd {
	sel := m.Selected()
	if sel == nil {
		return nil
	}

	if sel.IsDir {
		m.Path = sel.Path
		m.reload()
		return nil
	}

	// When inside an archive and the entry has an internal path, build the
	// proper virtual path before checking the extension.
	entryName := sel.Name

	// Check if it's a supported archive.
	if isArchivePath(entryName) {
		// Compute the real filesystem path for the archive file.
		var archPath string
		if ap, ok := m.Provider.(*archive.Provider); ok {
			// We're already inside an archive — sel.Path is an internal path.
			// For now, nested archives are not supported; treat as no-op.
			_ = ap
			return nil
		}
		archPath = sel.Path

		archProv, err := archive.New(archPath, m.Provider)
		if err == nil {
			m.archiveStack = append(m.archiveStack, archiveFrame{
				prevPath:     m.Path,
				prevProvider: m.Provider,
			})
			m.Provider = archProv
			m.Path = archPath // virtual root == archive path (List called with archivePath)
			m.reload()
		}
		return nil
	}

	// Non-dir, non-archive: signal the app to open in editor.
	return func() tea.Msg { return OpenFileMsg{Path: sel.Path} }
}

// OpenFileMsg is emitted when the user presses enter on a regular file.
// The app opens it in the configured editor.
type OpenFileMsg struct{ Path string }

// isArchivePath reports whether a filename has a supported archive extension.
func isArchivePath(name string) bool {
	return archive.IsArchive(name)
}

func (m *Model) GoParent() {
	// If the current provider is an archive, check whether we should pop the
	// archive stack instead of navigating within the archive.
	if ap, ok := m.Provider.(*archive.Provider); ok {
		archRoot := ap.ArchivePath() // real path of the archive file
		// Navigate up within the archive virtual tree.
		// m.Path is either archRoot (the virtual root) or archRoot+"/"+subpath.
		if m.Path == archRoot || m.Path == archRoot+"/" {
			// We're at the archive root — pop back to the real filesystem.
			if len(m.archiveStack) > 0 {
				frame := m.archiveStack[len(m.archiveStack)-1]
				m.archiveStack = m.archiveStack[:len(m.archiveStack)-1]
				m.Provider = frame.prevProvider
				m.Path = frame.prevPath
				m.reload()
			}
			return
		}
		// Otherwise go up one level inside the virtual archive tree.
		parent := filepath.Dir(m.Path)
		if parent == m.Path {
			return
		}
		m.Path = parent
		m.reload()
		return
	}

	// Normal (non-archive) parent navigation.
	parent := filepath.Dir(m.Path)
	if parent == m.Path {
		return // already at root
	}
	m.Path = parent
	m.reload()
}

func (m *Model) ToggleHidden() {
	m.ShowHidden = !m.ShowHidden
	m.reload()
}

// StartDelete begins a delete operation. Returns a Cmd when confirmRequired is false
// (immediate delete path) so the app can enqueue it; returns nil when going to
// ModeConfirmDelete (the Cmd will come from doDelete instead).
func (m *Model) StartDelete(confirmRequired bool) tea.Cmd {
	sel := m.Selected()
	if sel == nil {
		return nil
	}
	m.ConfirmTarget = *sel
	if confirmRequired {
		m.Mode = ModeConfirmDelete
		return nil
	}
	// Immediate delete — emit the msg so app can enqueue as a background job.
	path := sel.Path
	prov := m.Provider
	m.reload()
	return func() tea.Msg {
		return DeleteConfirmedMsg{Path: path, Provider: prov}
	}
}

func (m *Model) StartRename() {
	sel := m.Selected()
	if sel == nil {
		return
	}
	m.Mode = ModeRename
	m.Input.SetValue(sel.Name)
	m.Input.Focus()
	m.Input.CursorEnd()
}

func (m *Model) StartNewFile() {
	m.Mode = ModeNewFile
	m.Input.SetValue("")
	m.Input.Placeholder = "filename"
	m.Input.Focus()
}

func (m *Model) StartNewDir() {
	m.Mode = ModeNewDir
	m.Input.SetValue("")
	m.Input.Placeholder = "dirname"
	m.Input.Focus()
}

func (m *Model) StartFilter() {
	m.Mode = ModeFilter
	m.FilterStr = ""
}

func (m *Model) StartGotoPath() {
	m.Mode = ModeGotoPath
	m.Input.SetValue(m.Path + "/")
	m.Input.Placeholder = "path"
	m.Input.Focus()
	m.Input.CursorEnd()
}

func (m *Model) GoHome() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	m.Path = home
	m.archiveStack = nil
	m.reload()
}

// ApplyFilterPublic is an exported wrapper around applyFilter for use by app.
func (m *Model) ApplyFilterPublic() {
	m.applyFilter()
}

func (m *Model) Reload() {
	m.reload()
}

// --- Rendering ---

// View renders the pane to a string.
func (m *Model) View() string {
	innerW := m.Width - 2  // subtract border
	innerH := m.Height - 2 // subtract border

	if innerW < 4 || innerH < 3 {
		return ""
	}

	var sb strings.Builder

	// Path header line — accent color when active, dim when inactive.
	pathDisplay := m.Path
	if len(pathDisplay) > innerW {
		pathDisplay = "…" + pathDisplay[len(pathDisplay)-innerW+1:]
	}
	var pathStyle lipgloss.Style
	if m.IsActive {
		pathStyle = m.Theme.PathHeader.
			Copy().
			Foreground(lipgloss.Color("#00ffd0"))
	} else {
		pathStyle = m.Theme.PathHeader
	}
	header := pathStyle.Width(innerW).Render(pathDisplay)
	sb.WriteString(header)
	sb.WriteString("\n")

	// Filter indicator.
	if m.FilterStr != "" {
		filterLine := m.Theme.PaletteInput.Width(innerW).Render("/" + m.FilterStr)
		sb.WriteString(filterLine)
		sb.WriteString("\n")
		innerH--
	}

	// Inline input line.
	var inputPrompt string
	switch m.Mode {
	case ModeRename:
		inputPrompt = "Rename: "
	case ModeNewFile:
		inputPrompt = "New file: "
	case ModeNewDir:
		inputPrompt = "New dir: "
	case ModeGotoPath:
		inputPrompt = "Go to: "
	case ModeConfirmDelete:
		sel := m.ConfirmTarget
		inputPrompt = fmt.Sprintf("Delete %q? (y/n) ", sel.Name)
		inputLine := m.Theme.StatusBar.Width(innerW).Render(inputPrompt)
		sb.WriteString(inputLine)
		sb.WriteString("\n")
		innerH--
		inputPrompt = ""
	}
	if inputPrompt != "" {
		inputLine := inputPrompt + m.Input.View()
		sb.WriteString(lipgloss.NewStyle().Width(innerW).Render(inputLine))
		sb.WriteString("\n")
		innerH--
	}

	// Compute visible window.
	listH := innerH - 1 // -1 for header
	if listH < 1 {
		listH = 1
	}

	startIdx := 0
	if m.Cursor >= listH {
		startIdx = m.Cursor - listH + 1
	}

	for i := startIdx; i < startIdx+listH; i++ {
		if i >= len(m.Filtered) {
			sb.WriteString(strings.Repeat(" ", innerW))
			sb.WriteString("\n")
			continue
		}
		fi := m.Filtered[i]
		row := m.renderEntry(fi, i == m.Cursor, innerW)
		sb.WriteString(row)
		sb.WriteString("\n")
	}

	content := sb.String()
	// Trim trailing newline.
	content = strings.TrimRight(content, "\n")

	var boxStyle lipgloss.Style
	if m.IsActive {
		boxStyle = m.Theme.ActiveBorder.
			Width(innerW).
			Height(innerH)
	} else {
		boxStyle = m.Theme.InactiveBorder.
			Width(innerW).
			Height(innerH)
	}

	return boxStyle.Render(content)
}

func (m *Model) renderEntry(fi fileinfo.FileInfo, selected bool, width int) string {
	icon := fileinfo.Icon(fi)

	name := fi.Name
	if fi.IsSymlink && fi.SymlinkTarget != "" {
		name = fi.Name + " -> " + fi.SymlinkTarget
	}

	maxName := width - 3 // icon + space + name
	if len(name) > maxName {
		name = name[:maxName-1] + "…"
	}

	line := fmt.Sprintf("%s %s", icon, name)
	// Pad to full width.
	if len(line) < width {
		line += strings.Repeat(" ", width-len(line))
	}

	var style lipgloss.Style
	if selected {
		style = m.Theme.Cursor
	} else if fi.IsSymlink {
		style = m.Theme.SymlinkName
	} else if fi.IsDir {
		style = m.Theme.DirName
	} else {
		style = m.Theme.FileName
	}

	return style.Render(line)
}
