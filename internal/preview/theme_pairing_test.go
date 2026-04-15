package preview

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mogglemoss/pelorus/internal/theme"
	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

// TestThemedPipeline runs the full preview render pipeline for every built-in
// theme against a code file and a markdown file, then verifies invariants that
// keep content readable:
//
//  1. No bare \x1b[39m sequences remain (softenResets must snap to theme fg).
//  2. No explicit background SGR codes remain (stripANSIBg must remove them).
//  3. No reverse-video (\x1b[7m) sequences remain.
//  4. No bg-reset (\x1b[49m) sequences remain.
//
// This guards against the invisible-text-on-light-theme class of bug.
func TestThemedPipeline(t *testing.T) {
	tmp := t.TempDir()

	goModPath := filepath.Join(tmp, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module foo\n\ngo 1.26\n\nrequire (\n\tfoo/bar v1.0.0\n)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	readmePath := filepath.Join(tmp, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Hello\n\nSome **bold** text with `code` inline.\n\n> quote\n"), 0644); err != nil {
		t.Fatal(err)
	}

	themes := []string{"haruspex", "gruvbox", "nord", "light", "dracula", "catppuccin"}
	files := []string{goModPath, readmePath}

	for _, tn := range themes {
		for _, path := range files {
			t.Run(tn+"/"+filepath.Base(path), func(t *testing.T) {
				th := theme.Get(tn)
				m := New(&th)
				m.Width = 80
				m.Height = 30

				st, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				fi := &fileinfo.FileInfo{
					Name: filepath.Base(path),
					Path: path,
					Size: st.Size(),
					Mode: st.Mode(),
				}

				raw, err := renderFile(fi, m.Width, m.Height, &th, "")
				if err != nil {
					t.Fatalf("renderFile: %v", err)
				}

				m.SetContent(ContentReadyMsg{Content: raw})
				content := m.rawContent

				if strings.Contains(content, "\x1b[39m") {
					t.Errorf("bare fg-reset \\x1b[39m present — softenResets didn't snap to theme fg")
				}
				if strings.Contains(content, "\x1b[49m") {
					t.Errorf("bg-reset \\x1b[49m present — stripANSIBg missed it")
				}
				if strings.Contains(content, "\x1b[7m") {
					t.Errorf("reverse-video \\x1b[7m present — would bleed bg")
				}

				// Any bg code that remains must match the theme's pane bg —
				// otherwise it will render as a visible patch of a different
				// colour inside the pane.
				themeBg := hexFromColor(th.PreviewBorder.GetBackground())
				if foreign := foreignBgCodes(content, themeBg); len(foreign) > 0 {
					t.Errorf("bg codes not matching theme bg %s:\n%s",
						themeBg, strings.Join(foreign, "\n"))
				}
			})
		}
	}
}

var sgrAny = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// foreignBgCodes returns SGR sequences containing bg-setting params that do
// NOT match the given theme bg hex. Allowed: 48;2;R;G;B where R/G/B equal
// the theme bg. Anything else (basic 40-47/100-107 bg, 48;5;n palette bg,
// or 48;2 with wrong RGB) is flagged.
func foreignBgCodes(s, themeBg string) []string {
	var tr, tg, tb int
	var okTheme bool
	if r, g, b, ok := parseHex(themeBg); ok {
		tr, tg, tb, okTheme = r, g, b, true
	}

	var found []string
	for _, m := range sgrAny.FindAllStringSubmatch(s, -1) {
		params := strings.Split(m[1], ";")
		i := 0
		for i < len(params) {
			n, ok := atoi(params[i])
			if !ok {
				i++
				continue
			}
			switch {
			case n == 38 || n == 58:
				// Extended fg / underline colour — skip whole sub-group so
				// rgb values don't get misread as bg codes.
				if i+1 < len(params) {
					sub, _ := atoi(params[i+1])
					if sub == 5 && i+2 < len(params) {
						i += 3
					} else if sub == 2 && i+4 < len(params) {
						i += 5
					} else {
						i++
					}
				} else {
					i++
				}
			case (n >= 40 && n <= 47) || (n >= 100 && n <= 107):
				found = append(found, fmt.Sprintf("ESC[%sm (basic bg)", m[1]))
				i = len(params)
			case n == 48:
				if i+1 >= len(params) {
					i = len(params)
					break
				}
				sub := params[i+1]
				switch sub {
				case "5":
					found = append(found, fmt.Sprintf("ESC[%sm (256-colour bg)", m[1]))
					i = len(params)
				case "2":
					if i+4 >= len(params) {
						i = len(params)
						break
					}
					r, rOK := atoi(params[i+2])
					g, gOK := atoi(params[i+3])
					b, bOK := atoi(params[i+4])
					if rOK && gOK && bOK && okTheme && r == tr && g == tg && b == tb {
						// Theme bg — allowed.
					} else {
						found = append(found, fmt.Sprintf("ESC[%sm (bg %d,%d,%d)", m[1], r, g, b))
					}
					i += 5
				default:
					i += 2
				}
			default:
				i++
			}
		}
	}
	return found
}

func atoi(s string) (int, bool) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

func firstFewCodes(s string) string {
	seen := 0
	var b strings.Builder
	for _, m := range sgrAny.FindAllString(s, -1) {
		if seen > 5 {
			break
		}
		b.WriteString(strings.ReplaceAll(m, "\x1b", "ESC"))
		b.WriteString("\n")
		seen++
	}
	return b.String()
}
