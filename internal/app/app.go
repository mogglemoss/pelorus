package app

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mogglemoss/pelorus/internal/actions"
	"github.com/mogglemoss/pelorus/internal/config"
	"github.com/mogglemoss/pelorus/internal/connect"
	"github.com/mogglemoss/pelorus/internal/help"
	"github.com/mogglemoss/pelorus/internal/jump"
	"github.com/mogglemoss/pelorus/internal/ops"
	"github.com/mogglemoss/pelorus/internal/palette"
	"github.com/mogglemoss/pelorus/internal/pane"
	"github.com/mogglemoss/pelorus/internal/preview"
	"github.com/mogglemoss/pelorus/internal/provider"
	"github.com/mogglemoss/pelorus/internal/provider/archive"
	sftpprov "github.com/mogglemoss/pelorus/internal/provider/sftp"
	"github.com/mogglemoss/pelorus/internal/theme"
	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

// tickMsg is sent by the progress ticker every 200ms.
type tickMsg struct{}

// Model is the root Bubbletea model for Pelorus.
type Model struct {
	panes      [2]*pane.Model
	activePane int

	paletteModel *palette.Model
	paletteOpen  bool

	jumpModel *jump.Model
	jumpOpen  bool

	connectModel  *connect.Model
	connectOpen   bool
	sftpProviders map[string]*sftpprov.Provider // keyed by host alias, for cleanup

	helpModel *help.Model
	helpOpen  bool

	previewModel *preview.Model
	showPreview  bool

	queueModel    *ops.QueueModel
	queue         *ops.Queue
	queueOpen     bool
	tickerRunning bool

	store *jump.Store

	registry *actions.Registry
	keyMap   map[string]string // key -> actionID, built from registry after all registrations
	cfg      *config.Config
	theme    *theme.Theme

	width  int
	height int

	statusMsg string // transient message shown in status bar
}

// New creates the root app model.
func New(
	leftPath, rightPath string,
	prov provider.Provider,
	reg *actions.Registry,
	cfg *config.Config,
	t *theme.Theme,
) *Model {
	m := &Model{
		registry:    reg,
		keyMap:      actions.BuildKeyMap(reg),
		cfg:         cfg,
		theme:       t,
		showPreview: cfg.Layout.ShowPreview,
	}

	m.panes[0] = pane.New(leftPath, prov, t, cfg.General.ShowHidden)
	m.panes[1] = pane.New(rightPath, prov, t, cfg.General.ShowHidden)
	m.panes[0].IsActive = true

	m.paletteModel = palette.New(reg, t)
	m.previewModel = preview.New(t)
	m.queue = ops.NewQueue()
	m.queueModel = ops.NewQueueModel(m.queue, t)
	m.connectModel, _ = connect.NewModel(t) // ignore error (empty hosts ok)
	m.sftpProviders = make(map[string]*sftpprov.Provider)
	m.helpModel = help.New(reg, t)

	// Load jump store (ignore error — use empty store on failure).
	store, _ := jump.LoadStore()
	if store == nil {
		store = jump.NewStore()
	}
	m.store = store
	m.jumpModel = jump.NewModel(store, t)

	return m
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layoutPanes()
		m.paletteModel.Width = msg.Width
		m.paletteModel.Height = msg.Height
		m.jumpModel.Width = msg.Width
		m.jumpModel.Height = msg.Height
		if m.connectModel != nil {
			m.connectModel.Width = msg.Width
			m.connectModel.Height = msg.Height
		}
		m.helpModel.Width = msg.Width
		m.helpModel.Height = msg.Height
		return m, nil

	// --- Error display ---
	case pane.ErrMsg:
		m.statusMsg = "Error: " + msg.Err.Error()
		return m, nil

	// --- Palette messages ---
	case palette.ClosePaletteMsg:
		m.paletteOpen = false
		return m, nil

	case palette.RunActionMsg:
		m.paletteOpen = false
		if a, ok := m.registry.ByID(msg.ActionID); ok {
			state := m.buildAppState()
			cmd := a.Handler(state)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	// --- Jump list messages ---
	case jump.JumpToMsg:
		ap := m.activeP()
		ap.Path = msg.Path
		ap.Reload()
		m.jumpOpen = false
		m.store.Visit(msg.Path)
		_ = m.store.Save()
		return m, m.updatePreview()

	case jump.CloseJumpMsg:
		m.jumpOpen = false
		return m, nil

	// --- Preview messages ---
	case preview.ContentReadyMsg:
		m.previewModel.SetContent(msg)
		return m, nil

	// --- Action messages ---
	case actions.NavMsg:
		cmd := m.handleNav(msg.Dir)
		return m, cmd

	case actions.SwitchPaneMsg:
		m.switchPane()
		return m, m.updatePreview()

	case actions.TogglePreviewMsg:
		m.showPreview = !m.showPreview
		m.layoutPanes()
		if m.showPreview {
			return m, m.updatePreview()
		}
		return m, nil

	case actions.ToggleHiddenMsg:
		for _, p := range m.panes {
			p.ToggleHidden()
		}
		return m, nil

	case actions.DeleteSelectedMsg:
		ap := m.activeP()
		cmd := ap.StartDelete(m.cfg.General.ConfirmDelete)
		return m, cmd

	case actions.RenameSelectedMsg:
		m.activeP().StartRename()
		return m, nil

	case actions.NewFileMsg:
		m.activeP().StartNewFile()
		return m, nil

	case actions.NewDirMsg:
		m.activeP().StartNewDir()
		return m, nil

	case actions.CopySelectedMsg:
		cmd := m.enqueueCopy()
		return m, cmd

	case actions.MoveSelectedMsg:
		cmd := m.enqueueMove()
		return m, cmd

	case actions.GoHomeMsg:
		m.activeP().GoHome()
		m.store.Visit(m.activeP().Path)
		_ = m.store.Save()
		return m, m.updatePreview()

	case actions.GotoPathMsg:
		m.activeP().StartGotoPath()
		return m, nil

	case actions.OpenPaletteMsg:
		m.paletteModel.Reset()
		m.paletteOpen = true
		return m, nil

	case actions.OpenJumpMsg:
		m.jumpOpen = true
		m.jumpModel.Open()
		return m, nil

	case actions.BookmarkMsg:
		ap := m.activeP()
		m.store.Pin(ap.Path, "")
		_ = m.store.Save()
		m.statusMsg = fmt.Sprintf("Bookmarked %q", ap.Path)
		return m, nil

	case actions.QuitMsg:
		m.closeAllSFTP()
		return m, tea.Quit

	case actions.OpenConnectMsg:
		if m.connectModel != nil {
			m.connectOpen = true
			m.connectModel.Width = m.width
			m.connectModel.Height = m.height
			cmd := m.connectModel.Open()
			return m, cmd
		}
		return m, nil

	case connect.CloseConnectMsg:
		m.connectOpen = false
		return m, nil

	// --- Help overlay messages ---
	case actions.OpenHelpMsg:
		m.helpOpen = true
		m.helpModel.Width = m.width
		m.helpModel.Height = m.height
		m.helpModel.Open()
		return m, nil

	case help.CloseHelpMsg:
		m.helpOpen = false
		return m, nil

	case connect.TsNodesMsg:
		if m.connectModel != nil {
			m.connectModel, _ = m.connectModel.Update(msg)
		}
		return m, nil

	case connect.ConnectMsg:
		m.connectOpen = false
		return m, m.connectToHost(msg)

	case connectErrMsg:
		m.statusMsg = "Connect failed: " + msg.err.Error()
		return m, nil

	case connectedMsg:
		ip := m.inactiveP()
		if old, ok := m.sftpProviders[msg.alias]; ok {
			old.Close()
		}
		m.sftpProviders[msg.alias] = msg.prov
		ip.Provider = msg.prov
		ip.Path = "/"
		ip.Reload()
		m.statusMsg = fmt.Sprintf("Connected to %s", msg.alias)
		return m, nil

	// --- Queue messages ---
	case actions.OpenQueueMsg:
		m.queueOpen = true
		m.queueModel.Width = m.width
		m.queueModel.Height = m.height
		return m, nil

	case ops.CloseQueueMsg:
		m.queueOpen = false
		return m, nil

	case tickMsg:
		var tickCmds []tea.Cmd
		running := m.queue.Running()
		for _, job := range running {
			tickCmds = append(tickCmds, ops.TickProgress(job))
		}
		if len(running) > 0 {
			tickCmds = append(tickCmds, m.startProgressTicker())
		} else {
			m.tickerRunning = false
			m.panes[0].Reload()
			m.panes[1].Reload()
		}
		return m, tea.Batch(tickCmds...)

	case ops.ProgressMsg:
		// Progress is mutated directly on the job struct; nothing to do here.
		return m, nil

	case ops.JobDoneMsg:
		if job, ok := m.queue.Get(msg.JobID); ok {
			if msg.Err != nil {
				job.Status = ops.StatusError
				job.Err = msg.Err
				m.statusMsg = fmt.Sprintf("Job %d failed: %s", msg.JobID, msg.Err)
			} else {
				job.Status = ops.StatusDone
				job.Progress = 1.0
				m.statusMsg = fmt.Sprintf("Job %d done", msg.JobID)
			}
			m.panes[0].Reload()
			m.panes[1].Reload()
		}
		return m, nil

	case pane.DeleteConfirmedMsg:
		cmd := m.enqueueDelete(msg.Path, msg.Provider)
		return m, cmd

	// --- Spinner tick (preview pane loading) ---
	case spinner.TickMsg:
		if m.previewModel.IsLoading() {
			cmd := m.previewModel.UpdateSpinner(msg)
			return m, cmd
		}
		return m, nil

	// --- Progress bar frame (job queue) ---
	case progress.FrameMsg:
		var cmd tea.Cmd
		m.queueModel, cmd = m.queueModel.Update(msg)
		return m, cmd

	// --- Key events ---
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, tea.Batch(cmds...)
}

// handleKey dispatches key events.
func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If help overlay is open, route all keys there first.
	if m.helpOpen {
		var cmd tea.Cmd
		m.helpModel, cmd = m.helpModel.Update(msg)
		return m, cmd
	}

	// If queue overlay is open, send to queue model.
	if m.queueOpen {
		var cmd tea.Cmd
		m.queueModel, cmd = m.queueModel.Update(msg)
		return m, cmd
	}

	// If connect palette is open, send to connect model.
	if m.connectOpen && m.connectModel != nil {
		var cmd tea.Cmd
		m.connectModel, cmd = m.connectModel.Update(msg)
		return m, cmd
	}

	// If jump list is open, send to jump model.
	if m.jumpOpen {
		var cmd tea.Cmd
		m.jumpModel, cmd = m.jumpModel.Update(msg)
		return m, cmd
	}

	// If palette is open, send to palette.
	if m.paletteOpen {
		var cmd tea.Cmd
		m.paletteModel, cmd = m.paletteModel.Update(msg)
		return m, cmd
	}

	// If active pane is in a special mode, let it handle the key.
	ap := m.activeP()
	if ap.Mode != pane.ModeNormal && ap.Mode != pane.ModeFilter {
		updated, cmd := ap.Update(msg)
		m.panes[m.activePane] = updated
		return m, cmd
	}

	// Handle filter mode in pane.
	if ap.Mode == pane.ModeFilter {
		updated, cmd := ap.Update(msg)
		m.panes[m.activePane] = updated
		return m, cmd
	}

	key := msg.String()

	// Preview scroll keys — handled before action dispatch.
	if m.showPreview {
		switch key {
		case "]":
			m.previewModel.ScrollDown(3)
			return m, nil
		case "[":
			m.previewModel.ScrollUp(3)
			return m, nil
		case "}":
			m.previewModel.HalfPageDown()
			return m, nil
		case "{":
			m.previewModel.HalfPageUp()
			return m, nil
		}
	}

	// Check for printable chars that might start a filter (not bound to actions).
	// We first check if the key maps to an action.
	if actionID, ok := m.keyToAction(key); ok {
		if a, ok := m.registry.ByID(actionID); ok {
			state := m.buildAppState()
			cmd := a.Handler(state)
			return m, cmd
		}
	}

	// Colon also opens palette.
	if key == ":" {
		m.paletteModel.Reset()
		m.paletteOpen = true
		return m, nil
	}

	// Printable single chars that are not bound -> start fuzzy filter on active pane.
	if len(key) == 1 && key >= " " && key <= "~" {
		ap.StartFilter()
		ap.FilterStr = key
		ap.ApplyFilterPublic()
		return m, nil
	}

	return m, nil
}

// keyToAction maps a key string to an action ID using the registry-driven keyMap.
// The keyMap is built once in New() from all registered actions (builtins + custom),
// with any config keybinding overrides already applied via ApplyKeybindings.
// Arrow keys and enter are aliased here for terminal compatibility.
func (m *Model) keyToAction(key string) (string, bool) {
	// Terminal arrow-key aliases: map to the same logical keys used in builtin registrations.
	switch key {
	case "down":
		key = "j"
	case "up":
		key = "k"
	case "left":
		key = "h"
	case "right":
		key = "l"
	case "enter":
		key = "l"
	}

	id, ok := m.keyMap[key]
	return id, ok
}

func (m *Model) handleNav(dir string) tea.Cmd {
	ap := m.activeP()
	switch dir {
	case "up":
		ap.CursorUp()
	case "down":
		ap.CursorDown()
	case "enter":
		prevPath := ap.Path
		ap.EnterSelected()
		if ap.Path != prevPath {
			m.store.Visit(ap.Path)
			_ = m.store.Save()
		}
	case "parent":
		prevPath := ap.Path
		ap.GoParent()
		if ap.Path != prevPath {
			m.store.Visit(ap.Path)
			_ = m.store.Save()
		}
	}
	return m.updatePreview()
}

// updatePreview triggers async content loading for the currently selected file.
func (m *Model) updatePreview() tea.Cmd {
	if !m.showPreview {
		return nil
	}
	sel := m.activeP().Selected()
	if sel == nil {
		return nil
	}
	return m.previewModel.SetFile(sel)
}

func (m *Model) switchPane() {
	m.panes[m.activePane].IsActive = false
	m.activePane = 1 - m.activePane
	m.panes[m.activePane].IsActive = true
}

func (m *Model) activeP() *pane.Model {
	return m.panes[m.activePane]
}

func (m *Model) inactiveP() *pane.Model {
	return m.panes[1-m.activePane]
}

func (m *Model) buildAppState() actions.AppState {
	ap := m.activeP()
	var sel *fileinfo.FileInfo
	if s := ap.Selected(); s != nil {
		cp := *s
		sel = &cp
	}
	return actions.AppState{
		ActivePane:   m.activePane,
		Selected:     sel,
		ActivePath:   ap.Path,
		InactivePath: m.inactiveP().Path,
		ShowHidden:   ap.ShowHidden,
	}
}

func (m *Model) enqueueCopy() tea.Cmd {
	ap := m.activeP()
	sel := ap.Selected()
	if sel == nil {
		return nil
	}
	dst := filepath.Join(m.inactiveP().Path, sel.Name)
	job := m.queue.Add(ops.KindCopy, sel.Path, dst)
	job.Status = ops.StatusRunning
	job.StartTime = time.Now()
	m.statusMsg = fmt.Sprintf("Copying %q…", sel.Name)
	return tea.Batch(
		ops.StartJob(job, ap.Provider, m.inactiveP().Provider),
		m.startProgressTicker(),
	)
}

func (m *Model) enqueueMove() tea.Cmd {
	ap := m.activeP()
	sel := ap.Selected()
	if sel == nil {
		return nil
	}
	dst := filepath.Join(m.inactiveP().Path, sel.Name)
	job := m.queue.Add(ops.KindMove, sel.Path, dst)
	job.Status = ops.StatusRunning
	job.StartTime = time.Now()
	m.statusMsg = fmt.Sprintf("Moving %q…", sel.Name)
	return tea.Batch(
		ops.StartJob(job, ap.Provider, m.inactiveP().Provider),
		m.startProgressTicker(),
	)
}

func (m *Model) enqueueDelete(path string, prov provider.Provider) tea.Cmd {
	job := m.queue.Add(ops.KindDelete, path, "")
	job.Status = ops.StatusRunning
	job.StartTime = time.Now()
	m.statusMsg = fmt.Sprintf("Deleting %q…", filepath.Base(path))
	return tea.Batch(
		ops.StartJob(job, prov, prov),
		m.startProgressTicker(),
	)
}

func (m *Model) startProgressTicker() tea.Cmd {
	if m.tickerRunning {
		return nil
	}
	m.tickerRunning = true
	return tea.Tick(200*time.Millisecond, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

// --- SFTP connect helpers ---

type connectErrMsg struct{ err error }
type connectedMsg struct {
	alias string
	prov  *sftpprov.Provider
}

func (m *Model) connectToHost(msg connect.ConnectMsg) tea.Cmd {
	if msg.Host != nil {
		h := *msg.Host
		return func() tea.Msg {
			prov, err := sftpprov.Connect(h.HostName, h.Port, h.User, h.IdentityFiles)
			if err != nil {
				return connectErrMsg{err: err}
			}
			return connectedMsg{alias: h.Alias, prov: prov}
		}
	}
	if msg.Node != nil {
		node := *msg.Node
		return func() tea.Msg {
			username := ""
			if u, err := user.Current(); err == nil && u != nil {
				username = u.Username
			}
			identFiles := defaultIdentityFiles()
			prov, err := sftpprov.Connect(node.DNS, 22, username, identFiles)
			if err != nil {
				return connectErrMsg{err: err}
			}
			return connectedMsg{alias: node.Name, prov: prov}
		}
	}
	return nil
}

// defaultIdentityFiles returns common SSH identity file paths that exist on disk.
func defaultIdentityFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	candidates := []string{
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
	}
	var found []string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		}
	}
	return found
}


// closeAllSFTP closes all open SFTP provider connections.
func (m *Model) closeAllSFTP() {
	for _, prov := range m.sftpProviders {
		prov.Close()
	}
}

// Close shuts down all open SFTP connections. Call before quitting.
func (m *Model) Close() {
	m.closeAllSFTP()
}

// layoutPanes sets width/height on panes (and preview) based on terminal size.
func (m *Model) layoutPanes() {
	if m.width < 10 || m.height < 6 {
		return
	}
	// Reserve 1 row for header + 1 row for status bar.
	paneH := m.height - 2

	if m.showPreview {
		// Three-pane layout: left 30%, right 30%, preview 40% (configurable).
		// Two dividers between the three panes.
		usable := m.width - 2

		previewPct := m.cfg.Layout.PreviewWidth
		if previewPct <= 0 || previewPct >= 100 {
			previewPct = 40
		}

		previewW := usable * previewPct / 100
		remaining := usable - previewW
		leftW := remaining / 2
		rightW := remaining - leftW

		m.panes[0].Width = leftW
		m.panes[0].Height = paneH
		m.panes[1].Width = rightW
		m.panes[1].Height = paneH
		m.previewModel.Width = previewW
		m.previewModel.Height = paneH
		// Sync viewport inside preview (border=2, header+sep=2).
		m.previewModel.SetViewportSize(previewW-2, paneH-4)
	} else {
		// Two-pane layout: 50/50 with one divider.
		halfW := (m.width - 1) / 2
		m.panes[0].Width = halfW
		m.panes[0].Height = paneH
		m.panes[1].Width = m.width - halfW - 1
		m.panes[1].Height = paneH
	}
}

// View implements tea.Model.
func (m *Model) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	// Full-screen help overlay takes priority.
	if m.helpOpen {
		return m.helpModel.View()
	}

	// Header bar.
	header := m.renderHeader()

	leftView := m.panes[0].View()
	rightView := m.panes[1].View()

	divider := m.theme.Divider.Render(strings.Repeat("│\n", m.panes[0].Height))

	var panesRow string
	if m.showPreview {
		previewView := m.previewModel.View()
		panesRow = lipgloss.JoinHorizontal(lipgloss.Top, leftView, divider, rightView, divider, previewView)
	} else {
		// Join panes side by side.
		panesRow = lipgloss.JoinHorizontal(lipgloss.Top, leftView, divider, rightView)
	}

	// Status bar.
	statusBar := m.renderStatusBar()

	screen := lipgloss.JoinVertical(lipgloss.Left, header, panesRow, statusBar)

	// Overlay palette if open.
	if m.paletteOpen {
		paletteView := m.paletteModel.View()
		screen = overlayCenter(screen, paletteView, m.width, m.height)
	}

	// Overlay jump list if open.
	if m.jumpOpen {
		jumpView := m.jumpModel.View()
		screen = overlayCenter(screen, jumpView, m.width, m.height)
	}

	// Overlay job queue if open.
	if m.queueOpen {
		queueView := m.queueModel.View()
		screen = overlayCenter(screen, queueView, m.width, m.height)
	}

	// Overlay connect palette if open.
	if m.connectOpen && m.connectModel != nil {
		connectView := m.connectModel.View()
		screen = overlayCenter(screen, connectView, m.width, m.height)
	}

	return screen
}

// renderHeader builds the branded header bar (1 row).
func (m *Model) renderHeader() string {
	t := m.theme

	// Left: branded title.
	title := t.HeaderTitle.Render("  PELORUS")

	// Center: active pane path, dimly styled.
	ap := m.activeP()
	centerPath := ap.Path
	// Truncate if too long.
	maxPathW := m.width - 40
	if maxPathW < 10 {
		maxPathW = 10
	}
	if len(centerPath) > maxPathW {
		centerPath = "…" + centerPath[len(centerPath)-maxPathW+1:]
	}
	// Right: active pane indicator + key hints.
	paneLabel := "⬡ left"
	if m.activePane == 1 {
		paneLabel = "⬡ right"
	}
	hint := t.HeaderHint.Render("  ctrl+p palette  g jump  c connect")
	right := t.HeaderTitle.Copy().Foreground(lipgloss.Color("#00a896")).Render(paneLabel) + hint

	// Compute widths for the three sections.
	titleW := lipgloss.Width(title)
	rightW := lipgloss.Width(right)
	centerW := m.width - titleW - rightW
	if centerW < 0 {
		centerW = 0
	}

	centerPadded := t.HeaderPath.Width(centerW).Render(centerPath)

	row := lipgloss.JoinHorizontal(lipgloss.Top, title, centerPadded, right)

	// Pad/trim to exact terminal width.
	return t.Header.Width(m.width).Render(row)
}

// renderStatusBar builds the status bar string.
func (m *Model) renderStatusBar() string {
	ap := m.activeP()
	path := ap.Path
	count := len(ap.Filtered)

	// Show a provider label when the active pane is using a special provider.
	var providerLabel string
	if archProv, ok := ap.Provider.(*archive.Provider); ok {
		providerLabel = "[" + archProv.KindLabel() + "] "
	} else if sftpP, ok := ap.Provider.(*sftpprov.Provider); ok {
		providerLabel = "[" + sftpP.String() + "] "
	}

	info := fmt.Sprintf(" %s%s | %d items", providerLabel, path, count)

	sel := ap.Selected()
	if sel != nil {
		info += fmt.Sprintf(" | %s | %s",
			sel.Mode.String(),
			fileinfo.HumanSize(sel.Size),
		)
	}

	if m.statusMsg != "" {
		info += " | " + m.statusMsg
	}

	if ap.FilterStr != "" {
		info += fmt.Sprintf(" | filter: %q", ap.FilterStr)
	}

	return m.theme.StatusBar.Width(m.width).Render(info)
}

// overlayCenter places an overlay string in the center of the base view.
// This is a simple approach: split base into lines, inject the overlay lines.
func overlayCenter(base, overlay string, totalW, _ int) string {
	overlayLines := strings.Split(overlay, "\n")
	baseLines := strings.Split(base, "\n")

	overlayH := len(overlayLines)
	overlayW := 0
	for _, l := range overlayLines {
		if lipgloss.Width(l) > overlayW {
			overlayW = lipgloss.Width(l)
		}
	}

	startRow := (len(baseLines) - overlayH) / 2
	if startRow < 0 {
		startRow = 0
	}
	startCol := (totalW - overlayW) / 2
	if startCol < 0 {
		startCol = 0
	}

	result := make([]string, len(baseLines))
	copy(result, baseLines)

	for i, ol := range overlayLines {
		row := startRow + i
		if row >= len(result) {
			break
		}
		baseLine := result[row]
		result[row] = overlayLine(baseLine, ol, startCol)
	}

	return strings.Join(result, "\n")
}

// overlayLine replaces characters in baseLine starting at col with the overlay line.
func overlayLine(base, overlay string, col int) string {
	// Work with runes for correct Unicode handling.
	baseRunes := []rune(base)
	overlayRunes := []rune(overlay)

	// Extend base if needed.
	for len(baseRunes) < col+len(overlayRunes) {
		baseRunes = append(baseRunes, ' ')
	}

	copy(baseRunes[col:], overlayRunes)
	return string(baseRunes)
}
