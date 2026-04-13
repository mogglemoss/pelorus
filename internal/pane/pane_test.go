package pane

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mogglemoss/pelorus/internal/provider/local"
	"github.com/mogglemoss/pelorus/internal/theme"
)

// stripANSI removes ANSI escape sequences so we can count raw lines.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func countLines(s string) int { return len(strings.Split(s, "\n")) }

func testTheme() *theme.Theme {
	t := theme.PelorusTheme()
	return &t
}

// keyMsg constructs a tea.KeyMsg for the given key name.
// Supports single runes ("l", "a", "/"), plus named keys: "esc", "enter", "backspace".
func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	default:
		r := []rune(key)
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: r}
	}
}

// makeTestDir creates:
//
//	root/
//	    alpha.txt
//	    beta.txt
//	    sub-dir/
//	        child.txt
//	    link-to-sub  -> sub-dir       (symlink → dir)
//	    link-to-alpha -> alpha.txt    (symlink → file)
//	    broken-link  -> nonexistent   (broken symlink)
func makeTestDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	sub := filepath.Join(root, "sub-dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "child.txt"), []byte("child"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha.txt", "beta.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(sub, filepath.Join(root, "link-to-sub")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "alpha.txt"), filepath.Join(root, "link-to-alpha")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "nonexistent"), filepath.Join(root, "broken-link")); err != nil {
		t.Fatal(err)
	}
	return root
}

func newPane(t *testing.T, path string) *Model {
	t.Helper()
	th := testTheme()
	p := New(path, local.New(), th, false)
	p.Width = 60
	p.Height = 20
	return p
}

// findEntry returns the index of the named entry in p.Filtered, or -1.
func findEntry(p *Model, name string) int {
	for i, fi := range p.Filtered {
		if fi.Name == name {
			return i
		}
	}
	return -1
}

// =============================================================================
// Symlink navigation
// =============================================================================

func TestEnterSelectedOnSymlinkDir(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	idx := findEntry(p, "link-to-sub")
	if idx == -1 {
		t.Fatal("link-to-sub not found in listing")
	}
	p.Cursor = idx

	cmd := p.EnterSelected()

	// Directory entries (including symlink-to-dir) navigate inline: cmd == nil.
	if cmd != nil {
		t.Error("EnterSelected on symlink-dir returned non-nil cmd; expected directory navigation (nil cmd)")
	}

	// Path is set to the symlink's own path.
	wantPath := filepath.Join(root, "link-to-sub")
	if p.Path != wantPath {
		t.Errorf("Path after entering symlink-dir = %q, want %q", p.Path, wantPath)
	}

	// Contents of the target should be loaded.
	if findEntry(p, "child.txt") == -1 {
		t.Errorf("child.txt not found after entering link-to-sub (entries: %d)", len(p.Filtered))
	}
}

func TestEnterSelectedOnSymlinkFile(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	idx := findEntry(p, "link-to-alpha")
	if idx == -1 {
		t.Fatal("link-to-alpha not found in listing")
	}
	p.Cursor = idx
	originalPath := p.Path

	cmd := p.EnterSelected()

	if cmd == nil {
		t.Fatal("EnterSelected on symlink-file returned nil; expected OpenFileMsg")
	}
	msg := cmd()
	if _, ok := msg.(OpenFileMsg); !ok {
		t.Errorf("EnterSelected on symlink-file returned %T, want OpenFileMsg", msg)
	}
	if p.Path != originalPath {
		t.Errorf("path changed after entering symlink-file: %q", p.Path)
	}
}

// TestEnterSelectedOnBrokenSymlink documents a bug: a broken symlink falls
// through to OpenFileMsg, which will fail in the editor since the target does
// not exist. The correct behaviour is to return nil (no-op) or an ErrMsg.
func TestEnterSelectedOnBrokenSymlink(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	idx := findEntry(p, "broken-link")
	if idx == -1 {
		t.Fatal("broken-link not found in listing")
	}
	p.Cursor = idx
	originalPath := p.Path

	cmd := p.EnterSelected()

	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(OpenFileMsg); ok {
			// This is the known bug: opening a broken symlink path will error.
			t.Error("BUG: EnterSelected on broken symlink returns OpenFileMsg — " +
				"the path does not exist; should return nil or ErrMsg instead")
		}
	}

	if p.Path != originalPath {
		t.Errorf("path changed after entering broken symlink: %q", p.Path)
	}
}

func TestGoParentAfterSymlinkEntry(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	// Manually set path to the symlink path (as EnterSelected would do).
	p.Path = filepath.Join(root, "link-to-sub")
	p.Reload()

	p.GoParent()

	if p.Path != root {
		t.Errorf("GoParent after symlink entry: Path = %q, want %q", p.Path, root)
	}
}

// TestNavigateIntoSymlinkThenSubdir verifies multi-level navigation through a
// symlink: enter symlink dir → enter subdir inside it → GoParent → back to symlink.
func TestNavigateIntoSymlinkThenSubdir(t *testing.T) {
	// Create: root/link → target/  where target/ contains  nested/
	root := t.TempDir()
	target := filepath.Join(root, "target")
	nested := filepath.Join(target, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(root, "link")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatal(err)
	}

	p := newPane(t, root)

	// Enter link.
	p.Path = linkPath
	p.Reload()
	if p.Path != linkPath {
		t.Fatalf("expected path %q, got %q", linkPath, p.Path)
	}

	// Enter nested inside link.
	idx := findEntry(p, "nested")
	if idx == -1 {
		t.Fatal("nested not found inside link")
	}
	p.Cursor = idx
	p.EnterSelected()

	wantDeep := filepath.Join(linkPath, "nested")
	if p.Path != wantDeep {
		t.Errorf("after entering nested: Path = %q, want %q", p.Path, wantDeep)
	}

	// GoParent should return to linkPath.
	p.GoParent()
	if p.Path != linkPath {
		t.Errorf("GoParent from nested: Path = %q, want %q", p.Path, linkPath)
	}
}

// =============================================================================
// Filter mode
// =============================================================================

func TestFilterModeEnterConfirmsSelection(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	// Filter to a directory entry.
	p.StartFilter()
	p.FilterStr = "sub"
	p.ApplyFilterPublic()

	if findEntry(p, "sub-dir") == -1 {
		t.Fatal("sub-dir not found after filter 'sub'")
	}
	p.Cursor = 0 // sub-dir should be first (only dir match)

	// Press Enter — should navigate into sub-dir and exit filter mode.
	p, cmd := p.Update(keyMsg("enter"))

	if cmd != nil {
		t.Errorf("Enter on directory in filter mode: expected nil cmd (inline nav), got %T", cmd())
	}
	if p.Mode != ModeNormal {
		t.Errorf("after Enter in filter mode: Mode = %v, want ModeNormal", p.Mode)
	}
	if p.FilterStr != "" {
		t.Errorf("after Enter in filter mode: FilterStr = %q, want empty", p.FilterStr)
	}
	wantPath := filepath.Join(root, "sub-dir")
	if p.Path != wantPath {
		t.Errorf("after Enter in filter mode: Path = %q, want %q", p.Path, wantPath)
	}
}

func TestFilterModeEnterOnFileReturnsOpenMsg(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	// Filter to a file entry.
	p.StartFilter()
	p.FilterStr = "alpha"
	p.ApplyFilterPublic()

	idx := findEntry(p, "alpha.txt")
	if idx == -1 {
		t.Fatal("alpha.txt not found after filter 'alpha'")
	}
	p.Cursor = idx

	p, cmd := p.Update(keyMsg("enter"))

	if cmd == nil {
		t.Fatal("Enter on file in filter mode: expected OpenFileMsg cmd, got nil")
	}
	if _, ok := cmd().(OpenFileMsg); !ok {
		t.Errorf("Enter on file in filter mode: got %T, want OpenFileMsg", cmd())
	}
	if p.Mode != ModeNormal {
		t.Errorf("after Enter on file in filter mode: Mode = %v, want ModeNormal", p.Mode)
	}
	if p.FilterStr != "" {
		t.Errorf("after Enter on file in filter mode: FilterStr = %q, want empty", p.FilterStr)
	}
}

func TestFilterModeEscClears(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	p.StartFilter()
	p.FilterStr = "alp"
	p.ApplyFilterPublic()

	p, _ = p.Update(keyMsg("esc"))

	if p.Mode != ModeNormal {
		t.Errorf("after ESC: Mode = %v, want ModeNormal", p.Mode)
	}
	if p.FilterStr != "" {
		t.Errorf("after ESC: FilterStr = %q, want empty", p.FilterStr)
	}
	if len(p.Filtered) != len(p.Entries) {
		t.Errorf("after ESC: Filtered (%d) != Entries (%d)", len(p.Filtered), len(p.Entries))
	}
}

func TestFilterModeBackspaceToNormal(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	p.StartFilter()
	// Empty filter + backspace exits filter mode.
	p, _ = p.Update(keyMsg("backspace"))

	if p.Mode != ModeNormal {
		t.Errorf("backspace on empty filter: Mode = %v, want ModeNormal", p.Mode)
	}
}

func TestFilterModeShrinksByBackspace(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	p.StartFilter()
	p.FilterStr = "al"
	p.ApplyFilterPublic()

	p, _ = p.Update(keyMsg("backspace")) // removes 'l', leaving "a"
	if p.FilterStr != "a" {
		t.Errorf("backspace: FilterStr = %q, want %q", p.FilterStr, "a")
	}
	if p.Mode != ModeFilter {
		t.Errorf("backspace mid-filter: Mode = %v, want ModeFilter", p.Mode)
	}
}

func TestFilterClearedOnReload(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	p.StartFilter()
	p.FilterStr = "alpha"
	p.ApplyFilterPublic()

	p.Path = filepath.Join(root, "sub-dir")
	p.Reload()

	if p.FilterStr != "" {
		t.Errorf("FilterStr not cleared after Reload: %q", p.FilterStr)
	}
	if p.Mode != ModeNormal {
		t.Errorf("Mode not reset after Reload: %v", p.Mode)
	}
}

func TestFilterFuzzyMatch(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	p.StartFilter()
	p.FilterStr = "alp"
	p.ApplyFilterPublic()

	if findEntry(p, "alpha.txt") == -1 {
		t.Error("fuzzy filter 'alp' should match alpha.txt")
	}
}

func TestFilterNoMatchEmptiesFiltered(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	p.StartFilter()
	p.FilterStr = "zzznomatch"
	p.ApplyFilterPublic()

	if len(p.Filtered) != 0 {
		names := make([]string, len(p.Filtered))
		for i, fi := range p.Filtered {
			names[i] = fi.Name
		}
		t.Errorf("expected no matches for 'zzznomatch', got: %v", names)
	}
}

func TestFilterCursorResetsToZero(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	p.Cursor = len(p.Filtered) - 1

	p.StartFilter()
	p.FilterStr = "alpha"
	p.ApplyFilterPublic()

	if p.Cursor != 0 {
		t.Errorf("cursor not reset after filter: %d, want 0", p.Cursor)
	}
}

// =============================================================================
// Cursor / clamp
// =============================================================================

func TestCursorClampEmptyDir(t *testing.T) {
	p := newPane(t, t.TempDir())

	if p.Cursor != 0 {
		t.Errorf("cursor in empty dir = %d, want 0", p.Cursor)
	}
	if p.Selected() != nil {
		t.Error("Selected() should be nil in empty dir")
	}
}

func TestCursorDownClampsAtBottom(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	last := len(p.Filtered) - 1
	p.Cursor = last
	p.CursorDown()

	if p.Cursor != last {
		t.Errorf("CursorDown past end: %d, want %d", p.Cursor, last)
	}
}

func TestCursorUpClampsAtTop(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	p.Cursor = 0
	p.CursorUp()

	if p.Cursor != 0 {
		t.Errorf("CursorUp past top: %d, want 0", p.Cursor)
	}
}

func TestScrollDownClampsAtBottom(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	last := len(p.Filtered) - 1
	p.ScrollDown(9999)

	if p.Cursor != last {
		t.Errorf("ScrollDown huge: %d, want %d", p.Cursor, last)
	}
}

func TestScrollUpClampsAtTop(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	p.Cursor = last(p)
	p.ScrollUp(9999)

	if p.Cursor != 0 {
		t.Errorf("ScrollUp huge: %d, want 0", p.Cursor)
	}
}

func last(p *Model) int {
	if len(p.Filtered) == 0 {
		return 0
	}
	return len(p.Filtered) - 1
}

// =============================================================================
// View height consistency
// =============================================================================

// TestViewHeightWithoutFilter: rendered rows == m.Height.
func TestViewHeightWithoutFilter(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)
	p.IsActive = true

	got := countLines(stripANSI(p.View()))
	if got != p.Height {
		t.Errorf("View() without filter: %d rows, want %d", got, p.Height)
	}
}

// TestViewHeightWithFilter exposes the off-by-one in innerH accounting.
//
// When a filter is active, pane.View() decrements innerH and passes that to
// boxStyle.Height(innerH). But the content already accounts for the filter
// line separately, so the box ends up 1 row shorter than m.Height, causing
// misalignment with the divider in the full layout.
func TestViewHeightWithFilter(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)
	p.IsActive = true

	p.StartFilter()
	p.FilterStr = "a"
	p.ApplyFilterPublic()

	got := countLines(stripANSI(p.View()))
	if got != p.Height {
		t.Errorf("BUG: View() with active filter: %d rows, want %d — "+
			"innerH decrement causes pane to be 1 row shorter than allocated height",
			got, p.Height)
	}
}

// TestViewHeightGotoPathMode: height consistent in ModeGotoPath.
func TestViewHeightGotoPathMode(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)
	p.IsActive = true

	p.StartGotoPath()

	got := countLines(stripANSI(p.View()))
	if got != p.Height {
		t.Errorf("View() in ModeGotoPath: %d rows, want %d", got, p.Height)
	}
}

// TestViewHeightSmallPane: no panic for borderline-small dimensions.
func TestViewHeightSmallPane(t *testing.T) {
	root := makeTestDir(t)
	th := testTheme()
	p := New(root, local.New(), th, false)
	p.Width = 10
	p.Height = 6 // innerW=8, innerH=4 — just above the minimum

	output := p.View()
	if output == "" {
		t.Error("View() returned empty string for small pane")
	}
}

// TestViewTooSmallReturnsEmpty: below minimum dimensions returns "".
func TestViewTooSmallReturnsEmpty(t *testing.T) {
	root := makeTestDir(t)
	th := testTheme()
	p := New(root, local.New(), th, false)
	p.Width = 5
	p.Height = 4 // innerW=3 < 4, should return ""

	output := p.View()
	if output != "" {
		t.Errorf("View() below minimum: want empty string, got %q", output)
	}
}

// =============================================================================
// Normal-mode key passthrough (pane.Update is a no-op in ModeNormal)
// =============================================================================

// TestNormalModeUpdateIsNoOp: pane.Update in ModeNormal should not mutate state.
// Navigation keys are routed by app.go, not pane.Update.
func TestNormalModeUpdateIsNoOp(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	for _, key := range []string{"l", "h", "j", "k", "q", "p", "g"} {
		before := p.Path
		beforeFilter := p.FilterStr
		beforeMode := p.Mode

		p, _ = p.Update(keyMsg(key))

		if p.Path != before {
			t.Errorf("key %q in ModeNormal changed path: %q → %q", key, before, p.Path)
		}
		if p.FilterStr != beforeFilter {
			t.Errorf("key %q in ModeNormal set FilterStr: %q", key, p.FilterStr)
		}
		if p.Mode != beforeMode {
			t.Errorf("key %q in ModeNormal changed mode: %v → %v", key, beforeMode, p.Mode)
		}
	}
}

// =============================================================================
// Multi-select
// =============================================================================

func TestToggleSelection(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	sel := p.Selected()
	if sel == nil {
		t.Fatal("no entry selected")
	}
	path := sel.Path

	p.ToggleSelection()
	if !p.MultiSel[path] {
		t.Error("after ToggleSelection: entry not in MultiSel")
	}

	p.ToggleSelection()
	if p.MultiSel[path] {
		t.Error("after second ToggleSelection: entry still in MultiSel")
	}
}

func TestSelectedEntriesFallbackToCursor(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	entries := p.SelectedEntries()
	if len(entries) != 1 {
		t.Errorf("SelectedEntries with no multi-select: got %d, want 1", len(entries))
	}
}

func TestSelectedEntriesReturnsMarked(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	// Mark first two entries.
	p.Cursor = 0
	p.ToggleSelection()
	p.Cursor = 1
	p.ToggleSelection()

	entries := p.SelectedEntries()
	if len(entries) != 2 {
		t.Errorf("SelectedEntries with 2 marked: got %d, want 2", len(entries))
	}
}

func TestClearSelection(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	p.ToggleSelection()
	p.ClearSelection()

	if len(p.MultiSel) != 0 {
		t.Errorf("ClearSelection: MultiSel still has %d entries", len(p.MultiSel))
	}
}

// =============================================================================
// Sort cycling
// =============================================================================

func TestCycleSortWrapsAfterFour(t *testing.T) {
	root := makeTestDir(t)
	p := newPane(t, root)

	initial := p.SortMode
	for range 4 {
		p.CycleSort()
	}
	if p.SortMode != initial {
		t.Errorf("CycleSort did not wrap after 4 cycles: %v", p.SortMode)
	}
}
