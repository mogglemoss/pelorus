package ops

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mogglemoss/pelorus/internal/theme"
)

// CloseQueueMsg is sent when the user dismisses the job queue overlay.
type CloseQueueMsg struct{}

// QueueModel is the Bubbletea overlay model for the job queue.
type QueueModel struct {
	Width  int
	Height int
	Theme  *theme.Theme
	Queue  *Queue
	cursor int
	bars   map[int]progress.Model // keyed by job ID
}

// NewQueueModel creates a new QueueModel.
func NewQueueModel(q *Queue, t *theme.Theme) *QueueModel {
	return &QueueModel{
		Queue: q,
		Theme: t,
		bars:  make(map[int]progress.Model),
	}
}

// Update handles key events for the queue overlay.
func (m *QueueModel) Update(msg tea.Msg) (*QueueModel, tea.Cmd) {
	switch msg := msg.(type) {
	case progress.FrameMsg:
		var cmds []tea.Cmd
		for id, bar := range m.bars {
			newModel, cmd := bar.Update(msg)
			if pb, ok := newModel.(progress.Model); ok {
				m.bars[id] = pb
			}
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		jobs := m.Queue.Jobs()

		switch msg.String() {
		case "j", "down":
			if m.cursor < len(jobs)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "p":
			if m.cursor < len(jobs) {
				j := jobs[m.cursor]
				if j.Status == StatusRunning {
					m.Queue.PauseJob(j.ID)
				} else if j.Status == StatusPaused {
					m.Queue.ResumeJob(j.ID)
				}
			}
		case "x":
			if m.cursor < len(jobs) {
				m.Queue.CancelJob(jobs[m.cursor].ID)
			}
		case "c":
			m.Queue.ClearDone()
			// Re-clamp cursor.
			newJobs := m.Queue.Jobs()
			if m.cursor >= len(newJobs) && len(newJobs) > 0 {
				m.cursor = len(newJobs) - 1
			}
		case "esc", "q", "J":
			return m, func() tea.Msg { return CloseQueueMsg{} }
		}
	}

	return m, nil
}

// View renders the job queue overlay.
func (m *QueueModel) View() string {
	jobs := m.Queue.Jobs()

	// Build box content.
	var sb strings.Builder
	title := " [ Jobs ] "
	sb.WriteString(title)
	sb.WriteString("\n")

	if len(jobs) == 0 {
		sb.WriteString("  No jobs.\n")
	}

	boxW := m.Width * 3 / 4
	if boxW < 60 {
		boxW = 60
	}
	if boxW > m.Width-4 {
		boxW = m.Width - 4
	}

	const barWidth = 20

	for i, j := range jobs {
		icon := jobIcon(j.Status)
		srcBase := filepath.Base(j.Src)
		dst := j.Dst
		if dst != "" {
			dst = filepath.Dir(dst) + "/"
		}

		selected := i == m.cursor

		var line string
		switch j.Status {
		case StatusRunning, StatusPaused:
			pct := int(j.Progress * 100)

			// Get or create a progress bar for this job.
			bar, ok := m.bars[j.ID]
			if !ok {
				bar = progress.New(
					progress.WithScaledGradient("#004d4d", "#00ffd0"),
					progress.WithoutPercentage(),
					progress.WithWidth(barWidth),
				)
				m.bars[j.ID] = bar
			}
			progressStr := bar.ViewAs(j.Progress)

			var extra string
			if j.Speed > 0 {
				extra = fmt.Sprintf("  %s  ETA %s", humanSpeed(j.Speed), humanETA(j.ETA))
			}
			if j.Status == StatusPaused {
				extra = "  paused"
			}
			line = fmt.Sprintf("  %s  %-6s  %s → %s  %3d%%  %s%s",
				icon, string(j.Kind), srcBase, dst, pct, progressStr, extra)
		case StatusDone:
			// Clean up bar for done jobs.
			delete(m.bars, j.ID)
			line = fmt.Sprintf("  %s  %-6s  %s → %s  done",
				icon, string(j.Kind), srcBase, dst)
		case StatusError:
			delete(m.bars, j.ID)
			errStr := "unknown error"
			if j.Err != nil {
				errStr = j.Err.Error()
			}
			line = fmt.Sprintf("  %s  %-6s  %s → %s  %s",
				icon, string(j.Kind), srcBase, dst, errStr)
		default: // pending
			line = fmt.Sprintf("  %s  %-6s  %s → %s  pending",
				icon, string(j.Kind), srcBase, dst)
		}

		// Truncate to avoid wrapping.
		maxW := m.Width - 8
		if maxW < 20 {
			maxW = 20
		}
		if lipgloss.Width(line) > maxW {
			runes := []rune(line)
			if maxW > 1 {
				line = string(runes[:maxW-1]) + "…"
			} else {
				line = "…"
			}
		}

		if selected {
			line = m.Theme.PaletteSelected.Render(line)
		} else {
			line = m.Theme.PaletteItem.Render(line)
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(m.Theme.PaletteItem.Render("  p=pause/resume  x=cancel  c=clear done  esc=close"))

	content := sb.String()

	return m.Theme.PaletteBox.Width(boxW).Render(content)
}

func jobIcon(s JobStatus) string {
	switch s {
	case StatusDone:
		return "✓"
	case StatusRunning:
		return "↑"
	case StatusError:
		return "✗"
	case StatusPaused:
		return "⏸"
	default:
		return "·"
	}
}

func humanSpeed(bytesPerSec float64) string {
	switch {
	case bytesPerSec >= 1_000_000_000:
		return fmt.Sprintf("%.1f GB/s", bytesPerSec/1_000_000_000)
	case bytesPerSec >= 1_000_000:
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/1_000_000)
	case bytesPerSec >= 1_000:
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1_000)
	default:
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	}
}

func humanETA(d time.Duration) string {
	secs := int(d.Seconds())
	if secs < 0 {
		secs = 0
	}
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	mins := secs / 60
	secs = secs % 60
	return fmt.Sprintf("%dm%ds", mins, secs)
}
