package theme

import "testing"

// TestBorderBackgroundBaked ensures every theme sets BorderBackground on its
// border styles, so the border glyphs themselves render on the pane colour
// instead of the user's terminal bg.
func TestBorderBackgroundBaked(t *testing.T) {
	for _, name := range []string{"haruspex", "gruvbox", "nord", "light", "dracula", "catppuccin"} {
		th := Get(name)
		paneBg := th.ActiveBorder.GetBackground()
		if paneBg == nil {
			t.Fatalf("%s: no pane bg", name)
		}
		// BorderBackground sets all four sides; check the left as a proxy.
		for label, got := range map[string]interface{}{
			"ActiveBorder":   th.ActiveBorder.GetBorderLeftBackground(),
			"InactiveBorder": th.InactiveBorder.GetBorderLeftBackground(),
			"PreviewBorder":  th.PreviewBorder.GetBorderLeftBackground(),
		} {
			if got != paneBg {
				t.Errorf("%s/%s: BorderBackground=%v, want paneBg=%v", name, label, got, paneBg)
			}
		}
	}
}
