package local

import (
	"os"
	"path/filepath"
	"testing"
)

// makeTree creates a temp directory containing:
//
//	real-dir/
//	    inside.txt
//	real-file.txt
//	link-to-dir  -> real-dir
//	link-to-file -> real-file.txt
//	rel-link-dir -> real-dir          (relative target)
//	broken-link  -> nonexistent
func makeTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// real dir with a file inside
	realDir := filepath.Join(root, "real-dir")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "inside.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	// real file
	if err := os.WriteFile(filepath.Join(root, "real-file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// absolute symlink to directory
	if err := os.Symlink(realDir, filepath.Join(root, "link-to-dir")); err != nil {
		t.Fatal(err)
	}

	// absolute symlink to file
	if err := os.Symlink(filepath.Join(root, "real-file.txt"), filepath.Join(root, "link-to-file")); err != nil {
		t.Fatal(err)
	}

	// relative symlink to directory (target = "real-dir", resolved relative to root)
	if err := os.Symlink("real-dir", filepath.Join(root, "rel-link-dir")); err != nil {
		t.Fatal(err)
	}

	// broken symlink
	if err := os.Symlink(filepath.Join(root, "nonexistent"), filepath.Join(root, "broken-link")); err != nil {
		t.Fatal(err)
	}

	return root
}

func TestSymlinkToDirIsDir(t *testing.T) {
	root := makeTree(t)
	p := New()

	entries, err := p.List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	byName := map[string]interface{ GetName() string }{}
	_ = byName

	found := map[string]struct {
		IsDir     bool
		IsSymlink bool
		Broken    bool
	}{}
	for _, fi := range entries {
		found[fi.Name] = struct {
			IsDir     bool
			IsSymlink bool
			Broken    bool
		}{fi.IsDir, fi.IsSymlink, fi.SymlinkBroken}
	}

	cases := []struct {
		name      string
		wantDir   bool
		wantSym   bool
		wantBroke bool
	}{
		{"real-dir", true, false, false},
		{"real-file.txt", false, false, false},
		{"link-to-dir", true, true, false},   // symlink to dir → IsDir true
		{"link-to-file", false, true, false},  // symlink to file → IsDir false
		{"rel-link-dir", true, true, false},   // relative symlink to dir → IsDir true
		{"broken-link", false, true, true},    // broken → IsDir false, Broken true
	}

	for _, tc := range cases {
		got, ok := found[tc.name]
		if !ok {
			t.Errorf("entry %q not found in listing", tc.name)
			continue
		}
		if got.IsDir != tc.wantDir {
			t.Errorf("%s: IsDir = %v, want %v", tc.name, got.IsDir, tc.wantDir)
		}
		if got.IsSymlink != tc.wantSym {
			t.Errorf("%s: IsSymlink = %v, want %v", tc.name, got.IsSymlink, tc.wantSym)
		}
		if got.Broken != tc.wantBroke {
			t.Errorf("%s: SymlinkBroken = %v, want %v", tc.name, got.Broken, tc.wantBroke)
		}
	}
}

func TestSymlinkTargetPreserved(t *testing.T) {
	root := makeTree(t)
	p := New()

	entries, err := p.List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	for _, fi := range entries {
		if fi.Name == "link-to-dir" {
			if fi.SymlinkTarget == "" {
				t.Error("link-to-dir: SymlinkTarget should be non-empty")
			}
			return
		}
	}
	t.Error("link-to-dir not found")
}

func TestRelativeSymlinkResolvedCorrectly(t *testing.T) {
	root := makeTree(t)
	p := New()

	entries, err := p.List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	for _, fi := range entries {
		if fi.Name == "rel-link-dir" {
			if !fi.IsDir {
				t.Errorf("rel-link-dir: relative symlink to dir should have IsDir=true, got false")
			}
			if fi.SymlinkBroken {
				t.Error("rel-link-dir: should not be broken")
			}
			return
		}
	}
	t.Error("rel-link-dir not found")
}

// TestListSymlinkDirContents verifies that List() on a symlink-to-dir returns
// the target's contents (i.e., os.ReadDir follows the link).
func TestListSymlinkDirContents(t *testing.T) {
	root := makeTree(t)
	p := New()

	linkPath := filepath.Join(root, "link-to-dir")
	entries, err := p.List(linkPath)
	if err != nil {
		t.Fatalf("List(%q): %v", linkPath, err)
	}

	if len(entries) == 0 {
		t.Error("expected entries inside link-to-dir, got none")
	}

	found := false
	for _, fi := range entries {
		if fi.Name == "inside.txt" {
			found = true
		}
	}
	if !found {
		t.Error("inside.txt not found inside link-to-dir")
	}
}

// TestEntryPathsRootedAtSymlink verifies that entries returned from listing a
// symlink directory have paths rooted at the symlink path, not the target.
func TestEntryPathsRootedAtSymlink(t *testing.T) {
	root := makeTree(t)
	p := New()

	linkPath := filepath.Join(root, "link-to-dir")
	entries, err := p.List(linkPath)
	if err != nil {
		t.Fatalf("List(%q): %v", linkPath, err)
	}

	for _, fi := range entries {
		expected := filepath.Join(linkPath, fi.Name)
		if fi.Path != expected {
			t.Errorf("entry %q: Path = %q, want %q", fi.Name, fi.Path, expected)
		}
	}
}

// TestBrokenSymlinkNotNavigable verifies that a broken symlink has IsDir=false
// so EnterSelected will NOT attempt a directory navigation.
func TestBrokenSymlinkNotNavigable(t *testing.T) {
	root := makeTree(t)
	p := New()

	entries, err := p.List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	for _, fi := range entries {
		if fi.Name == "broken-link" {
			if fi.IsDir {
				t.Error("broken-link: IsDir should be false; navigating into it would call ReadDir on a broken path")
			}
			if !fi.IsSymlink {
				t.Error("broken-link: IsSymlink should be true")
			}
			if !fi.SymlinkBroken {
				t.Error("broken-link: SymlinkBroken should be true")
			}
			return
		}
	}
	t.Error("broken-link not found")
}

// TestCircularSymlink verifies that a circular/self-referential symlink doesn't
// cause a crash or infinite loop when listed.
func TestCircularSymlink(t *testing.T) {
	root := t.TempDir()

	// Create a symlink that points to itself: self -> self
	selfLink := filepath.Join(root, "self")
	if err := os.Symlink(selfLink, selfLink); err != nil {
		// Some OSes may reject self-referential symlinks at creation time.
		t.Skipf("OS rejected self-referential symlink: %v", err)
	}

	p := New()

	// Listing root should not crash; the circular link should appear as broken.
	entries, err := p.List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	for _, fi := range entries {
		if fi.Name == "self" {
			if fi.IsDir {
				t.Error("circular symlink should not appear as a directory")
			}
			// Either broken or at least not a navigable directory.
			return
		}
	}
}
