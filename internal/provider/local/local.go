package local

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mogglemoss/pelorus/internal/provider"
	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

// Provider implements provider.Provider for the local filesystem.
type Provider struct{}

// New returns a new local filesystem provider.
func New() *Provider {
	return &Provider{}
}

func (p *Provider) String() string {
	return "local"
}

func (p *Provider) Capabilities() provider.Caps {
	return provider.Caps{
		CanSetPermissions: true,
		CanSymlink:        true,
		CanPreview:        true,
		CanTrash:          true,
		IsRemote:          false,
		SupportsArchive:   false,
	}
}

func (p *Provider) List(path string) ([]fileinfo.FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("list %q: %w", path, err)
	}

	result := make([]fileinfo.FileInfo, 0, len(entries))
	for _, entry := range entries {
		fi, err := entryToFileInfo(path, entry)
		if err != nil {
			continue // skip unreadable entries
		}
		result = append(result, fi)
	}
	return result, nil
}

func (p *Provider) Stat(path string) (fileinfo.FileInfo, error) {
	linfo, err := os.Lstat(path)
	if err != nil {
		return fileinfo.FileInfo{}, fmt.Errorf("stat %q: %w", path, err)
	}
	return lstatToFileInfo(path, linfo), nil
}

func (p *Provider) Read(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return f, nil
}

func (p *Provider) Copy(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("copy stat %q: %w", src, err)
	}
	if srcInfo.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func (p *Provider) Move(src, dst string) error {
	// Try os.Rename first (same filesystem, fast path).
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// Cross-filesystem: copy then delete.
	if err := p.Copy(src, dst); err != nil {
		return fmt.Errorf("move copy phase: %w", err)
	}
	if err := p.Delete(src); err != nil {
		return fmt.Errorf("move delete phase: %w", err)
	}
	return nil
}

func (p *Provider) Delete(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("delete %q: %w", path, err)
	}
	return nil
}

func (p *Provider) MakeDir(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("mkdir %q: %w", path, err)
	}
	return nil
}

func (p *Provider) Rename(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("rename %q -> %q: %w", src, dst, err)
	}
	return nil
}

// --- helpers ---

func entryToFileInfo(dir string, entry os.DirEntry) (fileinfo.FileInfo, error) {
	fullPath := filepath.Join(dir, entry.Name())
	linfo, err := os.Lstat(fullPath)
	if err != nil {
		return fileinfo.FileInfo{}, err
	}
	return lstatToFileInfo(fullPath, linfo), nil
}

func lstatToFileInfo(path string, linfo os.FileInfo) fileinfo.FileInfo {
	fi := fileinfo.FileInfo{
		Name:    linfo.Name(),
		Path:    path,
		Size:    linfo.Size(),
		Mode:    linfo.Mode(),
		ModTime: linfo.ModTime(),
		IsDir:   linfo.IsDir(),
	}

	if linfo.Mode()&os.ModeSymlink != 0 {
		fi.IsSymlink = true
		target, err := os.Readlink(path)
		if err == nil {
			fi.SymlinkTarget = target
			// Resolve to detect broken symlinks.
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}
			if _, err := os.Stat(target); err != nil {
				fi.SymlinkBroken = true
			}
		}
	}

	return fi
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		s := filepath.Join(src, entry.Name())
		d := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
		} else {
			if err := copyFile(s, d); err != nil {
				return err
			}
		}
	}
	return nil
}
