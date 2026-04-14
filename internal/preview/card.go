package preview

import (
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
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/mogglemoss/pelorus/internal/theme"
	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

// fileClass categorises a file for card rendering: chooses icon, descriptor,
// and artist-tile glyph palette.
type fileClass int

const (
	classImage fileClass = iota
	classBinary
	classArchive
	classDocument
	classCode
	classConfig
	classVideo
	classAudio
	classShell
	classSymlink
	classUnknown
)

// classifyFile returns the class for a FileInfo based on extension and metadata.
func classifyFile(fi *fileinfo.FileInfo) fileClass {
	if fi.IsSymlink {
		return classSymlink
	}
	ext := strings.ToLower(filepath.Ext(fi.Name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg", ".tiff", ".tif", ".ico", ".heic", ".avif":
		return classImage
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar", ".tgz":
		return classArchive
	case ".pdf", ".doc", ".docx", ".odt", ".rtf", ".epub":
		return classDocument
	case ".go", ".rs", ".py", ".js", ".ts", ".jsx", ".tsx", ".c", ".cpp", ".h", ".java", ".rb", ".swift", ".kt":
		return classCode
	case ".json", ".yaml", ".yml", ".toml", ".xml", ".ini", ".env":
		return classConfig
	case ".mp4", ".mov", ".mkv", ".avi", ".webm", ".flv":
		return classVideo
	case ".mp3", ".flac", ".wav", ".ogg", ".m4a", ".aac":
		return classAudio
	case ".sh", ".fish", ".zsh", ".bash":
		return classShell
	}
	if fi.Mode&0111 != 0 && !fi.IsDir {
		return classBinary
	}
	return classUnknown
}

// classDescriptor returns a short human label for a class, e.g. "image · jpeg".
func classDescriptor(fi *fileinfo.FileInfo, cls fileClass) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fi.Name), "."))
	if ext == "" {
		ext = "—"
	}
	switch cls {
	case classImage:
		return "image · " + ext
	case classArchive:
		return "archive · " + ext
	case classDocument:
		return "document · " + ext
	case classCode:
		return "source · " + ext
	case classConfig:
		return "config · " + ext
	case classVideo:
		return "video · " + ext
	case classAudio:
		return "audio · " + ext
	case classShell:
		return "script · " + ext
	case classBinary:
		return "binary · executable"
	case classSymlink:
		if fi.SymlinkBroken {
			return "symlink · broken"
		}
		return "symlink"
	default:
		return "file · " + ext
	}
}

// tilePalette returns the block-character palette for an artist tile based on class.
// Different palettes give each file class a distinct visual texture.
func tilePalette(cls fileClass) []rune {
	switch cls {
	case classImage:
		return []rune("▀▄█▌▐▙▛▜▟░▒▓")
	case classCode, classConfig:
		// Braille — denser, more code-like
		return []rune("⠁⠂⠃⠄⠅⠆⠇⠈⠉⠊⠋⠌⠍⠎⠏⠐⠑⠒⠓⠔⠕⠖⠗⠘⠙⠚⠛⠜⠝⠞⠟")
	case classArchive:
		// Shade ramp — compressed / packed feel
		return []rune("░▒▓█▓▒░ ")
	case classBinary:
		return []rune("▚▞▟▙▛▜▘▝▖▗")
	case classShell:
		return []rune("❯»›∴∷⊕⊗")
	case classDocument:
		return []rune("≡≣⋮⋯⫶⦂")
	case classVideo:
		return []rune("▶▷▸▹◉●○")
	case classAudio:
		return []rune("♪♫♩♬∿~⌇")
	case classSymlink:
		return []rune("↪→⇾⇢⟶↦")
	default:
		return []rune("·•◦∙○◯")
	}
}

// buildArtistTile generates a deterministic block-character tile for a file.
// The pattern is derived from filename+size hash; the color comes from the caller.
// The glyph palette varies by file class for visual distinction.
func buildArtistTile(name string, size int64, cls fileClass, col lipgloss.Color, width int) string {
	h := fnv.New64a()
	h.Write([]byte(name))
	h.Write([]byte(strconv.FormatInt(size, 10)))
	v := h.Sum64()

	const (
		lcgA  uint64 = 6364136223846793005
		lcgC  uint64 = 1442695040888963407
		tileH        = 5
	)
	glyphs := tilePalette(cls)

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
			line.WriteRune(glyphs[v>>58%uint64(len(glyphs))])
		}
		if row > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(style.Render(line.String()))
	}
	return sb.String()
}

// buildCard assembles a standard fallback card: artist tile on top, rows below.
// rows is a pre-formatted []string of label+value lines.
func buildCard(tile string, rows []string) string {
	return tile + "\n\n" + strings.Join(rows, "\n")
}

// --- Row builders --------------------------------------------------------

var valStyle = lipgloss.NewStyle().Bold(true)

// row formats a label + value pair with consistent padding.
func row(t *theme.Theme, label, value string) string {
	const labelW = 10
	// Pad label to fixed width so values align.
	if len(label) < labelW {
		label = label + strings.Repeat(" ", labelW-len(label))
	}
	return t.CardLabel.Render(label) + valStyle.Render(value)
}

// permsRow renders unix permission bits with colored rwx glyphs.
//
//	rwx r-x r-- — semantic colours from the active theme.
func permsRow(t *theme.Theme, mode os.FileMode) string {
	perms := mode.Perm()
	bits := [9]rune{
		'r', 'w', 'x', // owner
		'r', 'w', 'x', // group
		'r', 'w', 'x', // other
	}
	mask := []os.FileMode{0400, 0200, 0100, 0040, 0020, 0010, 0004, 0002, 0001}

	var sb strings.Builder
	for i := 0; i < 9; i++ {
		ch := string(bits[i])
		if perms&mask[i] != 0 {
			switch i % 3 {
			case 0:
				sb.WriteString(t.CardInfo.Render(ch))
			case 1:
				sb.WriteString(t.CardWarning.Render(ch))
			case 2:
				sb.WriteString(t.CardSuccess.Render(ch))
			}
		} else {
			sb.WriteString(t.CardDim.Render("-"))
		}
		if i == 2 || i == 5 {
			sb.WriteRune(' ')
		}
	}
	return t.CardLabel.Render("perms     ") + sb.String()
}

// sizeBarRow renders a log-scaled horizontal bar showing file size magnitude.
// Scale: 1 B → 1 GB mapped across barW chars.
func sizeBarRow(t *theme.Theme, size int64, barW int) string {
	if barW < 4 {
		barW = 4
	}
	// Log10 scale: 0 B → 0, 1 GB (1e9) → full.
	const maxLog = 9.0
	l := 0.0
	if size > 0 {
		l = math.Log10(float64(size))
	}
	frac := l / maxLog
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac*float64(barW) + 0.5)
	if filled > barW {
		filled = barW
	}

	// Colour the bar based on size magnitude: success (small) → warning → danger (large).
	var on lipgloss.Style
	switch {
	case frac < 0.33:
		on = t.CardSuccess
	case frac < 0.66:
		on = t.CardWarning
	default:
		on = t.CardDanger
	}
	bar := on.Render(strings.Repeat("━", filled)) + t.CardDim.Render(strings.Repeat("━", barW-filled))
	return t.CardLabel.Render("size      ") + valStyle.Render(fileinfo.HumanSize(size)) + "  " + bar
}

// ageRow renders mod time with a recency glyph.
func ageRow(t *theme.Theme, mod time.Time) string {
	now := time.Now()
	age := now.Sub(mod)
	var glyph string
	var style lipgloss.Style
	switch {
	case age < 24*time.Hour:
		glyph, style = "◉", t.CardSuccess
	case age < 7*24*time.Hour:
		glyph, style = "◎", t.CardWarning
	case age < 90*24*time.Hour:
		glyph, style = "○", t.CardInfo
	default:
		glyph, style = "◌", t.CardDim
	}
	return t.CardLabel.Render("modified  ") + valStyle.Render(mod.Format("2006-01-02 15:04")) + "  " + style.Render(glyph)
}

// --- Top-level card renderers -------------------------------------------

// renderImageCard renders a rich metadata card for image files, using the
// sampled pixel colour (or hash fallback) for both the tile and accent.
func renderImageCard(fi *fileinfo.FileInfo, path string, width int, t *theme.Theme) (string, error) {
	dimStr := "unknown"
	if f, err := os.Open(path); err == nil {
		if cfg, _, err2 := image.DecodeConfig(f); err2 == nil {
			dimStr = fmt.Sprintf("%d × %d", cfg.Width, cfg.Height)
		}
		f.Close()
	}

	col := sampleOrHashColor(fi, path)
	tile := buildArtistTile(fi.Name, fi.Size, classImage, col, width)

	rows := []string{
		row(t, "type", classDescriptor(fi, classImage)),
		row(t, "dims", dimStr),
		sizeBarRow(t, fi.Size, 16),
		ageRow(t, fi.ModTime),
		permsRow(t, fi.Mode),
	}
	return buildCard(tile, rows), nil
}

// renderInfoCard renders a metadata card for any non-image file that cannot
// (or should not) be previewed directly: binaries, unreadable files, non-UTF8
// content, or explicit fallback cases.
// InfoCardFor is the exported wrapper used by overlays (e.g. the app-level
// quick-info modal) to render the same card the preview pane uses for
// unpreviewable files.
func InfoCardFor(fi *fileinfo.FileInfo, width int, t *theme.Theme) string {
	return renderInfoCard(fi, width, t)
}

func renderInfoCard(fi *fileinfo.FileInfo, width int, t *theme.Theme) string {
	cls := classifyFile(fi)
	col := hashColor(fi.Name, fi.Size)
	tile := buildArtistTile(fi.Name, fi.Size, cls, col, width)

	rows := []string{
		row(t, "type", classDescriptor(fi, cls)),
		sizeBarRow(t, fi.Size, 16),
		ageRow(t, fi.ModTime),
		permsRow(t, fi.Mode),
	}
	return buildCard(tile, rows)
}

// renderSymlinkCard renders a card specifically for symlinks, showing the target.
func renderSymlinkCard(fi *fileinfo.FileInfo, width int, t *theme.Theme) string {
	cls := classSymlink
	var col lipgloss.Color
	if fi.SymlinkBroken {
		// Danger style fg for tile when broken.
		if c, ok := t.CardDanger.GetForeground().(lipgloss.Color); ok {
			col = c
		} else {
			col = hashColor(fi.Name, fi.Size)
		}
	} else {
		col = hashColor(fi.Name, fi.Size)
	}
	tile := buildArtistTile(fi.Name, fi.Size, cls, col, width)

	target := fi.SymlinkTarget
	if target == "" {
		target = "(unknown target)"
	}
	arrow := lipgloss.NewStyle().Foreground(col).Render(" → ")
	targetLine := t.CardLabel.Render("target    ") + valStyle.Render(fi.Name) + arrow + valStyle.Render(target)

	var statusStyled string
	if fi.SymlinkBroken {
		statusStyled = t.CardDanger.Bold(true).Render("broken")
	} else {
		statusStyled = t.CardSuccess.Bold(true).Render("ok")
	}

	rows := []string{
		row(t, "type", classDescriptor(fi, cls)),
		targetLine,
		t.CardLabel.Render("status    ") + statusStyled,
		ageRow(t, fi.ModTime),
		permsRow(t, fi.Mode),
	}
	return buildCard(tile, rows)
}

// --- Color helpers -------------------------------------------------------

// sampleOrHashColor tries to sample a representative pixel from the image for
// its color. Falls back to a deterministic color from the filename hash when
// the image is too large to decode safely or decoding fails.
func sampleOrHashColor(fi *fileinfo.FileInfo, path string) lipgloss.Color {
	f, err := os.Open(path)
	if err != nil {
		return hashColor(fi.Name, fi.Size)
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width > 1000 || cfg.Height > 1000 {
		return hashColor(fi.Name, fi.Size)
	}

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
	if bestSat > 20 {
		return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", bestR, bestG, bestB))
	}
	return hashColor(fi.Name, fi.Size)
}

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

func hashColor(name string, size int64) lipgloss.Color {
	h := fnv.New32a()
	h.Write([]byte(name))
	h.Write([]byte(strconv.FormatInt(size, 10)))
	v := h.Sum32()
	r, g, b := hslToRGB(float64(v%360), 0.70, 0.62)
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
}

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
