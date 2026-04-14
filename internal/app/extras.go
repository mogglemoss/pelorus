// "extras.go" houses the additional file-management actions that aren't core
// plumbing: duplicate, symlink, extract, chmod, quick info, glob-select, and
// recursive directory size. Each executor follows the same shape as
// executeRename / executeNewFile so the semantics stay consistent.
package app

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/mogglemoss/pelorus/internal/ops"
	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

// --- Duplicate -------------------------------------------------------------

// executeDuplicate copies the selected item(s) into the same directory with a
// "copy" suffix. Uses ops.UniquePath so repeated invocations produce
// "foo copy", "foo copy 2", "foo copy 3" etc.
//
// Runs synchronously for files; large directories still use the job queue so
// the UI doesn't block.
func (m *Model) executeDuplicate() tea.Cmd {
	ap := m.activeP()
	if ap.Provider.Capabilities().IsRemote {
		m.statusMsg = "Duplicate not yet available on remote panes"
		return nil
	}
	entries := ap.SelectedEntries()
	if len(entries) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, sel := range entries {
		dst := ops.UniquePath(filepath.Dir(sel.Path), sel.Name)
		job := m.queue.Add(ops.KindCopy, sel.Path, dst)
		job.Status = ops.StatusRunning
		cmds = append(cmds, ops.StartJob(job, ap.Provider, ap.Provider))
	}
	if len(entries) == 1 {
		m.statusMsg = fmt.Sprintf("Duplicating %q…", entries[0].Name)
	} else {
		m.statusMsg = fmt.Sprintf("Duplicating %d items…", len(entries))
	}
	ap.ClearSelection()
	cmds = append(cmds, m.startProgressTicker())
	return tea.Batch(cmds...)
}

// --- Symlink ---------------------------------------------------------------

// executeSymlink creates a symbolic link in the inactive pane pointing at the
// selected item in the active pane. Local-only (SFTP symlink support is
// possible but deferred — sftp clients have SymlinkFromRemote semantics that
// differ from local).
func (m *Model) executeSymlink() tea.Cmd {
	ap := m.activeP()
	ip := m.inactiveP()
	if ap.Provider.Capabilities().IsRemote || ip.Provider.Capabilities().IsRemote {
		m.statusMsg = "Symlink only supported between local panes"
		return nil
	}
	sel := ap.Selected()
	if sel == nil {
		return nil
	}
	dst := ops.UniquePath(ip.Path, sel.Name)
	if err := os.Symlink(sel.Path, dst); err != nil {
		m.statusMsg = "Symlink failed: " + err.Error()
		return nil
	}
	m.statusMsg = fmt.Sprintf("Symlinked %q → %s", filepath.Base(dst), sel.Path)
	ip.Reload()
	m.updateWatchers()
	return m.updatePreview()
}

// --- Extract archive -------------------------------------------------------

// executeExtract extracts the selected archive into the inactive pane's
// directory. Supports .zip, .tar, .tar.gz (.tgz), .tar.bz2.
//
// Streams one entry at a time; large archives don't block the UI for long.
func (m *Model) executeExtract() tea.Cmd {
	ap := m.activeP()
	ip := m.inactiveP()
	if ap.Provider.Capabilities().IsRemote || ip.Provider.Capabilities().IsRemote {
		m.statusMsg = "Extract only supported between local panes"
		return nil
	}
	sel := ap.Selected()
	if sel == nil || sel.IsDir {
		return nil
	}

	// Destination: a new subdirectory named after the archive (without extension)
	// inside the inactive pane. Guarantees no collision with existing files.
	base := archiveBaseName(sel.Name)
	destRoot := ops.UniquePath(ip.Path, base)

	return func() tea.Msg {
		if err := os.MkdirAll(destRoot, 0755); err != nil {
			return extractDoneMsg{err: err}
		}
		if err := extractArchive(sel.Path, destRoot); err != nil {
			return extractDoneMsg{err: err}
		}
		return extractDoneMsg{dest: destRoot}
	}
}

type extractDoneMsg struct {
	dest string
	err  error
}

// archiveBaseName strips compound extensions (.tar.gz, .tar.bz2, .tgz, .zip).
func archiveBaseName(name string) string {
	l := strings.ToLower(name)
	for _, suffix := range []string{".tar.gz", ".tar.bz2", ".tar.xz", ".tgz", ".tbz2", ".zip", ".tar"} {
		if strings.HasSuffix(l, suffix) {
			return name[:len(name)-len(suffix)]
		}
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// extractArchive dispatches to the appropriate extractor based on extension.
func extractArchive(src, dest string) error {
	l := strings.ToLower(src)
	switch {
	case strings.HasSuffix(l, ".zip"):
		return extractZip(src, dest)
	case strings.HasSuffix(l, ".tar"):
		return extractTarPlain(src, dest)
	case strings.HasSuffix(l, ".tar.gz"), strings.HasSuffix(l, ".tgz"):
		return extractTarGz(src, dest)
	case strings.HasSuffix(l, ".tar.bz2"), strings.HasSuffix(l, ".tbz2"):
		return extractTarBz2(src, dest)
	default:
		return errors.New("unsupported archive format (.zip, .tar, .tar.gz, .tar.bz2 only)")
	}
}

// safeJoin returns dest/name iff the result stays inside dest. Defends against
// path traversal entries like "../../etc/passwd" in untrusted archives.
func safeJoin(dest, name string) (string, error) {
	clean := filepath.Clean(name)
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("unsafe archive path: %q", name)
	}
	full := filepath.Join(dest, clean)
	rel, err := filepath.Rel(dest, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("unsafe archive path: %q", name)
	}
	return full, nil
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target, err := safeJoin(dest, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return err
		}
		rc.Close()
		out.Close()
	}
	return nil
}

func extractTarStream(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			_ = os.Symlink(hdr.Linkname, target)
		}
	}
}

func extractTarPlain(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	return extractTarStream(f, dest)
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	return extractTarStream(gz, dest)
}

func extractTarBz2(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	return extractTarStream(bzip2.NewReader(f), dest)
}

// --- chmod -----------------------------------------------------------------

// openChmodForm opens a Huh input asking for an octal mode like "755".
// Applies via os.Chmod on the selected file (local panes only).
func (m *Model) openChmodForm() tea.Cmd {
	ap := m.activeP()
	if ap.Provider.Capabilities().IsRemote {
		m.statusMsg = "chmod not yet available on remote panes"
		return nil
	}
	sel := ap.Selected()
	if sel == nil {
		return nil
	}
	// Pre-fill with current perms as octal.
	m.huhInputValue = fmt.Sprintf("%o", sel.Mode.Perm())
	m.huhMode = "chmod"
	f := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Change Permissions").
			Description(fmt.Sprintf("Octal mode for %q (current: %s)", sel.Name, sel.Mode.Perm().String())).
			Placeholder("e.g. 755, 644, 600").
			Value(&m.huhInputValue).
			Validate(func(s string) error {
				s = strings.TrimSpace(s)
				if s == "" {
					return errors.New("mode cannot be empty")
				}
				if _, err := parseOctalMode(s); err != nil {
					return err
				}
				return nil
			}),
	)).WithTheme(m.huhTheme()).WithWidth(56)
	m.huhOverlay = f
	return f.Init()
}

// executeChmod is invoked from onHuhComplete after the mode dialog closes.
func (m *Model) executeChmod(input string) tea.Cmd {
	mode, err := parseOctalMode(strings.TrimSpace(input))
	if err != nil {
		m.statusMsg = "chmod: " + err.Error()
		return nil
	}
	ap := m.activeP()
	sel := ap.Selected()
	if sel == nil {
		return nil
	}
	if err := os.Chmod(sel.Path, mode); err != nil {
		m.statusMsg = "chmod failed: " + err.Error()
		return nil
	}
	m.statusMsg = fmt.Sprintf("chmod %o %s", mode, sel.Name)
	ap.Reload()
	return m.updatePreview()
}

func parseOctalMode(s string) (os.FileMode, error) {
	if s == "" {
		return 0, errors.New("mode cannot be empty")
	}
	if len(s) > 4 {
		return 0, errors.New("mode must be 3 or 4 octal digits")
	}
	var v os.FileMode
	for _, c := range s {
		if c < '0' || c > '7' {
			return 0, errors.New("mode must be octal (0–7)")
		}
		v = v*8 + os.FileMode(c-'0')
	}
	return v & os.ModePerm, nil
}

// --- Quick info modal ------------------------------------------------------

// openQuickInfo flags the model to render a centered info card next frame.
// The actual rendering is done by renderQuickInfo() in overlay.go.
func (m *Model) openQuickInfo() {
	sel := m.activeP().Selected()
	if sel == nil {
		return
	}
	m.quickInfoItem = sel
	m.quickInfoOpen = true
}

// --- Glob multi-select -----------------------------------------------------

// openGlobSelectForm prompts for a glob pattern and multi-selects matching
// entries in the active pane's Filtered list.
func (m *Model) openGlobSelectForm() tea.Cmd {
	m.huhInputValue = ""
	m.huhMode = "glob-select"
	f := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Select by Pattern").
			Description("Glob pattern (e.g. *.go, test_*, **.log)").
			Placeholder("*.ext").
			Value(&m.huhInputValue).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("pattern cannot be empty")
				}
				if _, err := filepath.Match(strings.TrimSpace(s), "x"); err != nil {
					return err
				}
				return nil
			}),
	)).WithTheme(m.huhTheme()).WithWidth(56)
	m.huhOverlay = f
	return f.Init()
}

func (m *Model) executeGlobSelect(pattern string) tea.Cmd {
	pattern = strings.TrimSpace(pattern)
	ap := m.activeP()
	if ap.MultiSel == nil {
		ap.MultiSel = map[string]bool{}
	}
	matched := 0
	for _, fi := range ap.Filtered {
		ok, err := filepath.Match(pattern, fi.Name)
		if err == nil && ok {
			ap.MultiSel[fi.Path] = true
			matched++
		}
	}
	if matched == 0 {
		m.statusMsg = fmt.Sprintf("No matches for %q", pattern)
	} else {
		m.statusMsg = fmt.Sprintf("Selected %d items matching %q", matched, pattern)
	}
	return nil
}

// --- Recursive directory size ---------------------------------------------

type dirSizeDoneMsg struct {
	name  string
	size  int64
	count int
	err   error
}

// calcDirSize walks the selected directory on a goroutine and sends a
// dirSizeDoneMsg when complete. Works on local paths only; remote SFTP would
// be far too slow to be useful.
func (m *Model) calcDirSize() tea.Cmd {
	ap := m.activeP()
	if ap.Provider.Capabilities().IsRemote {
		m.statusMsg = "Directory size not available on remote panes"
		return nil
	}
	sel := ap.Selected()
	if sel == nil || !sel.IsDir {
		return nil
	}
	path := sel.Path
	name := sel.Name
	m.statusMsg = fmt.Sprintf("Calculating size of %q…", name)
	return func() tea.Msg {
		var total int64
		var count int
		err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // skip inaccessible entries
			}
			if info.Mode().IsRegular() {
				total += info.Size()
				count++
			}
			return nil
		})
		return dirSizeDoneMsg{name: name, size: total, count: count, err: err}
	}
}

// HumanSize wrapper — re-export so extras.go doesn't need a direct import.
var _ = fileinfo.HumanSize
