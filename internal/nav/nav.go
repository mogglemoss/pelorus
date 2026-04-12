package nav

import (
	"sort"
	"strings"

	"github.com/mogglemoss/pelorus/internal/provider"
	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

// ReadDir reads and sorts directory contents from the given provider.
// Directories are listed first, then files, each group sorted
// case-insensitively alphabetical. Hidden entries (starting with '.')
// are excluded unless showHidden is true.
func ReadDir(path string, p provider.Provider, showHidden bool) ([]fileinfo.FileInfo, error) {
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

	// Sort: dirs first, then files; case-insensitive alphabetical within groups.
	sort.SliceStable(visible, func(i, j int) bool {
		a, b := visible[i], visible[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})

	return visible, nil
}
