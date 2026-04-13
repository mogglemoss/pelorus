package preview

import (
	"strconv"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// hasBgCode checks whether a string contains any ANSI SGR sequence with a
// background-color parameter. It parses parameter boundaries properly, so
// numeric sub-values inside fg colors (e.g. 100 inside 38;2;100;200;50) are
// not mistaken for bright-background codes (100-107).
func hasBgCode(s string) bool {
	matches := ansiSGR.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if m[1] == "" {
			continue
		}
		params := strings.Split(m[1], ";")
		i := 0
		for i < len(params) {
			n, err := strconv.Atoi(params[i])
			if err != nil {
				i++
				continue
			}
			// Skip extended fg/underline sub-params — their values must not be
			// tested against background code ranges.
			if n == 38 || n == 58 {
				if i+1 < len(params) {
					sub, _ := strconv.Atoi(params[i+1])
					if sub == 5 {
						i += 3
					} else if sub == 2 {
						i += 5
					} else {
						i += 2
					}
				} else {
					i++
				}
				continue
			}
			if (n >= 40 && n <= 47) || (n >= 100 && n <= 107) || n == 48 {
				return true
			}
			i++
		}
	}
	return false
}

func TestStripANSIBg(t *testing.T) {
	esc := "\x1b["

	tests := []struct {
		name string
		input string
		// wantBgGone: no background SGR sequences should remain
		wantBgGone bool
		// wantContains: these substrings must survive in the output
		wantContains []string
		// wantExact: if non-empty, output must equal this exactly
		wantExact string
	}{
		{
			name:       "standalone true-color background stripped",
			input:      esc + "48;2;30;0;16m",
			wantBgGone: true,
			wantExact:  "",
		},
		{
			name:       "standalone 256-color background stripped",
			input:      esc + "48;5;220m",
			wantBgGone: true,
			wantExact:  "",
		},
		{
			name:       "basic background (41 red) stripped",
			input:      esc + "41m",
			wantBgGone: true,
			wantExact:  "",
		},
		{
			name:         "bright background (101) stripped",
			input:        esc + "101mtext" + esc + "0m",
			wantBgGone:   true,
			wantContains: []string{"text", esc + "0m"},
		},
		{
			name:       "foreground-only sequence untouched",
			input:      esc + "38;2;248;248;242m",
			wantBgGone: true,
			wantExact:  esc + "38;2;248;248;242m",
		},
		{
			name:       "bold untouched",
			input:      esc + "1m",
			wantBgGone: true,
			wantExact:  esc + "1m",
		},
		{
			name:       "bare reset untouched",
			input:      esc + "0m",
			wantBgGone: true,
			wantExact:  esc + "0m",
		},
		{
			name:       "empty reset untouched",
			input:      esc + "m",
			wantBgGone: true,
			wantExact:  esc + "m",
		},
		{
			name:       "combined fg+bg: bg stripped, fg kept",
			input:      esc + "38;2;248;248;242;48;2;39;40;34m",
			wantBgGone: true,
			wantExact:  esc + "38;2;248;248;242m",
		},
		{
			name:       "combined bold+bg: bg stripped, bold kept",
			input:      esc + "1;41m",
			wantBgGone: true,
			wantExact:  esc + "1m",
		},
		{
			name:       "combined fg+bold+bg: bg stripped, rest kept",
			// 100 here is the red component of the fg RGB — must not be
			// mistaken for a bright-background code (100-107).
			input:      esc + "1;38;2;100;200;50;48;2;30;0;16m",
			wantBgGone: true,
			wantExact:  esc + "1;38;2;100;200;50m",
		},
		{
			name: "zed-lexer year pattern: fg + standalone-bg + text + reset",
			// Chroma Zed lexer emits this for a year like "2026":
			// \x1b[38;2;150;0;80m  \x1b[48;2;30;0;16m  2026  \x1b[0m
			input:        esc + "38;2;150;0;80m" + esc + "48;2;30;0;16m" + "2026" + esc + "0m",
			wantBgGone:   true,
			wantContains: []string{esc + "38;2;150;0;80m", "2026", esc + "0m"},
		},
		{
			name:       "fg with high rgb values (243) not confused with bg codes",
			input:      esc + "38;2;243;243;243m",
			wantBgGone: true,
			wantExact:  esc + "38;2;243;243;243m",
		},
		{
			name:       "plain text with no ANSI untouched",
			input:      "hello world",
			wantBgGone: true,
			wantExact:  "hello world",
		},
		{
			name:       "empty string",
			input:      "",
			wantBgGone: true,
			wantExact:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripANSIBg(tc.input)

			if tc.wantBgGone && hasBgCode(got) {
				t.Errorf("background ANSI code still present\n  input: %q\n  got:   %q", tc.input, got)
			}

			if tc.wantExact != "" && got != tc.wantExact {
				t.Errorf("output mismatch\n  input: %q\n  want:  %q\n  got:   %q", tc.input, tc.wantExact, got)
			}

			for _, sub := range tc.wantContains {
				if !strings.Contains(got, sub) {
					t.Errorf("expected %q in output\n  input: %q\n  got:   %q", sub, tc.input, got)
				}
			}
		})
	}
}

// TestFixResets verifies that bare SGR resets are followed by a background restore.
func TestFixResets(t *testing.T) {
	bg := "#1a1815"
	restore := "\x1b[48;2;26;24;21m" // #1a1815 decoded

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bare 0m reset gets restore appended",
			input: "text\x1b[0mmore",
			want:  "text\x1b[0m" + restore + "more",
		},
		{
			name:  "bare m reset gets restore appended",
			input: "text\x1b[mmore",
			want:  "text\x1b[m" + restore + "more",
		},
		{
			name:  "multiple resets all get restore",
			input: "\x1b[0mA\x1b[0mB",
			want:  "\x1b[0m" + restore + "A\x1b[0m" + restore + "B",
		},
		{
			name:  "no resets unchanged",
			input: "\x1b[38;2;100;200;50mtext",
			want:  "\x1b[38;2;100;200;50mtext",
		},
		{
			name:  "invalid bg color returns unchanged",
			input: "text\x1b[0mmore",
			// bg will be "" when passed to fixResets — function should return unchanged
		},
		{
			name:  "empty input unchanged",
			input: "",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			useBg := bg
			if tc.name == "invalid bg color returns unchanged" {
				useBg = ""
				tc.want = tc.input // unchanged
			}
			got := fixResets(tc.input, useBg)
			if got != tc.want {
				t.Errorf("fixResets mismatch\n  input: %q\n  want:  %q\n  got:   %q", tc.input, tc.want, got)
			}
		})
	}
}

// TestRenderChromaNoBackground runs the full Chroma pipeline (same as
// renderChroma) on the MIT License text, which the Zed lexer highlights with
// background ANSI codes on years. Verifies stripANSIBg removes all of them.
func TestRenderChromaNoBackground(t *testing.T) {
	// Full MIT License text — enough for lexers.Analyse to detect "Zed".
	content := `MIT License

Copyright (c) 2024 Scott Corbin

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.`

	lexer := lexers.Match("LICENSE")
	if lexer == nil {
		lexer = lexers.Analyse(content)
	}
	if lexer == nil {
		t.Skip("no Chroma lexer for this content — skipping")
	}
	t.Logf("lexer: %s", lexer.Config().Name)

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
		t.Fatalf("tokenise: %v", err)
	}

	var buf strings.Builder
	if err := formatter.Format(&buf, style, iterator); err != nil {
		t.Fatalf("format: %v", err)
	}

	raw := buf.String()

	if !hasBgCode(raw) {
		t.Log("NOTE: raw Chroma output has no background codes — test is vacuous if lexer changes")
	} else {
		t.Log("raw output contains background codes — stripping")
	}

	stripped := stripANSIBg(raw)

	if hasBgCode(stripped) {
		// Locate offending sequences for diagnosis.
		var offenders []string
		for _, m := range ansiSGR.FindAllString(stripped, -1) {
			if hasBgCode(m) {
				offenders = append(offenders, m)
			}
		}
		t.Errorf("background codes remain after stripANSIBg: %v", offenders)
	}
}
