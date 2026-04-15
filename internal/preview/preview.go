package preview

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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
	IsImage bool // true for image card output — skip stripANSIBg post-processing
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

	// Search state
	searchOpen    bool
	searchQuery   string
	searchMatches []int // line indices with matches
	matchIdx      int
	rawContent    string // content before search highlighting
	searchInput   textinput.Model

	// contentReadyAt marks when SetContent last completed; used to flash a
	// brief accent divider in View() so content changes are visually obvious.
	contentReadyAt time.Time
}

// New creates a new preview Model.
func New(t *theme.Theme) *Model {
	s := spinner.New()
	s.Spinner = spinner.Meter
	s.Style = lipgloss.NewStyle().Foreground(t.StatusBarAccent.GetForeground()).Bold(true)

	vp := viewport.New(0, 0)

	si := textinput.New()
	si.Placeholder = "search…"
	si.CharLimit = 128

	return &Model{
		Theme:       t,
		spinner:     s,
		vp:          vp,
		searchInput: si,
	}
}

// Init returns the spinner tick command.
func (m *Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// SetFile sets the file to preview and returns a tea.Cmd that loads content asynchronously.
func (m *Model) SetFile(fi *fileinfo.FileInfo) tea.Cmd {
	return m.SetFileWithGit(fi, "")
}

// SetFileWithGit is like SetFile but takes an optional git status glyph.
// When glyph is "M" or "A", the preview attempts to render `git diff` output
// instead of the file body. Falls through to the normal preview if the diff
// is empty or git is unavailable.
func (m *Model) SetFileWithGit(fi *fileinfo.FileInfo, gitGlyph string) tea.Cmd {
	m.file = fi
	m.err = nil
	m.loading = true

	// Capture local copies for the goroutine.
	width := m.Width
	height := m.Height

	isImg := fi != nil && isImageExt(filepath.Ext(fi.Name))
	// Capture the theme for the goroutine — Theme is a value, not a pointer,
	// so this is a snapshot that won't race with later theme switches.
	t := m.Theme
	return tea.Batch(
		func() tea.Msg {
			content, err := renderFile(fi, width, height, t, gitGlyph)
			return ContentReadyMsg{Content: content, Err: err, IsImage: isImg}
		},
		m.spinner.Tick,
	)
}

// SetContent stores the loaded content (called from app on ContentReadyMsg).
// All content — regardless of how it was rendered — is post-processed here to:
//  1. Strip any residual background ANSI codes (catches markdown, plain text, etc.)
//  2. Restore the theme background after every SGR reset so inline resets don't
//     snap individual characters to the terminal's default background color.
func (m *Model) SetContent(msg ContentReadyMsg) {
	m.loading = false
	m.err = msg.Err

	content := msg.Content
	if msg.Err == nil && content != "" && !msg.IsImage {
		// For text content: strip explicit background codes so it inherits
		// the pane background, and convert full SGR resets to fg-only resets
		// so the background is never cleared between tokens.
		content = stripANSIBg(content)
		fg := hexFromColor(m.Theme.FileName.GetForeground())
		bg := hexFromColor(m.Theme.PreviewBorder.GetBackground())
		content = softenResets(content, fg, bg)
	}
	// Image card content uses only fg colors — leave its ANSI sequences untouched.

	m.rawContent = content
	m.vp.SetContent(content)
	m.vp.GotoTop()
	m.contentReadyAt = time.Now()
	if m.searchOpen && m.searchQuery != "" {
		m.rebuildSearch()
	}
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

// OpenSearch opens the inline search bar.
func (m *Model) OpenSearch() {
	m.searchOpen = true
	m.searchQuery = ""
	m.searchInput.SetValue("")
	m.searchInput.Focus()
	m.searchMatches = nil
	m.matchIdx = 0
}

// CloseSearch closes the inline search bar and restores unhighlighted content.
func (m *Model) CloseSearch() {
	m.searchOpen = false
	m.searchInput.Blur()
	m.searchMatches = nil
	m.vp.SetContent(m.rawContent)
}

// SearchOpen reports whether the search bar is open.
func (m *Model) SearchOpen() bool { return m.searchOpen }

// HandleSearchKey routes a key message to the search bar.
// Returns true if the key was consumed.
func (m *Model) HandleSearchKey(msg tea.KeyMsg) bool {
	if !m.searchOpen {
		return false
	}
	switch msg.Type {
	case tea.KeyEsc:
		m.CloseSearch()
		return true
	case tea.KeyEnter:
		m.nextMatch()
		return true
	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		_ = cmd
		newQuery := m.searchInput.Value()
		if newQuery != m.searchQuery {
			m.searchQuery = newQuery
			m.rebuildSearch()
		}
		return true
	}
}

func (m *Model) nextMatch() {
	if len(m.searchMatches) == 0 {
		return
	}
	m.matchIdx = (m.matchIdx + 1) % len(m.searchMatches)
	m.vp.SetYOffset(m.searchMatches[m.matchIdx])
}

func (m *Model) prevMatch() {
	if len(m.searchMatches) == 0 {
		return
	}
	m.matchIdx = (m.matchIdx - 1 + len(m.searchMatches)) % len(m.searchMatches)
	m.vp.SetYOffset(m.searchMatches[m.matchIdx])
}

// NextMatch advances to the next search match.
func (m *Model) NextMatch() { m.nextMatch() }

// PrevMatch advances to the previous search match.
func (m *Model) PrevMatch() { m.prevMatch() }

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[mGKHF]`)

// ansiSGR matches any ANSI SGR escape sequence.
var ansiSGR = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// stripANSIBg removes background-color parameters from ANSI SGR sequences while
// preserving foreground colors, bold, italic, and other attributes. It handles
// combined sequences like \x1b[38;2;r;g;b;48;2;r;g;bm that Chroma emits.
func stripANSIBg(s string) string {
	return ansiSGR.ReplaceAllStringFunc(s, func(seq string) string {
		m := ansiSGR.FindStringSubmatch(seq)
		if m == nil || m[1] == "" {
			return seq // bare reset — keep as-is
		}
		params := strings.Split(m[1], ";")
		kept := make([]string, 0, len(params))
		i := 0
		for i < len(params) {
			n, err := strconv.Atoi(params[i])
			if err != nil {
				kept = append(kept, params[i])
				i++
				continue
			}
			switch {
			case n == 38 || n == 58:
				// Extended foreground or underline color (38;5;n or 38;2;r;g;b).
				// Keep the whole sub-group intact; do NOT process sub-params
				// individually — their numeric values (e.g. 100) would be
				// misidentified as bright-background codes.
				if i+1 < len(params) {
					sub, _ := strconv.Atoi(params[i+1])
					if sub == 5 && i+2 < len(params) {
						kept = append(kept, params[i:i+3]...)
						i += 3
					} else if sub == 2 && i+4 < len(params) {
						kept = append(kept, params[i:i+5]...)
						i += 5
					} else {
						kept = append(kept, params[i])
						i++
					}
				} else {
					kept = append(kept, params[i])
					i++
				}
			case (n >= 40 && n <= 47) || (n >= 100 && n <= 107):
				// Basic or bright background — skip 1 param.
				i++
			case n == 48:
				// Extended background: 48;5;n or 48;2;r;g;b — skip all sub-params.
				if i+1 < len(params) {
					sub, _ := strconv.Atoi(params[i+1])
					if sub == 5 && i+2 < len(params) {
						i += 3 // 48;5;n
					} else if sub == 2 && i+4 < len(params) {
						i += 5 // 48;2;r;g;b
					} else {
						i += 2
					}
				} else {
					i++
				}
			default:
				kept = append(kept, params[i])
				i++
			}
		}
		if len(kept) == 0 {
			return "" // nothing left — drop sequence
		}
		return "\x1b[" + strings.Join(kept, ";") + "m"
	})
}

func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

// softenResets replaces full SGR resets (\x1b[0m and \x1b[m) with a reset
// that snaps fg/bg back to the *theme* colours rather than the terminal
// defaults. Without this, every reset between Chroma tokens would revert to
// the user's terminal foreground — which on a light pelorus theme running in
// a dark terminal makes the text invisibly white on light pane bg.
func softenResets(s, fgHex, bgHex string) string {
	fr, fg, fb, okFg := parseHex(fgHex)
	br, bg, bb, okBg := parseHex(bgHex)
	var reset string
	switch {
	case okFg && okBg:
		reset = fmt.Sprintf("\x1b[0;38;2;%d;%d;%d;48;2;%d;%d;%dm", fr, fg, fb, br, bg, bb)
	case okFg:
		reset = fmt.Sprintf("\x1b[0;38;2;%d;%d;%dm", fr, fg, fb)
	case okBg:
		reset = fmt.Sprintf("\x1b[0;48;2;%d;%d;%dm", br, bg, bb)
	default:
		reset = "\x1b[39m"
	}
	s = strings.ReplaceAll(s, "\x1b[0m", reset)
	s = strings.ReplaceAll(s, "\x1b[m", reset)
	return s
}

// hexFromColor pulls a #rrggbb string out of a lipgloss color, returning ""
// if the color is not a direct hex value (e.g. an ANSI number).
func hexFromColor(c lipgloss.TerminalColor) string {
	if lc, ok := c.(lipgloss.Color); ok {
		s := string(lc)
		if strings.HasPrefix(s, "#") && len(s) == 7 {
			return s
		}
	}
	return ""
}

// parseHex parses #rrggbb into r/g/b in [0,255]. Returns ok=false otherwise.
func parseHex(s string) (int, int, int, bool) {
	if !strings.HasPrefix(s, "#") || len(s) != 7 {
		return 0, 0, 0, false
	}
	var rgb [3]int
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseInt(s[1+i*2:3+i*2], 16, 0)
		if err != nil {
			return 0, 0, 0, false
		}
		rgb[i] = int(v)
	}
	return rgb[0], rgb[1], rgb[2], true
}

func (m *Model) rebuildSearch() {
	m.searchMatches = nil
	m.matchIdx = 0
	if m.searchQuery == "" {
		m.vp.SetContent(m.rawContent)
		return
	}
	query := strings.ToLower(m.searchQuery)
	lines := strings.Split(m.rawContent, "\n")
	highlightStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#f9e2af")).
		Foreground(lipgloss.Color("#1e1e2e"))
	var out []string
	for i, line := range lines {
		plain := strings.ToLower(stripANSI(line))
		if strings.Contains(plain, query) {
			m.searchMatches = append(m.searchMatches, i)
			out = append(out, highlightStyle.Render(stripANSI(line)))
		} else {
			out = append(out, line)
		}
	}
	m.vp.SetContent(strings.Join(out, "\n"))
	if len(m.searchMatches) > 0 {
		m.vp.SetYOffset(m.searchMatches[0])
	}
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
	border := m.Theme.PreviewBorder

	innerW := m.Width - 2   // subtract border
	innerH := m.Height - 2  // subtract border
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	// Header line with filename. Lipgloss wraps at Width(innerW) — measure the
	// actual rendered height so the viewport below shrinks as the header grows,
	// instead of overflowing the bottom border.
	headerText := " Preview"
	if m.file != nil {
		headerText = " " + m.file.Name
	}
	header := m.Theme.StatusBarAccent.Copy().Bold(true).Width(innerW).Render(headerText)
	headerH := lipgloss.Height(header)

	// Separator. Flashes to a thicker accent line for ~800ms after SetContent
	// so users notice the preview just updated. The mascot tick at 550ms
	// guarantees a re-render that drops back to the normal divider.
	var separator string
	if !m.contentReadyAt.IsZero() && time.Since(m.contentReadyAt) < 800*time.Millisecond {
		accent := m.Theme.StatusBarAccent.Copy().Bold(true)
		separator = accent.Render(strings.Repeat("━", innerW))
	} else {
		separator = m.Theme.Divider.Render(strings.Repeat("─", innerW))
	}

	// Viewport area: innerH minus actual header height, minus separator (1 line).
	vpH := innerH - headerH - 1
	if m.searchOpen {
		vpH--
	}
	if vpH < 1 {
		vpH = 1
	}

	// Sync viewport dimensions.
	m.vp.Width = innerW
	m.vp.Height = vpH

	// Pane background taken from the preview border style so every body line
	// renders on the theme's pane colour — otherwise chroma/glamour output
	// leaves stray terminal-default gaps between tokens.
	paneBg := m.Theme.PreviewBorder.GetBackground()
	bgStyle := lipgloss.NewStyle().Background(paneBg)

	var body string
	if m.file == nil {
		body = bgStyle.Copy().
			Width(innerW).Height(vpH).
			Render("No file selected")
	} else if m.loading {
		name := ""
		if m.file != nil {
			name = m.file.Name
		}
		accent := m.Theme.StatusBarAccent.GetForeground()
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")).Background(paneBg)
		nameStyle := lipgloss.NewStyle().Foreground(accent).Background(paneBg).Bold(true)
		loadLine := m.spinner.View() + "  " + nameStyle.Render(name)
		hint := dim.Render("reading…")
		body = bgStyle.Copy().
			Width(innerW).Height(vpH).
			Align(lipgloss.Center, lipgloss.Center).
			Render(loadLine + "\n\n" + hint)
	} else if m.err != nil {
		body = bgStyle.Copy().
			Width(innerW).Height(vpH).
			Render("Error: " + m.err.Error())
	} else {
		body = bgStyle.Copy().
			Width(innerW).Height(vpH).
			Render(m.vp.View())
	}

	var inner string
	if m.searchOpen {
		matchInfo := ""
		if len(m.searchMatches) > 0 {
			matchInfo = fmt.Sprintf(" [%d/%d]", m.matchIdx+1, len(m.searchMatches))
		} else if m.searchQuery != "" {
			matchInfo = " [no matches]"
		}
		searchBar := m.Theme.StatusBar.Copy().Width(innerW).
			Render("/ " + m.searchInput.View() + matchInfo)
		inner = lipgloss.JoinVertical(lipgloss.Left, header, separator, body, searchBar)
	} else {
		inner = lipgloss.JoinVertical(lipgloss.Left, header, separator, body)
	}

	return border.
		Width(innerW).
		Height(innerH).
		Render(inner)
}

// renderFile does the heavy lifting: reads the file and produces a rendered string.
func renderFile(fi *fileinfo.FileInfo, width, height int, t *theme.Theme, gitGlyph string) (string, error) {
	if fi == nil {
		return "", nil
	}

	// Match what View() computes for the viewport so renderers wrap to the
	// exact width the viewport will display. Off-by-2 here leaves a bg gap
	// on the right and causes glamour to over-wrap URLs.
	innerW := width - 2
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

	// Symlinks: show a target-focused card (including broken-link indicator).
	if fi.IsSymlink {
		return renderSymlinkCard(fi, innerW, t), nil
	}

	// Git-modified files: try to render diff output first.
	if gitGlyph == "M" || gitGlyph == "A" {
		if diff, ok := renderGitDiff(fi.Path, innerW); ok {
			return diff, nil
		}
	}

	// Image files: render metadata card with artist tile.
	if isImageExt(filepath.Ext(fi.Name)) {
		return renderImageCard(fi, fi.Path, innerW, t)
	}

	// Try to read the file.
	f, err := os.Open(fi.Path)
	if err != nil {
		return renderInfoCard(fi, innerW, t), nil
	}
	defer f.Close()

	limited := io.LimitReader(f, maxReadBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return renderInfoCard(fi, innerW, t), nil
	}

	truncated := false
	if len(raw) > maxReadBytes {
		raw = raw[:maxReadBytes]
		truncated = true
	}

	// Check UTF-8.
	if !utf8.Valid(raw) {
		return renderInfoCard(fi, innerW, t), nil
	}

	content := string(raw)

	var rendered string

	// Markdown.
	ext := strings.ToLower(filepath.Ext(fi.Name))
	if ext == ".md" || ext == ".markdown" {
		glamourStyle := ""
		if t != nil {
			glamourStyle = t.GlamourStyle
		}
		rendered, err = renderMarkdown(content, innerW, glamourStyle)
		if err != nil {
			rendered = content // fallback to raw
		}
	} else {
		chromaStyle := ""
		if t != nil {
			chromaStyle = t.ChromaStyle
		}
		// Try Chroma syntax highlight.
		highlighted, chromaErr := renderChroma(fi.Name, content, innerW, chromaStyle)
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

func renderMarkdown(content string, width int, styleName string) (string, error) {
	if styleName == "" {
		styleName = "dark"
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath(styleName),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", err
	}
	return renderer.Render(content)
}

func renderChroma(filename, content string, _ int, styleName string) (string, error) {
	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Analyse(content)
	}
	if lexer == nil {
		return "", fmt.Errorf("no lexer found")
	}
	lexer = chroma.Coalesce(lexer)

	if styleName == "" {
		styleName = "monokai"
	}
	style := styles.Get(styleName)
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


func isImageExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg", ".tiff", ".tif", ".ico", ".heic", ".avif":
		return true
	}
	return false
}
