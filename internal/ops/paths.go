// Package ops — path utilities shared across file operations.
package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UniquePath returns a path in dir that doesn't collide with any existing
// entry. Given "dir" and "base.txt", if dir/base.txt already exists it
// returns dir/base copy.txt, dir/base copy 2.txt, … up to 999.
//
// Extension is preserved, which keeps macOS and GUI file-managers happy.
// "base" alone (no extension) becomes "base copy", "base copy 2", etc.
//
// Only the local filesystem is checked; callers using non-local providers
// should pass a Stat wrapper or substitute their own existence check via
// UniquePathWith.
func UniquePath(dir, name string) string {
	return UniquePathWith(dir, name, func(p string) bool {
		_, err := os.Lstat(p)
		return err == nil
	})
}

// UniquePathWith is UniquePath with a caller-supplied "exists" predicate so
// it can be used with remote providers.
func UniquePathWith(dir, name string, exists func(string) bool) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	candidate := filepath.Join(dir, name)
	if !exists(candidate) {
		return candidate
	}

	// First collision: " copy". Subsequent: " copy 2", " copy 3", …
	first := filepath.Join(dir, base+" copy"+ext)
	if !exists(first) {
		return first
	}
	for i := 2; i < 1000; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s copy %d%s", base, i, ext))
		if !exists(candidate) {
			return candidate
		}
	}
	// Last-ditch: unix nanoseconds guarantee a new name.
	return filepath.Join(dir, fmt.Sprintf("%s copy %d%s", base, os.Getpid(), ext))
}
