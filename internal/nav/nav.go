package nav

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/mogglemoss/pelorus/internal/provider"
	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

// SortMode controls how directory entries are ordered within each group.
type SortMode int

const (
	SortName SortMode = iota // dirs-first, case-insensitive alpha (default)
	SortSize                  // dirs-first, descending size
	SortDate                  // dirs-first, descending ModTime
	SortExt                   // dirs-first, extension alpha then name alpha
)

// ReadDir reads and sorts directory contents from the given provider.
// Directories are listed first, then files, each group sorted according
// to the given SortMode. Hidden entries (starting with '.') are excluded
// unless showHidden is true.
func ReadDir(path string, p provider.Provider, showHidden bool, mode SortMode) ([]fileinfo.FileInfo, error) {
	entries, err := p.List(path)
	if err != nil {
		return nil, err
	}

	// Filter hidden files.
	visible := entries[:0]
	for _, e := range entries {
		if !showHidden && strings.HasPrefix(e.Name, ".") {
			continue
		}
		visible = append(visible, e)
	}

	// Sort: dirs first, then files; mode-aware ordering within groups.
	sort.SliceStable(visible, func(i, j int) bool {
		a, b := visible[i], visible[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		switch mode {
		case SortSize:
			if a.Size != b.Size {
				return a.Size > b.Size
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case SortDate:
			if !a.ModTime.Equal(b.ModTime) {
				return a.ModTime.After(b.ModTime)
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case SortExt:
			ea := filepath.Ext(a.Name)
			eb := filepath.Ext(b.Name)
			if ea != eb {
				return strings.ToLower(ea) < strings.ToLower(eb)
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		default: // SortName
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
	})

	return visible, nil
}
