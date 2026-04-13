package nav

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mogglemoss/pelorus/internal/provider/local"
)

// makeTree creates a temp dir:
//
//	root/
//	    .hidden
//	    aardvark.go     (8 bytes)
//	    zebra.txt       (3 bytes)
//	    middle.md       (5 bytes)
//	    sub-dir/
//	    link-to-sub  -> sub-dir
func makeTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	sub := filepath.Join(root, "sub-dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		".hidden":      "hidden",
		"aardvark.go":  "aardvark",  // 8 bytes
		"zebra.txt":    "zeb",       // 3 bytes
		"middle.md":    "middl",     // 5 bytes
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(sub, filepath.Join(root, "link-to-sub")); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestReadDirExcludesHiddenByDefault(t *testing.T) {
	root := makeTree(t)
	p := local.New()

	entries, err := ReadDir(root, p, false, SortName)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	for _, fi := range entries {
		if fi.Name == ".hidden" {
			t.Error("hidden file '.hidden' should not appear when showHidden=false")
		}
	}
}

func TestReadDirIncludesHiddenWhenRequested(t *testing.T) {
	root := makeTree(t)
	p := local.New()

	entries, err := ReadDir(root, p, true, SortName)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	found := false
	for _, fi := range entries {
		if fi.Name == ".hidden" {
			found = true
		}
	}
	if !found {
		t.Error("hidden file '.hidden' should appear when showHidden=true")
	}
}

func TestReadDirDirsBeforeFiles(t *testing.T) {
	root := makeTree(t)
	p := local.New()

	for _, mode := range []SortMode{SortName, SortSize, SortDate, SortExt} {
		entries, err := ReadDir(root, p, false, mode)
		if err != nil {
			t.Fatalf("ReadDir(%v): %v", mode, err)
		}

		seenFile := false
		for _, fi := range entries {
			if !fi.IsDir {
				seenFile = true
			}
			if seenFile && fi.IsDir {
				t.Errorf("SortMode %v: directory %q appears after a file", mode, fi.Name)
			}
		}
	}
}

func TestReadDirSortNameAlpha(t *testing.T) {
	root := makeTree(t)
	p := local.New()

	entries, err := ReadDir(root, p, false, SortName)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	// Files only (dirs sorted separately — dirs come first).
	var files []string
	for _, fi := range entries {
		if !fi.IsDir {
			files = append(files, fi.Name)
		}
	}

	for i := 1; i < len(files); i++ {
		if files[i-1] > files[i] {
			t.Errorf("SortName: %q > %q at positions %d,%d", files[i-1], files[i], i-1, i)
		}
	}
}

func TestReadDirSortSizeDescending(t *testing.T) {
	root := makeTree(t)
	p := local.New()

	entries, err := ReadDir(root, p, false, SortSize)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	var files []int64
	for _, fi := range entries {
		if !fi.IsDir {
			files = append(files, fi.Size)
		}
	}

	for i := 1; i < len(files); i++ {
		if files[i-1] < files[i] {
			t.Errorf("SortSize: size[%d]=%d < size[%d]=%d — not descending", i-1, files[i-1], i, files[i])
		}
	}
}

func TestReadDirSortDateDescending(t *testing.T) {
	root := t.TempDir()
	p := local.New()

	// Create files with different mtime by touching with os.Chtimes.
	base := time.Now()
	type f struct {
		name  string
		mtime time.Time
	}
	items := []f{
		{"older.txt", base.Add(-2 * time.Hour)},
		{"middle.txt", base.Add(-1 * time.Hour)},
		{"newest.txt", base},
	}
	for _, item := range items {
		path := filepath.Join(root, item.name)
		if err := os.WriteFile(path, []byte(item.name), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, item.mtime, item.mtime); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := ReadDir(root, p, false, SortDate)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	for i := 1; i < len(entries); i++ {
		if entries[i-1].ModTime.Before(entries[i].ModTime) {
			t.Errorf("SortDate: entry[%d] (%s) is older than entry[%d] (%s)",
				i-1, entries[i-1].Name, i, entries[i].Name)
		}
	}
}

func TestReadDirSortExtension(t *testing.T) {
	root := t.TempDir()
	p := local.New()

	for _, name := range []string{"b.txt", "a.go", "c.md", "d.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := ReadDir(root, p, false, SortExt)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	// Files should be grouped by extension: .go < .md < .txt.
	// Within the same extension, alphabetical by name.
	var names []string
	for _, fi := range entries {
		if !fi.IsDir {
			names = append(names, fi.Name)
		}
	}

	want := []string{"a.go", "c.md", "b.txt", "d.txt"}
	if len(names) != len(want) {
		t.Fatalf("SortExt: got %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("SortExt[%d]: got %q, want %q", i, names[i], want[i])
		}
	}
}

func TestReadDirEmptyDir(t *testing.T) {
	root := t.TempDir()
	p := local.New()

	entries, err := ReadDir(root, p, false, SortName)
	if err != nil {
		t.Fatalf("ReadDir on empty dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries in empty dir, got %d", len(entries))
	}
}

func TestReadDirNonExistentPath(t *testing.T) {
	p := local.New()

	_, err := ReadDir("/nonexistent/path/that/does/not/exist", p, false, SortName)
	if err == nil {
		t.Error("ReadDir on non-existent path should return error")
	}
}

// TestReadDirSymlinkDirCounted verifies that a symlink-to-dir is listed and
// counts as a directory (sorted with dirs group).
func TestReadDirSymlinkDirCountedAsDir(t *testing.T) {
	root := makeTree(t)
	p := local.New()

	entries, err := ReadDir(root, p, false, SortName)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	var dirNames []string
	for _, fi := range entries {
		if fi.IsDir {
			dirNames = append(dirNames, fi.Name)
		}
	}

	// Both sub-dir and link-to-sub should be in the dirs group.
	found := map[string]bool{}
	for _, n := range dirNames {
		found[n] = true
	}
	for _, want := range []string{"sub-dir", "link-to-sub"} {
		if !found[want] {
			t.Errorf("%q not found in dirs group (dirs: %v)", want, dirNames)
		}
	}
}

// TestReadDirCaseInsensitiveSort verifies case-insensitive ordering.
func TestReadDirCaseInsensitiveSort(t *testing.T) {
	root := t.TempDir()
	p := local.New()

	for _, name := range []string{"Banana.txt", "apple.txt", "Cherry.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := ReadDir(root, p, false, SortName)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	var names []string
	for _, fi := range entries {
		names = append(names, fi.Name)
	}

	want := []string{"apple.txt", "Banana.txt", "Cherry.txt"}
	for i, w := range want {
		if i >= len(names) || names[i] != w {
			t.Errorf("case-insensitive sort: got %v, want %v", names, want)
			break
		}
	}
}
