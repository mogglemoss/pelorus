// Package archive implements a read-only provider.Provider that exposes the
// contents of a supported archive file (.zip, .tar, .tar.gz, .tgz,
// .tar.bz2, .tar.xz) as a virtual directory tree.
package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ulikunitz/xz"

	"github.com/mogglemoss/pelorus/internal/provider"
	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

// ErrNotSupported is returned for write operations on an archive provider.
var ErrNotSupported = errors.New("operation not supported on archives")

// archiveKind identifies which format an archive uses.
type archiveKind int

const (
	kindUnknown archiveKind = iota
	kindZip
	kindTar
	kindTarGz
	kindTarBz2
	kindTarXz
)

// Provider implements provider.Provider for a single archive file.
type Provider struct {
	archivePath string
	localProv   provider.Provider
	kind        archiveKind
}

// New creates an archive Provider for the file at archivePath.
// localProv is used only for Capabilities delegation.
func New(archivePath string, localProv provider.Provider) (*Provider, error) {
	k := detectKind(archivePath)
	if k == kindUnknown {
		return nil, fmt.Errorf("unsupported archive format: %s", filepath.Base(archivePath))
	}
	return &Provider{
		archivePath: archivePath,
		localProv:   localProv,
		kind:        k,
	}, nil
}

func (p *Provider) String() string {
	return fmt.Sprintf("archive(%s)", filepath.Base(p.archivePath))
}

// ArchivePath returns the absolute path of the underlying archive file.
func (p *Provider) ArchivePath() string {
	return p.archivePath
}

// Capabilities returns the archive provider's capability set.
func (p *Provider) Capabilities() provider.Caps {
	return provider.Caps{
		CanSetPermissions: false,
		CanSymlink:        false,
		CanPreview:        true,
		CanTrash:          false,
		IsRemote:          false,
		SupportsArchive:   false,
	}
}

// --- Virtual path helpers ---

// internalPath strips the archivePath+"/" prefix from virtualPath and returns
// the path inside the archive. Returns "" for the archive root.
func (p *Provider) internalPath(virtualPath string) string {
	root := p.archivePath + "/"
	if virtualPath == p.archivePath || virtualPath == root {
		return ""
	}
	return strings.TrimPrefix(virtualPath, root)
}

// virtualPathFor converts an internal archive path to a virtual path usable
// externally (i.e. archivePath + "/" + internal).
func (p *Provider) virtualPathFor(internal string) string {
	if internal == "" {
		return p.archivePath + "/"
	}
	return p.archivePath + "/" + internal
}

// --- List ---

// List returns the direct children of the virtual directory at virtualPath.
func (p *Provider) List(virtualPath string) ([]fileinfo.FileInfo, error) {
	internal := p.internalPath(virtualPath)
	// Normalise: no leading/trailing slash.
	internal = strings.Trim(internal, "/")

	switch p.kind {
	case kindZip:
		return p.listZip(internal)
	default:
		return p.listTar(internal)
	}
}

func (p *Provider) listZip(dirInternal string) ([]fileinfo.FileInfo, error) {
	zr, err := zip.OpenReader(p.archivePath)
	if err != nil {
		return nil, fmt.Errorf("open zip %q: %w", p.archivePath, err)
	}
	defer zr.Close()

	return collectEntries(dirInternal, func(visit func(name string, isDir bool, size int64, mod time.Time, mode os.FileMode)) {
		for _, f := range zr.File {
			name := path.Clean(f.Name)
			if name == "." {
				continue
			}
			isDir := f.FileInfo().IsDir()
			visit(name, isDir, int64(f.UncompressedSize64), f.Modified, f.Mode())
		}
	}), nil
}

func (p *Provider) listTar(dirInternal string) ([]fileinfo.FileInfo, error) {
	tr, closer, err := p.openTar()
	if err != nil {
		return nil, err
	}
	defer closer()

	return collectEntries(dirInternal, func(visit func(name string, isDir bool, size int64, mod time.Time, mode os.FileMode)) {
		for {
			hdr, err := tr.Next()
			if err != nil {
				break
			}
			name := path.Clean(hdr.Name)
			if name == "." {
				continue
			}
			isDir := hdr.Typeflag == tar.TypeDir
			visit(name, isDir, hdr.Size, hdr.ModTime, hdr.FileInfo().Mode())
		}
	}), nil
}

// collectEntries iterates all archive entries via the provided callback and
// returns the direct children of dirInternal.
func collectEntries(
	dirInternal string,
	walk func(visit func(name string, isDir bool, size int64, mod time.Time, mode os.FileMode)),
) []fileinfo.FileInfo {
	// seen tracks entries we've already added to avoid duplicates (mainly for
	// implicit directories inferred from nested paths).
	seen := make(map[string]bool)
	var result []fileinfo.FileInfo

	addEntry := func(name string, isDir bool, size int64, mod time.Time, mode os.FileMode) {
		if seen[name] {
			return
		}
		seen[name] = true
		fi := fileinfo.FileInfo{
			Name:    path.Base(name),
			Path:    name, // will be replaced by caller if needed
			Size:    size,
			Mode:    mode,
			ModTime: mod,
			IsDir:   isDir,
		}
		result = append(result, fi)
	}

	walk(func(entryName string, isDir bool, size int64, mod time.Time, mode os.FileMode) {
		// entryName is the cleaned path inside the archive, e.g. "a/b/c.txt".

		// Infer all implicit ancestor directories for this entry.
		parts := strings.Split(entryName, "/")
		for i := 1; i < len(parts); i++ {
			ancestor := strings.Join(parts[:i], "/")
			if !seen[ancestor] {
				// Only add the ancestor if it's a direct child of dirInternal.
				if isDirectChild(dirInternal, ancestor) {
					addEntry(ancestor, true, 0, time.Time{}, 0755|os.ModeDir)
				} else {
					seen[ancestor] = true // mark to avoid future rechecks
				}
			}
		}

		// Add the entry itself if it is a direct child of dirInternal.
		if isDirectChild(dirInternal, entryName) {
			addEntry(entryName, isDir, size, mod, mode)
		}
	})

	// Replace raw internal paths with proper virtual paths (base name is
	// already set; just rewrite Path to the full internal path for callers that
	// construct virtual paths themselves).  We keep Path as the internal path;
	// pane/provider consumers build virtual paths via archivePath+"/"+Path.
	//
	// Actually: callers use p.virtualPathFor(fi.Path), so keep Path = internal.
	return result
}

// isDirectChild returns true when child is an immediate child of parent.
// parent == "" means the archive root.
func isDirectChild(parent, child string) bool {
	child = strings.Trim(child, "/")
	parent = strings.Trim(parent, "/")

	if parent == "" {
		// Root: child must have no "/" in it.
		return !strings.Contains(child, "/")
	}
	if !strings.HasPrefix(child, parent+"/") {
		return false
	}
	rest := child[len(parent)+1:]
	return !strings.Contains(rest, "/")
}

// --- Stat ---

// Stat returns metadata for the given virtualPath.
func (p *Provider) Stat(virtualPath string) (fileinfo.FileInfo, error) {
	internal := strings.Trim(p.internalPath(virtualPath), "/")

	// Archive root is always a directory.
	if internal == "" {
		info, err := os.Stat(p.archivePath)
		if err != nil {
			return fileinfo.FileInfo{}, err
		}
		return fileinfo.FileInfo{
			Name:    filepath.Base(p.archivePath),
			Path:    p.archivePath + "/",
			Size:    0,
			Mode:    0755 | os.ModeDir,
			ModTime: info.ModTime(),
			IsDir:   true,
		}, nil
	}

	entries, err := p.List(filepath.Dir(virtualPath))
	if err != nil {
		return fileinfo.FileInfo{}, err
	}
	for _, e := range entries {
		if e.Name == path.Base(internal) {
			return e, nil
		}
	}
	return fileinfo.FileInfo{}, fmt.Errorf("stat %q: not found in archive", virtualPath)
}

// --- Read ---

// Read returns an io.ReadCloser for the file at virtualPath within the archive.
func (p *Provider) Read(virtualPath string) (io.ReadCloser, error) {
	internal := strings.Trim(p.internalPath(virtualPath), "/")
	if internal == "" {
		return nil, fmt.Errorf("read: cannot read archive root as a file")
	}

	switch p.kind {
	case kindZip:
		return p.readZip(internal)
	default:
		return p.readTar(internal)
	}
}

func (p *Provider) readZip(internal string) (io.ReadCloser, error) {
	zr, err := zip.OpenReader(p.archivePath)
	if err != nil {
		return nil, fmt.Errorf("open zip %q: %w", p.archivePath, err)
	}
	for _, f := range zr.File {
		if path.Clean(f.Name) == internal {
			rc, err := f.Open()
			if err != nil {
				zr.Close()
				return nil, err
			}
			// Return a ReadCloser that closes both the entry and the zip reader.
			return &zipEntry{rc: rc, zr: zr}, nil
		}
	}
	zr.Close()
	return nil, fmt.Errorf("read: %q not found in zip", internal)
}

type zipEntry struct {
	rc io.ReadCloser
	zr *zip.ReadCloser
}

func (ze *zipEntry) Read(b []byte) (int, error) { return ze.rc.Read(b) }
func (ze *zipEntry) Close() error {
	err := ze.rc.Close()
	ze.zr.Close()
	return err
}

func (p *Provider) readTar(internal string) (io.ReadCloser, error) {
	tr, closer, err := p.openTar()
	if err != nil {
		return nil, err
	}
	for {
		hdr, err := tr.Next()
		if err != nil {
			closer()
			return nil, fmt.Errorf("read: %q not found in tar", internal)
		}
		if path.Clean(hdr.Name) == internal {
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, tr); err != nil {
				closer()
				return nil, fmt.Errorf("read tar entry %q: %w", internal, err)
			}
			closer()
			return io.NopCloser(&buf), nil
		}
	}
}

// --- Copy (extract) ---

// Copy extracts the archive entry at src (virtual path) to dst (real filesystem path).
// If src is a virtual directory, all children are extracted recursively.
func (p *Provider) Copy(src, dst string) error {
	internal := strings.Trim(p.internalPath(src), "/")

	switch p.kind {
	case kindZip:
		return p.extractZip(internal, dst)
	default:
		return p.extractTar(internal, dst)
	}
}

func (p *Provider) extractZip(internal, dst string) error {
	zr, err := zip.OpenReader(p.archivePath)
	if err != nil {
		return fmt.Errorf("open zip %q: %w", p.archivePath, err)
	}
	defer zr.Close()

	matched := false
	for _, f := range zr.File {
		name := path.Clean(f.Name)
		var rel string
		if internal == "" {
			// Extracting archive root: rel = name itself.
			rel = name
		} else if name == internal {
			rel = filepath.Base(internal)
		} else if strings.HasPrefix(name, internal+"/") {
			rel = filepath.Join(filepath.Base(internal), strings.TrimPrefix(name, internal+"/"))
		} else {
			continue
		}

		matched = true
		target := filepath.Join(dst, rel)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
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
		if err := writeFile(target, rc, f.Mode()); err != nil {
			rc.Close()
			return err
		}
		rc.Close()
	}
	if !matched {
		return fmt.Errorf("extract: %q not found in zip", internal)
	}
	return nil
}

func (p *Provider) extractTar(internal, dst string) error {
	tr, closer, err := p.openTar()
	if err != nil {
		return err
	}
	defer closer()

	matched := false
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		name := path.Clean(hdr.Name)

		var rel string
		if internal == "" {
			rel = name
		} else if name == internal {
			rel = filepath.Base(internal)
		} else if strings.HasPrefix(name, internal+"/") {
			rel = filepath.Join(filepath.Base(internal), strings.TrimPrefix(name, internal+"/"))
		} else {
			continue
		}

		matched = true
		target := filepath.Join(dst, rel)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		default:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := writeFile(target, tr, hdr.FileInfo().Mode()); err != nil {
				return err
			}
		}
	}
	if !matched {
		return fmt.Errorf("extract: %q not found in tar", internal)
	}
	return nil
}

func writeFile(dst string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

// --- Unsupported write operations ---

func (p *Provider) Move(src, dst string) error   { return ErrNotSupported }
func (p *Provider) Delete(path string) error      { return ErrNotSupported }
func (p *Provider) MakeDir(path string) error     { return ErrNotSupported }
func (p *Provider) Rename(src, dst string) error  { return ErrNotSupported }

// --- Format detection ---

// IsArchive reports whether name has a supported archive extension.
func IsArchive(name string) bool {
	return detectKind(name) != kindUnknown
}

func detectKind(name string) archiveKind {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return kindZip
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return kindTarGz
	case strings.HasSuffix(lower, ".tar.bz2"):
		return kindTarBz2
	case strings.HasSuffix(lower, ".tar.xz"):
		return kindTarXz
	case strings.HasSuffix(lower, ".tar"):
		return kindTar
	default:
		return kindUnknown
	}
}

// kindLabel returns a short label for display in the status bar.
func (p *Provider) KindLabel() string {
	switch p.kind {
	case kindZip:
		return "zip"
	case kindTar:
		return "tar"
	case kindTarGz:
		return "tar.gz"
	case kindTarBz2:
		return "tar.bz2"
	case kindTarXz:
		return "tar.xz"
	default:
		return "archive"
	}
}

// --- Tar helpers ---

// openTar opens the archive file and wraps it in the appropriate decompressor,
// returning a *tar.Reader and a closer function.
func (p *Provider) openTar() (*tar.Reader, func(), error) {
	f, err := os.Open(p.archivePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open %q: %w", p.archivePath, err)
	}

	var r io.Reader = f
	var extra io.Closer

	switch p.kind {
	case kindTarGz:
		gr, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, nil, fmt.Errorf("gzip reader %q: %w", p.archivePath, err)
		}
		r = gr
		extra = gr
	case kindTarBz2:
		r = bzip2.NewReader(f)
	case kindTarXz:
		xr, err := xz.NewReader(f)
		if err != nil {
			f.Close()
			return nil, nil, fmt.Errorf("xz reader %q: %w", p.archivePath, err)
		}
		r = xr
	}

	closer := func() {
		if extra != nil {
			extra.Close()
		}
		f.Close()
	}

	return tar.NewReader(r), closer, nil
}
