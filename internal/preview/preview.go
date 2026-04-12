package preview

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mogglemoss/pelorus/internal/theme"
	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

const maxReadBytes = 64 * 1024 // 64 KB

// ContentReadyMsg is sent when async file loading completes.
type ContentReadyMsg struct {
	Content string
	Err     error
}

// Model is the Bubbletea model for the preview pane.
type Model struct {
	Width  int
	Height int
	Theme  *theme.Theme

	file    *fileinfo.FileInfo
	loading bool
	err     error

	vp      viewport.Model
	spinner spinner.Model
}

// New creates a new preview Model.
func New(t *theme.Theme) *Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffd0"))

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().Background(lipgloss.Color("#0d1520"))

	return &Model{
		Theme:   t,
		spinner: s,
		vp:      vp,
	}
}

// Init returns the spinner tick command.
func (m *Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// SetFile sets the file to preview and returns a tea.Cmd that loads content asynchronously.
func (m *Model) SetFile(fi *fileinfo.FileInfo) tea.Cmd {
	m.file = fi
	m.err = nil
	m.loading = true

	// Capture local copies for the goroutine.
	width := m.Width
	height := m.Height

	return tea.Batch(
		func() tea.Msg {
			content, err := renderFile(fi, width, height)
			return ContentReadyMsg{Content: content, Err: err}
		},
		m.spinner.Tick,
	)
}

// SetContent stores the loaded content (called from app on ContentReadyMsg).
func (m *Model) SetContent(msg ContentReadyMsg) {
	m.loading = false
	m.err = msg.Err
	m.vp.SetContent(msg.Content)
	m.vp.GotoTop()
}

// IsLoading reports whether content is currently being loaded.
func (m *Model) IsLoading() bool {
	return m.loading
}

// UpdateSpinner advances the spinner animation. Returns a tick command.
func (m *Model) UpdateSpinner(msg spinner.TickMsg) tea.Cmd {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return cmd
}

// SetViewportSize updates the viewport dimensions directly (called from layoutPanes).
func (m *Model) SetViewportSize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	m.vp.Width = w
	m.vp.Height = h
}

// ScrollDown scrolls the preview viewport down by n lines.
func (m *Model) ScrollDown(n int) {
	m.vp.LineDown(n)
}

// ScrollUp scrolls the preview viewport up by n lines.
func (m *Model) ScrollUp(n int) {
	m.vp.LineUp(n)
}

// HalfPageDown scrolls the preview viewport down by half a page.
func (m *Model) HalfPageDown() {
	m.vp.HalfViewDown()
}

// HalfPageUp scrolls the preview viewport up by half a page.
func (m *Model) HalfPageUp() {
	m.vp.HalfViewUp()
}

// View renders the preview pane.
func (m *Model) View() string {
	border := m.Theme.InactiveBorder

	innerW := m.Width - 2   // subtract border
	innerH := m.Height - 2  // subtract border
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	// Header line with filename.
	headerText := " Preview"
	if m.file != nil {
		headerText = " " + m.file.Name
	}
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00a896")).
		Bold(true).
		Width(innerW)
	header := headerStyle.Render(headerText)

	// Separator.
	sep := strings.Repeat("─", innerW)
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1a3040"))
	separator := sepStyle.Render(sep)

	// Viewport area (innerH - 2 for header + separator).
	vpH := innerH - 2
	if vpH < 1 {
		vpH = 1
	}

	// Sync viewport dimensions.
	m.vp.Width = innerW
	m.vp.Height = vpH

	var body string
	if m.file == nil {
		body = lipgloss.NewStyle().
			Width(innerW).Height(vpH).
			Render("No file selected")
	} else if m.loading {
		body = lipgloss.NewStyle().
			Width(innerW).Height(vpH).
			Render(m.spinner.View() + " Loading…")
	} else if m.err != nil {
		body = lipgloss.NewStyle().
			Width(innerW).Height(vpH).
			Render("Error: " + m.err.Error())
	} else {
		body = m.vp.View()
	}

	inner := lipgloss.JoinVertical(lipgloss.Left, header, separator, body)

	return border.
		Width(innerW).
		Height(innerH).
		Render(inner)
}

// renderFile does the heavy lifting: reads the file and produces a rendered string.
func renderFile(fi *fileinfo.FileInfo, width, height int) (string, error) {
	if fi == nil {
		return "", nil
	}

	innerW := width - 4
	if innerW < 10 {
		innerW = 10
	}
	contentLines := height - 4 // header + separator + border
	if contentLines < 1 {
		contentLines = 1
	}

	// Directory: list entries.
	if fi.IsDir {
		return renderDir(fi, innerW, contentLines)
	}

	// Image files: stub.
	if isImageExt(filepath.Ext(fi.Name)) {
		return renderImageStub(fi)
	}

	// Try to read the file.
	f, err := os.Open(fi.Path)
	if err != nil {
		return renderFileInfo(fi), nil
	}
	defer f.Close()

	limited := io.LimitReader(f, maxReadBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return renderFileInfo(fi), nil
	}

	truncated := false
	if len(raw) > maxReadBytes {
		raw = raw[:maxReadBytes]
		truncated = true
	}

	// Check UTF-8.
	if !utf8.Valid(raw) {
		return renderFileInfo(fi), nil
	}

	content := string(raw)

	var rendered string

	// Markdown.
	ext := strings.ToLower(filepath.Ext(fi.Name))
	if ext == ".md" || ext == ".markdown" {
		rendered, err = renderMarkdown(content, innerW)
		if err != nil {
			rendered = content // fallback to raw
		}
	} else {
		// Try Chroma syntax highlight.
		highlighted, chromaErr := renderChroma(fi.Name, content, innerW)
		if chromaErr == nil && highlighted != "" {
			rendered = highlighted
		} else {
			rendered = content
		}
	}

	if truncated {
		rendered += "\n\n[file truncated at 64 KB]"
	}

	return rendered, nil
}

func renderMarkdown(content string, width int) (string, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", err
	}
	return renderer.Render(content)
}

func renderChroma(filename, content string, _ int) (string, error) {
	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Analyse(content)
	}
	if lexer == nil {
		return "", fmt.Errorf("no lexer found")
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal16m")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func renderDir(fi *fileinfo.FileInfo, width, maxLines int) (string, error) {
	entries, err := os.ReadDir(fi.Path)
	if err != nil {
		return fmt.Sprintf("Cannot read directory: %v", err), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d items\n\n", len(entries)))

	shown := 0
	for _, e := range entries {
		if shown >= maxLines-2 {
			remaining := len(entries) - shown
			sb.WriteString(fmt.Sprintf("… and %d more", remaining))
			break
		}
		name := e.Name()
		if e.IsDir() {
			name = name + "/"
		}
		if len(name) > width {
			name = name[:width-1] + "…"
		}
		sb.WriteString(name + "\n")
		shown++
	}

	return sb.String(), nil
}

func renderImageStub(fi *fileinfo.FileInfo) (string, error) {
	return fmt.Sprintf(
		"Image file: %s\nSize: %s\n\n[image preview not available in this terminal]",
		fi.Name,
		fileinfo.HumanSize(fi.Size),
	), nil
}

func renderFileInfo(fi *fileinfo.FileInfo) string {
	return fmt.Sprintf(
		"Name:        %s\nSize:        %s\nPermissions: %s\nModified:    %s\n\n[binary or unreadable file]",
		fi.Name,
		fileinfo.HumanSize(fi.Size),
		fi.Mode.String(),
		fi.ModTime.Format("2006-01-02 15:04:05"),
	)
}

func isImageExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}
