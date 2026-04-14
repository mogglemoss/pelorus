// "diff.go" renders `git diff` output for the preview pane when the active
// file has a modified or staged status. Shells out to git once, parses the
// unified-diff output, and applies lipgloss styling per line — additions in
// green, deletions in red, hunk headers in accent, file headers dimmed.
//
// Returns (string, false) if git is unavailable, the file isn't in a repo,
// or the diff is empty — the caller falls through to the normal preview in
// that case.
package preview

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// renderGitDiff runs `git diff HEAD -- path` (with fallback to un-staged diff)
// and returns styled output. The ok return is false when no useful diff can
// be produced and the caller should show the normal file preview instead.
func renderGitDiff(path string, width int) (string, bool) {
	dir := filepath.Dir(path)

	// First try HEAD..working-tree so we catch both staged and unstaged.
	// If that's empty (e.g. the file is only staged with identical working
	// copy), fall back to `git diff` (unstaged only).
	out, ok := runGitDiff(dir, path, "HEAD")
	if !ok || strings.TrimSpace(out) == "" {
		out, ok = runGitDiff(dir, path, "")
	}
	if !ok || strings.TrimSpace(out) == "" {
		return "", false
	}

	return styleDiff(out, width), true
}

// runGitDiff returns raw diff output. ref may be empty (unstaged) or a commit.
func runGitDiff(dir, path, ref string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []string{"-C", dir, "--no-pager", "diff", "--no-color", "-U3"}
	if ref != "" {
		args = append(args, ref)
	}
	args = append(args, "--", path)

	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// styleDiff applies per-line colors to unified diff output.
func styleDiff(raw string, _ int) string {
	add := lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))   // green
	del := lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))   // red
	hunk := lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")).Bold(true) // blue
	meta := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))  // dim

	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff "),
			strings.HasPrefix(line, "index "),
			strings.HasPrefix(line, "--- "),
			strings.HasPrefix(line, "+++ "),
			strings.HasPrefix(line, "new file mode"),
			strings.HasPrefix(line, "deleted file mode"),
			strings.HasPrefix(line, "similarity index"),
			strings.HasPrefix(line, "rename from"),
			strings.HasPrefix(line, "rename to"):
			out = append(out, meta.Render(line))
		case strings.HasPrefix(line, "@@"):
			out = append(out, hunk.Render(line))
		case strings.HasPrefix(line, "+"):
			out = append(out, add.Render(line))
		case strings.HasPrefix(line, "-"):
			out = append(out, del.Render(line))
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
