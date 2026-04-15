package theme

import "testing"

// TestListItemStylesHavePaneBg ensures every theme bakes the pane background
// into DirName/FileName/SymlinkName so list rows stay on the pane colour
// instead of falling through to the user's terminal default bg (which
// manifested as invisible text on light theme in dark terminal sessions).
func TestListItemStylesHavePaneBg(t *testing.T) {
	for _, name := range []string{"haruspex", "gruvbox", "nord", "light", "dracula", "catppuccin"} {
		th := Get(name)
		paneBg := th.ActiveBorder.GetBackground()
		if paneBg == nil {
			t.Fatalf("%s: ActiveBorder has no background", name)
		}
		for styleName, gotBg := range map[string]interface{}{
			"DirName":     th.DirName.GetBackground(),
			"FileName":    th.FileName.GetBackground(),
			"SymlinkName": th.SymlinkName.GetBackground(),
			"MarkedEntry": th.MarkedEntry.GetBackground(),
		} {
			if gotBg != paneBg {
				t.Errorf("%s/%s: bg=%v, want paneBg=%v", name, styleName, gotBg, paneBg)
			}
		}
	}
}
