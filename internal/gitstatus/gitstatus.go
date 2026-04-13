package gitstatus

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// StatusMap maps absolute file paths to a single-character git status glyph.
type StatusMap = map[string]string

// GitStatusMsg is sent when an async git status fetch completes.
type GitStatusMsg struct {
	Dir    string
	Status StatusMap
	Branch string // current branch name, or short SHA if detached
	InRepo bool   // true when dir is inside a git repository
}

// cacheEntry holds a cached git status result.
type cacheEntry struct {
	status    StatusMap
	branch    string
	fetchedAt time.Time
}

var (
	cacheMu sync.Mutex
	cache   = map[string]cacheEntry{} // key: repo root
)

// FindRepoRoot walks upward from dir looking for a .git directory.
// Returns the directory containing .git and true if found.
func FindRepoRoot(dir string) (string, bool) {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// readBranch reads the current branch from .git/HEAD.
func readBranch(root string) string {
	data, err := os.ReadFile(filepath.Join(root, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(data))
	if strings.HasPrefix(s, "ref: refs/heads/") {
		return strings.TrimPrefix(s, "ref: refs/heads/")
	}
	// Detached HEAD: return short SHA.
	if len(s) >= 7 {
		return s[:7]
	}
	return s
}

// FetchStatus fetches git status for the repo containing dir.
// Results are cached for 5 seconds. Returns an empty map (not an error)
// when dir is not inside a git repo or git is not available.
func FetchStatus(dir string) (StatusMap, string) {
	root, ok := FindRepoRoot(dir)
	if !ok {
		return StatusMap{}, ""
	}

	cacheMu.Lock()
	if e, ok := cache[root]; ok && time.Since(e.fetchedAt) < 5*time.Second {
		cacheMu.Unlock()
		return e.status, e.branch
	}
	cacheMu.Unlock()

	branch := readBranch(root)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", root, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		// git not available, not a repo, or timed out — return empty gracefully.
		return StatusMap{}, branch
	}

	status := make(StatusMap)
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		xy := line[:2]
		// Skip rename lines (have " -> " in them).
		if strings.HasPrefix(strings.TrimSpace(xy), "R") {
			continue
		}
		name := strings.TrimSpace(line[3:])
		absPath := filepath.Join(root, filepath.FromSlash(name))

		var glyph string
		switch {
		case xy == "??":
			glyph = "?"
		case xy == "!!":
			glyph = "!"
		case strings.ContainsAny(xy[:1], "MADRCU"):
			glyph = string([]byte{xy[0]})
		case strings.ContainsAny(xy[1:2], "MADRCU"):
			glyph = string([]byte{xy[1]})
		default:
			glyph = "?"
		}
		// Normalise to M/A/D/? for display.
		switch glyph {
		case "M", "A", "D":
			// keep
		case "R", "C", "U":
			glyph = "M"
		default:
			glyph = "?"
		}
		status[absPath] = glyph
	}

	cacheMu.Lock()
	cache[root] = cacheEntry{status: status, branch: branch, fetchedAt: time.Now()}
	cacheMu.Unlock()

	return status, branch
}

// GitStatusCmd returns a tea.Cmd that fetches git status asynchronously.
func GitStatusCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		_, inRepo := FindRepoRoot(dir)
		status, branch := FetchStatus(dir)
		return GitStatusMsg{Dir: dir, Status: status, Branch: branch, InRepo: inRepo}
	}
}
