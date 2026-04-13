package app

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/atotto/clipboard"

	"github.com/mogglemoss/pelorus/internal/actions"
	"github.com/mogglemoss/pelorus/internal/gitstatus"
	"github.com/mogglemoss/pelorus/internal/config"
	"github.com/mogglemoss/pelorus/internal/connect"
	"github.com/mogglemoss/pelorus/internal/help"
	"github.com/mogglemoss/pelorus/internal/jump"
	"github.com/mogglemoss/pelorus/internal/ops"
	"github.com/mogglemoss/pelorus/internal/palette"
	"github.com/mogglemoss/pelorus/internal/pane"
	"github.com/mogglemoss/pelorus/internal/preview"
	"github.com/mogglemoss/pelorus/internal/provider"
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

	// multiDeletePending holds items awaiting confirmation for batch delete.
	multiDeletePending []fileinfo.FileInfo

	store *jump.Store

	registry *actions.Registry
	keyMap   map[string]string // key -> actionID, built from registry after all registrations
	cfg      *config.Config
	theme    *theme.Theme

	width  int
	height int

	statusMsg string // transient message shown in status bar
	gitBranch string // current git branch for active pane's repo
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
		return m, tea.Batch(m.updatePreview(), m.updateGitStatus())

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

	// --- Git status messages ---
	case gitstatus.GitStatusMsg:
		m.gitBranch = msg.Branch
		for _, p := range m.panes {
			p.GitStatus = msg.Status
		}
		return m, nil

	// --- Preview messages ---
	case preview.ContentReadyMsg:
		m.previewModel.SetContent(msg)
		return m, nil

	case pane.OpenFileMsg:
		return m, m.openInEditor(msg.Path)

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

	case actions.ToggleSelectMsg:
		ap := m.activeP()
		ap.ToggleSelection()
		return m, m.updatePreview()

	case actions.OpenEditorMsg:
		sel := m.activeP().Selected()
		if sel != nil && !sel.IsDir {
			return m, m.openInEditor(sel.Path)
		}
		return m, nil

	case actions.TrashMsg:
		return m, m.enqueueTrash()

	case actions.CycleSortMsg:
		m.activeP().CycleSort()
		return m, m.updatePreview()

	case actions.CopyPathMsg:
		if sel := m.activeP().Selected(); sel != nil {
			if err := clipboard.WriteAll(sel.Path); err != nil {
				m.statusMsg = "Clipboard unavailable"
			} else {
				m.statusMsg = "Copied: " + sel.Path
			}
		}
		return m, nil

	case actions.CopyFilenameMsg:
		if sel := m.activeP().Selected(); sel != nil {
			if err := clipboard.WriteAll(sel.Name); err != nil {
				m.statusMsg = "Clipboard unavailable"
			} else {
				m.statusMsg = "Copied: " + sel.Name
			}
		}
		return m, nil

	case actions.DeleteSelectedMsg:
		ap := m.activeP()
		entries := ap.SelectedEntries()
		if len(entries) > 1 {
			// Batch delete: show confirmation at app level.
			m.multiDeletePending = entries
			m.statusMsg = fmt.Sprintf("Delete %d items? (y/n)", len(entries))
			return m, nil
		}
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

	// Multi-delete confirmation prompt.
	if len(m.multiDeletePending) > 0 {
		switch key {
		case "y", "Y", "enter":
			entries := m.multiDeletePending
			m.multiDeletePending = nil
			m.activeP().ClearSelection()
			var cmds []tea.Cmd
			for _, fi := range entries {
				job := m.queue.Add(ops.KindDelete, fi.Path, "")
				job.Status = ops.StatusRunning
				job.StartTime = time.Now()
				cmds = append(cmds, ops.StartJob(job, m.activeP().Provider, m.activeP().Provider))
			}
			m.statusMsg = fmt.Sprintf("Deleting %d items…", len(entries))
			cmds = append(cmds, m.startProgressTicker())
			return m, tea.Batch(cmds...)
		default:
			m.multiDeletePending = nil
			m.statusMsg = "Cancelled"
			return m, nil
		}
	}

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
		enterCmd := ap.EnterSelected() // capture cmd (e.g. OpenFileMsg for regular files)
		if ap.Path != prevPath {
			m.store.Visit(ap.Path)
			_ = m.store.Save()
		}
		previewCmd := m.updatePreview()
		gitCmd := m.updateGitStatus()
		if enterCmd != nil {
			return tea.Batch(enterCmd, previewCmd, gitCmd)
		}
		return tea.Batch(previewCmd, gitCmd)
	case "parent":
		prevPath := ap.Path
		ap.GoParent()
		if ap.Path != prevPath {
			m.store.Visit(ap.Path)
			_ = m.store.Save()
		}
	}
	return tea.Batch(m.updatePreview(), m.updateGitStatus())
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

// updateGitStatus triggers async git status fetch for the active pane's directory.
func (m *Model) updateGitStatus() tea.Cmd {
	return gitstatus.GitStatusCmd(m.activeP().Path)
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
	entries := ap.SelectedEntries()
	if len(entries) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, sel := range entries {
		dst := filepath.Join(m.inactiveP().Path, sel.Name)
		job := m.queue.Add(ops.KindCopy, sel.Path, dst)
		job.Status = ops.StatusRunning
		job.StartTime = time.Now()
		cmds = append(cmds, ops.StartJob(job, ap.Provider, m.inactiveP().Provider))
	}
	if len(entries) == 1 {
		m.statusMsg = fmt.Sprintf("Copying %q…", entries[0].Name)
	} else {
		m.statusMsg = fmt.Sprintf("Copying %d items…", len(entries))
	}
	ap.ClearSelection()
	cmds = append(cmds, m.startProgressTicker())
	return tea.Batch(cmds...)
}

func (m *Model) enqueueMove() tea.Cmd {
	ap := m.activeP()
	entries := ap.SelectedEntries()
	if len(entries) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, sel := range entries {
		dst := filepath.Join(m.inactiveP().Path, sel.Name)
		job := m.queue.Add(ops.KindMove, sel.Path, dst)
		job.Status = ops.StatusRunning
		job.StartTime = time.Now()
		cmds = append(cmds, ops.StartJob(job, ap.Provider, m.inactiveP().Provider))
	}
	if len(entries) == 1 {
		m.statusMsg = fmt.Sprintf("Moving %q…", entries[0].Name)
	} else {
		m.statusMsg = fmt.Sprintf("Moving %d items…", len(entries))
	}
	ap.ClearSelection()
	cmds = append(cmds, m.startProgressTicker())
	return tea.Batch(cmds...)
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

func (m *Model) enqueueTrash() tea.Cmd {
	ap := m.activeP()
	if !ap.Provider.Capabilities().CanTrash {
		m.statusMsg = "Trash not supported on this provider"
		return nil
	}
	entries := ap.SelectedEntries()
	if len(entries) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, sel := range entries {
		job := m.queue.Add(ops.KindTrash, sel.Path, "")
		job.Status = ops.StatusRunning
		job.StartTime = time.Now()
		cmds = append(cmds, ops.StartJob(job, ap.Provider, ap.Provider))
	}
	if len(entries) == 1 {
		m.statusMsg = fmt.Sprintf("Trashing %q…", entries[0].Name)
	} else {
		m.statusMsg = fmt.Sprintf("Trashing %d items…", len(entries))
	}
	ap.ClearSelection()
	cmds = append(cmds, m.startProgressTicker())
	return tea.Batch(cmds...)
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
		// Use form-supplied username/port if non-zero, fall back to SSH config values.
		username := msg.Username
		if username == "" {
			username = h.User
		}
		port := msg.Port
		if port == 0 {
			port = h.Port
		}
		password := msg.Password
		return func() tea.Msg {
			prov, err := sftpprov.Connect(h.HostName, port, username, h.IdentityFiles, password)
			if err != nil {
				return connectErrMsg{err: err}
			}
			return connectedMsg{alias: h.Alias, prov: prov}
		}
	}
	if msg.Node != nil {
		node := *msg.Node
		username := msg.Username
		if username == "" {
			if u, err := user.Current(); err == nil && u != nil {
				username = u.Username
			}
		}
		port := msg.Port
		if port == 0 {
			port = 22
		}
		password := msg.Password
		identFiles := defaultIdentityFiles()
		return func() tea.Msg {
			prov, err := sftpprov.Connect(node.DNS, port, username, identFiles, password)
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


// ActivePath returns the path of the active pane. Used by the caller to save
// the last-used directory to the session file on quit.
func (m *Model) ActivePath() string {
	return m.activeP().Path
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

// openInEditor opens the given path in the configured editor.
// Uses cfg.General.Editor, falling back to $EDITOR then $VISUAL then vi.
func (m *Model) openInEditor(path string) tea.Cmd {
	editor := m.cfg.General.Editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	return tea.ExecProcess(exec.Command(editor, path), func(err error) tea.Msg {
		if err != nil {
			return pane.ErrMsg{Err: fmt.Errorf("editor: %w", err)}
		}
		return nil
	})
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

	// Build divider with exactly paneH rows (no trailing newline so JoinHorizontal
	// sees the same row count as the pane views and does not add a phantom extra row).
	divLines := strings.Repeat("│\n", m.panes[0].Height)
	divLines = divLines[:len(divLines)-1] // strip trailing \n → exactly paneH rows
	divider := m.theme.Divider.Render(divLines)

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
// Single lipgloss.Style applied to a plain fmt.Sprintf string — the most
// reliable approach for guaranteed background fill across all terminals.
func (m *Model) renderHeader() string {
	paneLabel := "⬡ left"
	if m.activePane == 1 {
		paneLabel = "⬡ right"
	}

	left := "  PELORUS"
	right := "   " + paneLabel + "   ctrl+p palette   g jump   c connect  "

	// Fit the active path into the center gap.
	path := m.activeP().Path
	used := len(left) + len(right) // ascii-safe: no emoji in left/right
	centerW := m.width - used
	if centerW < 2 {
		centerW = 2
	}
	runes := []rune(path)
	maxR := centerW - 2
	if maxR < 1 {
		maxR = 1
	}
	if len(runes) > maxR {
		path = "…" + string(runes[len(runes)-maxR:])
	}
	center := fmt.Sprintf("  %-*s", centerW-2, path)

	return lipgloss.NewStyle().
		Background(lipgloss.Color("#0e7c7b")).
		Foreground(lipgloss.Color("#caf0e4")).
		Bold(true).
		Width(m.width).
		Render(left + center + right)
}

// renderStatusBar builds the status bar string.
func (m *Model) renderStatusBar() string {
	ap := m.activeP()

	// If a transient message is set, show it full-width.
	if m.statusMsg != "" {
		return m.theme.StatusBar.Width(m.width).Render(" " + m.statusMsg)
	}

	// --- Left: breadcrumb path ---
	home, _ := os.UserHomeDir()
	path := ap.Path
	if home != "" && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	// Build breadcrumb by replacing separators with › .
	breadcrumb := strings.ReplaceAll(path, string(filepath.Separator), " › ")
	if breadcrumb == "" {
		breadcrumb = "/"
	}
	// Archive suffix.
	if ap.HasArchive() {
		if label := ap.ArchiveLabel(); label != "" {
			breadcrumb += "  [" + label + "]"
		}
	}
	// Filter indicator.
	if ap.FilterStr != "" {
		breadcrumb += fmt.Sprintf("  /%s", ap.FilterStr)
	}
	// Multi-select count.
	selCount := len(ap.MultiSel)

	// --- Center: git branch ---
	center := ""
	if m.gitBranch != "" {
		center = "⎇ " + m.gitBranch
	}

	// --- Right: remote badge ---
	right := ""
	if sftpP, ok := ap.Provider.(*sftpprov.Provider); ok {
		right = "● " + sftpP.String()
	}

	// --- Far-right: perms + size + sel count ---
	farRight := ""
	if sel := ap.Selected(); sel != nil {
		farRight = sel.Mode.String() + "  " + fileinfo.HumanSize(sel.Size)
	}
	if selCount > 0 {
		farRight = fmt.Sprintf("%d selected  ", selCount) + farRight
	}

	// Layout arithmetic.
	centerW := lipgloss.Width(center)
	rightW := lipgloss.Width(right)
	farRightW := lipgloss.Width(farRight)

	// Add padding around center and right.
	if centerW > 0 {
		centerW += 4 // 2 each side
	}
	if rightW > 0 {
		rightW += 4
	}
	if farRightW > 0 {
		farRightW += 2
	}

	leftW := m.width - centerW - rightW - farRightW
	if leftW < 4 {
		leftW = 4
	}

	bg := m.theme.StatusBar.GetBackground()
	base := lipgloss.NewStyle().Background(bg)
	accentStyle := m.theme.StatusBarAccent
	mutedStyle := m.theme.StatusBarMuted

	// Left: accent color breadcrumb.
	leftContent := " " + breadcrumb
	leftSeg := accentStyle.Width(leftW).Render(leftContent)

	// Center: muted branch indicator.
	var centerSeg string
	if centerW > 0 {
		centerSeg = mutedStyle.Width(centerW).Align(lipgloss.Center).Render(center)
	} else {
		centerSeg = base.Width(centerW).Render("")
	}

	// Right: remote badge in accent.
	var rightSeg string
	if rightW > 0 {
		rightSeg = accentStyle.Width(rightW).Align(lipgloss.Right).Render(right + "  ")
	} else {
		rightSeg = base.Width(rightW).Render("")
	}

	// Far-right: muted perms+size.
	var farRightSeg string
	if farRightW > 0 {
		farRightSeg = mutedStyle.Width(farRightW).Align(lipgloss.Right).Render(farRight + " ")
	} else {
		farRightSeg = base.Width(farRightW).Render("")
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, leftSeg, centerSeg, rightSeg, farRightSeg)
}

// overlayCenter centers an overlay on top of the base view using lipgloss.Place.
// The surrounding whitespace is tinted to create a "dimmed backdrop" effect —
// the canonical Charm/lipgloss approach for TUI overlays.
func overlayCenter(base, overlay string, totalW, totalH int) string {
	return lipgloss.Place(
		totalW, totalH,
		lipgloss.Center, lipgloss.Center,
		overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.AdaptiveColor{Light: "#dddddd", Dark: "#1a2030"}),
	)
}
