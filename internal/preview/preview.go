package preview

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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
}

// New creates a new preview Model.
func New(t *theme.Theme) *Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(t.StatusBarAccent.GetForeground())

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
	m.file = fi
	m.err = nil
	m.loading = true

	// Capture local copies for the goroutine.
	width := m.Width
	height := m.Height

	isImg := fi != nil && isImageExt(filepath.Ext(fi.Name))
	return tea.Batch(
		func() tea.Msg {
			content, err := renderFile(fi, width, height)
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
		content = softenResets(content)
	}
	// Image card content uses only fg colors — leave its ANSI sequences untouched.

	m.rawContent = content
	m.vp.SetContent(content)
	m.vp.GotoTop()
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

// softenResets replaces full SGR resets (\x1b[0m and \x1b[m) with a
// foreground-only reset (\x1b[39m). This preserves the current background
// color, which is set by the enclosing lipgloss container (the preview pane
// border). Without this, every \x1b[0m between Chroma tokens would snap the
// background to the terminal default (black), and the viewport's internal
// lipgloss padding would compound the problem by adding its own resets that
// we can't post-process.
func softenResets(s string) string {
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[39m")
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[39m")
	return s
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

	// Header line with filename.
	headerText := " Preview"
	if m.file != nil {
		headerText = " " + m.file.Name
	}
	header := m.Theme.StatusBarAccent.Copy().Bold(true).Width(innerW).Render(headerText)

	// Separator.
	sep := strings.Repeat("─", innerW)
	separator := m.Theme.Divider.Render(sep)

	// Viewport area (innerH - 2 for header + separator).
	vpH := innerH - 2
	if m.searchOpen {
		vpH--
	}
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

	// Image files: render metadata card with artist tile.
	if isImageExt(filepath.Ext(fi.Name)) {
		return renderImageCard(fi, fi.Path, innerW)
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

	// Strip background-color parameters so the viewport background is uniform.
	return stripANSIBg(buf.String()), nil
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

// renderImageCard renders a rich metadata card for image files.
// It shows a procedural artist tile (unique per file, colored from pixel data or hash)
// followed by a metadata table with format, size, dimensions, and modification time.
func renderImageCard(fi *fileinfo.FileInfo, path string, width int) (string, error) {
	// Try to decode image dimensions from header only.
	dimStr := "unknown"
	if f, err := os.Open(path); err == nil {
		if cfg, _, err2 := image.DecodeConfig(f); err2 == nil {
			dimStr = fmt.Sprintf("%d × %d", cfg.Width, cfg.Height)
		}
		f.Close()
	}

	col := sampleOrHashColor(fi, path)

	ext := strings.ToUpper(strings.TrimPrefix(strings.ToLower(filepath.Ext(fi.Name)), "."))
	if ext == "" {
		ext = "IMAGE"
	}

	lbl := lipgloss.NewStyle().Foreground(lipgloss.Color("#777777"))
	val := lipgloss.NewStyle().Bold(true)
	rows := []string{
		lbl.Render("format  ") + val.Render(ext),
		lbl.Render("size    ") + val.Render(fileinfo.HumanSize(fi.Size)),
		lbl.Render("dims    ") + val.Render(dimStr),
		lbl.Render("modified") + val.Render(fi.ModTime.Format("2006-01-02 15:04")),
	}

	tile := buildArtistTile(fi.Name, fi.Size, col, width)
	return tile + "\n\n" + strings.Join(rows, "\n"), nil
}

// sampleOrHashColor tries to sample a representative pixel from the image for
// its color. Falls back to a deterministic color derived from the filename hash
// when the image is too large to decode safely or decoding fails.
func sampleOrHashColor(fi *fileinfo.FileInfo, path string) lipgloss.Color {
	f, err := os.Open(path)
	if err != nil {
		return hashColor(fi.Name, fi.Size)
	}
	defer f.Close()

	// Check dimensions first to avoid allocating huge pixel buffers.
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width > 1000 || cfg.Height > 1000 {
		return hashColor(fi.Name, fi.Size)
	}

	// Re-open for full decode (DecodeConfig consumed the reader).
	f.Close()
	f2, err := os.Open(path)
	if err != nil {
		return hashColor(fi.Name, fi.Size)
	}
	defer f2.Close()

	img, _, err := image.Decode(io.LimitReader(f2, maxReadBytes))
	if err != nil {
		return hashColor(fi.Name, fi.Size)
	}

	bounds := img.Bounds()
	dx, dy := bounds.Dx(), bounds.Dy()
	if dx == 0 || dy == 0 {
		return hashColor(fi.Name, fi.Size)
	}

	// Sample from five positions distributed across the image.
	pts := [][2]int{
		{bounds.Min.X + dx/4, bounds.Min.Y + dy/4},
		{bounds.Min.X + dx/2, bounds.Min.Y + dy/2},
		{bounds.Min.X + 3*dx/4, bounds.Min.Y + 3*dy/4},
		{bounds.Min.X + dx/4, bounds.Min.Y + 3*dy/4},
		{bounds.Min.X + 3*dx/4, bounds.Min.Y + dy/4},
	}
	var bestSat uint32
	var bestR, bestG, bestB uint8
	for _, pt := range pts {
		if pt[0] >= bounds.Max.X || pt[1] >= bounds.Max.Y {
			continue
		}
		r, g, b, _ := img.At(pt[0], pt[1]).RGBA()
		r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)
		if sat := colorSaturation(r8, g8, b8); sat > bestSat {
			bestSat = sat
			bestR, bestG, bestB = r8, g8, b8
		}
	}
	if bestSat > 20 { // ignore near-grey pixels
		return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", bestR, bestG, bestB))
	}
	return hashColor(fi.Name, fi.Size)
}

// colorSaturation returns the chroma (max-min of RGB channels) as a saturation proxy.
func colorSaturation(r, g, b uint8) uint32 {
	max := r
	if g > max {
		max = g
	}
	if b > max {
		max = b
	}
	min := r
	if g < min {
		min = g
	}
	if b < min {
		min = b
	}
	return uint32(max) - uint32(min)
}

// hashColor derives a deterministic, vivid color from a filename and size.
func hashColor(name string, size int64) lipgloss.Color {
	h := fnv.New32a()
	h.Write([]byte(name))
	h.Write([]byte(strconv.FormatInt(size, 10)))
	v := h.Sum32()
	r, g, b := hslToRGB(float64(v%360), 0.70, 0.62)
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
}

// hslToRGB converts HSL values to 8-bit RGB components.
func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60.0, 2.0)-1.0))
	m := l - c/2.0
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return uint8((r+m)*255 + 0.5), uint8((g+m)*255 + 0.5), uint8((b+m)*255 + 0.5)
}

// buildArtistTile generates a deterministic block-character tile for an image file.
// The pattern is derived from the filename+size hash; the color comes from pixel
// sampling or falls back to a hash-derived hue.
func buildArtistTile(name string, size int64, col lipgloss.Color, width int) string {
	h := fnv.New64a()
	h.Write([]byte(name))
	h.Write([]byte(strconv.FormatInt(size, 10)))
	v := h.Sum64()

	const (
		lcgA  uint64 = 6364136223846793005
		lcgC  uint64 = 1442695040888963407
		tileH        = 5
	)
	blocks := []rune("▀▄█▌▐▙▛▜▟░▒▓")

	tileW := width - 2
	if tileW < 8 {
		tileW = 8
	}
	if tileW > 40 {
		tileW = 40
	}

	style := lipgloss.NewStyle().Foreground(col)
	var sb strings.Builder
	for row := 0; row < tileH; row++ {
		var line strings.Builder
		for ci := 0; ci < tileW; ci++ {
			v = v*lcgA + lcgC
			line.WriteRune(blocks[v>>58%uint64(len(blocks))])
		}
		if row > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(style.Render(line.String()))
	}
	return sb.String()
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
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg", ".tiff", ".tif", ".ico", ".heic", ".avif":
		return true
	}
	return false
}
