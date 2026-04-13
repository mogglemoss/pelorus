// Package mascot provides the animated robot-head mascot for the Pelorus
// header bar, ported from the HARUSPEX anglerfish design.
//
// The mascot is a 3-row × 11-column ASCII art robot head. When active (file
// ops, preview loading, remote connect) the antenna tip flares and the eye
// scans left/right. At rest the eye is centred and the antenna is dim.
package mascot

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TickMsg is sent by the Tick command on each animation step.
type TickMsg struct{}

// Tick returns a Bubbletea command that fires TickMsg after 550 ms.
func Tick() tea.Cmd {
	return tea.Tick(550*time.Millisecond, func(_ time.Time) tea.Msg {
		return TickMsg{}
	})
}

// frameData describes one animation frame.
type frameData struct {
	antennaTip   string // ·  •  ●
	antennaColor string // hex
	eyeLeft      int    // spaces to the left of ◉ inside the face (0–6)
}

// frames is the 6-frame active animation cycle.
// Eye scans right (frames 0→2) then left (frames 3→5); antenna flares at peak.
//
//	frame  antenna  eyeLeft  description
//	  0      ·   3     5      center, dim      (start)
//	  1      ·   #C1   4      right, building
//	  2      •   #e8   5      far right, flare
//	  3      ●   #FF   3      center, peak
//	  4      •   #e8   2      left, settling
//	  5      ·   #C1   1      far left, dim
var frames = [6]frameData{
	{"·", "#8A3820", 3}, // 0 rest / center
	{"·", "#C15F3C", 4}, // 1 right
	{"•", "#e8a559", 5}, // 2 far right + flare
	{"●", "#f5c842", 3}, // 3 center + peak
	{"•", "#e8a559", 2}, // 4 left + settling
	{"·", "#C15F3C", 1}, // 5 far left
}

// restFrame is shown when the mascot is idle (no active tasks).
var restFrame = frameData{"·", "#8A3820", 3}

// View renders the mascot as a 3-line string.
//
// active controls whether the animation plays; when false the rest pose is
// always used regardless of frame. headerBg is the hex background color so
// the art blends into the header bar.
//
// Layout (11 cols wide, 3 rows):
//
//	     ·      ← antenna tip (antennaColor, bold) — col 5
//	 ╭───●───╮  ← head outline + glow node (rust)
//	(│   ◉   │) ← face; eye shifts left/right when active
func View(frame int, active bool, headerBg string) string {
	f := restFrame
	if active {
		f = frames[frame%6]
	}

	bg := lipgloss.Color(headerBg)
	escaStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color(f.antennaColor)).Bold(true)
	frameStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#7a756e"))
	eyeStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#e8e6e3"))
	bgStyle := lipgloss.NewStyle().Background(bg)

	// Row 1: antenna tip at col 5, centred over ● in row 2
	row1 := bgStyle.Render("     ") + escaStyle.Render(f.antennaTip) + bgStyle.Render("     ")

	// Row 2: head outline with glow node
	row2 := frameStyle.Render(" ╭───") + escaStyle.Render("●") + frameStyle.Render("───╮ ")

	// Row 3: face — eyeLeft spaces + eye + remaining spaces inside │ │
	//   interior = 7 chars (eyeLeft + 1 + eyeRight = 7)
	eyeLeft := f.eyeLeft
	if eyeLeft < 0 {
		eyeLeft = 0
	}
	eyeRight := 6 - eyeLeft
	leftPad := strings.Repeat(" ", eyeLeft)
	rightPad := strings.Repeat(" ", eyeRight)
	row3 := frameStyle.Render("(│") + bgStyle.Render(leftPad) + eyeStyle.Render("◉") + bgStyle.Render(rightPad) + frameStyle.Render("│)")

	return row1 + "\n" + row2 + "\n" + row3
}

// Width returns the fixed character width of the mascot view.
func Width() int { return 11 }

// Height returns the fixed line height of the mascot view.
func Height() int { return 3 }
