package app

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"

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
	"github.com/mogglemoss/pelorus/internal/search"
	"github.com/mogglemoss/pelorus/internal/theme"
	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

// tickMsg is sent by the progress ticker every 200ms.
type tickMsg struct{}

// watchEventMsg is sent when the filesystem watcher detects a change.
type watchEventMsg struct{ dir string }

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

	// Huh overlay
	huhOverlay    *huh.Form
	huhMode       string // "delete"
	huhConfirm    bool
	huhDeleteItems []fileinfo.FileInfo
	huhInputValue  string // rename / new-file / new-dir single-input overlays

	// Session marks: m<key> to set, '<key> to jump.
	marks       map[rune]string // key → path
	markPending bool            // true = waiting for mark key
	jumpPending bool            // true = waiting for jump key

	// Search overlay.
	searchModel *search.Model
	searchOpen  bool

	store *jump.Store

	watcher       *fsnotify.Watcher
	watchDebounce map[string]time.Time

	registry *actions.Registry
	keyMap   map[string]string // key -> actionID, built from registry after all registrations
	cfg      *config.Config
	theme    *theme.Theme

	width      int
	height     int
	splitRatio float64 // left-pane share of usable width (0.0–1.0); 0 = 50/50

	// Run-command prompt state.
	cmdPromptOpen bool
	cmdPromptBuf  string

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
	m.marks = make(map[rune]string)
	m.searchModel = search.New(t)

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
	var err error
	m.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		// Watcher unavailable — run without auto-refresh.
		return nil
	}
	m.updateWatchers()
	return waitForWatchEvent(m.watcher)
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
		m.searchModel.Width = msg.Width
		m.searchModel.Height = msg.Height
		return m, tea.Batch(m.updatePreview(), m.updateGitStatus())
	}

	// Route all messages to Huh overlay when active.
	if m.huhOverlay != nil {
		// Huh uses ctrl+c to abort, not Escape. Intercept Escape ourselves
		// so users can always back out of any dialog with the natural key.
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyEsc {
			m.huhOverlay = nil
			m.statusMsg = "Cancelled"
			return m, nil
		}
		newModel, cmd := m.huhOverlay.Update(msg)
		if f, ok := newModel.(*huh.Form); ok {
			m.huhOverlay = f
		}
		if m.huhOverlay.State == huh.StateCompleted {
			c := m.onHuhComplete()
			m.huhOverlay = nil
			return m, c
		}
		if m.huhOverlay.State == huh.StateAborted {
			m.huhOverlay = nil
			m.statusMsg = "Cancelled"
			return m, nil
		}
		return m, cmd
	}

	switch msg := msg.(type) {

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
		m.updateWatchers()
		return m, m.updatePreview()

	case jump.CloseJumpMsg:
		m.jumpOpen = false
		return m, nil

	// --- Git status messages ---
	case gitstatus.GitStatusMsg:
		m.gitBranch = msg.Branch
		for _, p := range m.panes {
			p.GitStatus = msg.Status
			p.InGitRepo = msg.InRepo
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

	case actions.RevealInFinderMsg:
		ap := m.activeP()
		if ap.Provider.Capabilities().IsRemote {
			m.statusMsg = "Reveal in Finder not available for remote panes"
			return m, nil
		}
		sel := ap.Selected()
		if sel == nil {
			return m, nil
		}
		if err := exec.Command("open", "-R", sel.Path).Start(); err != nil {
			m.statusMsg = "Reveal failed: " + err.Error()
		}
		return m, nil

	case actions.ResizeSplitMsg:
		if m.showPreview {
			return m, nil // resize not applicable in three-pane mode
		}
		ratio := m.splitRatio
		if ratio <= 0 || ratio >= 1 {
			ratio = 0.5
		}
		ratio += float64(msg.Delta) / 100.0
		if ratio < 0.1 {
			ratio = 0.1
		}
		if ratio > 0.9 {
			ratio = 0.9
		}
		m.splitRatio = ratio
		m.layoutPanes()
		return m, nil

	case actions.OpenShellMsg:
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "sh"
		}
		dir := m.activeP().Path
		cmd := exec.Command(shell)
		cmd.Dir = dir
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return nil
		})

	case actions.RunCommandMsg:
		sel := m.activeP().Selected()
		if sel == nil {
			return m, nil
		}
		m.cmdPromptOpen = true
		m.cmdPromptBuf = ""
		m.statusMsg = "! run: "
		return m, nil

	case actions.QuickLookMsg:
		sel := m.activeP().Selected()
		if sel == nil {
			return m, nil
		}
		if err := exec.Command("qlmanage", "-p", sel.Path).Start(); err != nil {
			m.statusMsg = "Quick Look unavailable"
		}
		return m, nil

	case actions.OpenWithMsg:
		sel := m.activeP().Selected()
		if sel == nil {
			return m, nil
		}
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", sel.Path)
		case "linux":
			cmd = exec.Command("xdg-open", sel.Path)
		default:
			m.statusMsg = "Open with not supported on this OS"
			return m, nil
		}
		if err := cmd.Start(); err != nil {
			m.statusMsg = "Open failed: " + err.Error()
		}
		return m, nil

	case actions.DeleteSelectedMsg:
		ap := m.activeP()
		entries := ap.SelectedEntries()
		if len(entries) == 0 {
			return m, nil
		}
		// Single file with confirm disabled: delete immediately.
		if len(entries) == 1 && !m.cfg.General.ConfirmDelete {
			return m, m.executeDelete(entries)
		}
		// Otherwise: show Huh confirmation overlay.
		m.huhDeleteItems = entries
		m.huhMode = "delete"
		m.huhConfirm = false
		title := fmt.Sprintf("Delete %q?", entries[0].Name)
		desc := "This cannot be undone."
		if len(entries) > 1 {
			title = fmt.Sprintf("Delete %d items?", len(entries))
			desc = fmt.Sprintf("%s and %d more…", entries[0].Name, len(entries)-1)
		}
		f := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(title).
					Description(desc).
					Affirmative("Delete").
					Negative("Cancel").
					Value(&m.huhConfirm),
			),
		).WithTheme(m.huhTheme()).WithWidth(52)
		m.huhOverlay = f
		return m, f.Init()

	case actions.BulkRenameMsg:
		items := m.activeP().SelectedEntries()
		if len(items) == 0 {
			if sel := m.activeP().Selected(); sel != nil {
				items = []fileinfo.FileInfo{*sel}
			}
		}
		if len(items) < 2 {
			m.statusMsg = "Select 2 or more items to bulk rename (space to select)"
			return m, nil
		}
		return m, m.startEditorRename(items)

	case actions.RenameSelectedMsg:
		sel := m.activeP().Selected()
		if sel == nil {
			m.statusMsg = "Nothing selected"
			return m, nil
		}
		m.huhInputValue = sel.Name
		m.huhMode = "rename"
		f := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("Rename").
				Description("Current: " + sel.Name).
				Value(&m.huhInputValue).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("name cannot be empty")
					}
					if strings.Contains(s, "/") {
						return fmt.Errorf("name cannot contain /")
					}
					return nil
				}),
		)).WithTheme(m.huhTheme()).WithWidth(56)
		m.huhOverlay = f
		return m, f.Init()

	case actions.NewFileMsg:
		m.huhInputValue = ""
		m.huhMode = "new-file"
		f := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("New File").
				Placeholder("filename").
				Value(&m.huhInputValue).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("name cannot be empty")
					}
					if strings.Contains(s, "/") {
						return fmt.Errorf("name cannot contain /")
					}
					return nil
				}),
		)).WithTheme(m.huhTheme()).WithWidth(56)
		m.huhOverlay = f
		return m, f.Init()

	case actions.NewDirMsg:
		m.huhInputValue = ""
		m.huhMode = "new-dir"
		f := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("New Directory").
				Placeholder("dirname").
				Value(&m.huhInputValue).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("name cannot be empty")
					}
					if strings.Contains(s, "/") {
						return fmt.Errorf("name cannot contain /")
					}
					return nil
				}),
		)).WithTheme(m.huhTheme()).WithWidth(56)
		m.huhOverlay = f
		return m, f.Init()

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
		m.updateWatchers()
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

	// --- Marks messages ---
	case actions.SetMarkMsg:
		m.markPending = true
		m.statusMsg = "Mark: press key…"
		return m, nil

	case actions.JumpMarkMsg:
		m.jumpPending = true
		m.statusMsg = "Jump to mark: press key…"
		return m, nil

	// --- Search messages ---
	case actions.OpenSearchMsg:
		m.searchModel.Width = m.width
		m.searchModel.Height = m.height
		m.searchOpen = true
		openCmd := m.searchModel.Open(m.activeP().Path)
		searchCmd := search.RunSearch(m.activeP().Path)
		return m, tea.Batch(openCmd, searchCmd)

	case search.SearchDoneMsg:
		m.searchModel.SetResults(msg.Paths)
		return m, nil

	case search.ResultMsg:
		m.searchOpen = false
		// Navigate to the directory containing the result, then select the file.
		dir := msg.Path
		// If it's a file, navigate to its parent.
		info, err := os.Stat(msg.Path)
		if err == nil && !info.IsDir() {
			dir = filepath.Dir(msg.Path)
		}
		return m, m.navigateTo(dir)

	case search.CloseSearchMsg:
		m.searchOpen = false
		return m, nil

	// --- Editor rename messages ---
	case editorRenameMsg:
		return m, m.applyEditorRename(msg)

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
		m.updateWatchers()
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

	case watchEventMsg:
		if msg.dir == "" {
			// Error event — re-arm without reloading.
			return m, waitForWatchEvent(m.watcher)
		}
		// Debounce: skip reloads within 400ms of the last one for this dir.
		now := time.Now()
		if m.watchDebounce == nil {
			m.watchDebounce = make(map[string]time.Time)
		}
		if last, ok := m.watchDebounce[msg.dir]; ok && now.Sub(last) < 400*time.Millisecond {
			return m, waitForWatchEvent(m.watcher)
		}
		m.watchDebounce[msg.dir] = now
		// Reload any pane watching this directory.
		reloaded := false
		for _, p := range m.panes {
			if p.Path == msg.dir && !p.Provider.Capabilities().IsRemote {
				p.Reload()
				reloaded = true
			}
		}
		_ = reloaded
		return m, waitForWatchEvent(m.watcher)

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

	// If search overlay is open, route all keys there.
	if m.searchOpen {
		updated, cmd := m.searchModel.Update(msg)
		m.searchModel = updated
		return m, cmd
	}

	// If run-command prompt is open, accumulate input.
	if m.cmdPromptOpen {
		switch msg.Type {
		case tea.KeyEsc:
			m.cmdPromptOpen = false
			m.cmdPromptBuf = ""
			m.statusMsg = "Cancelled"
			return m, nil
		case tea.KeyEnter:
			cmdStr := strings.TrimSpace(m.cmdPromptBuf)
			m.cmdPromptOpen = false
			m.cmdPromptBuf = ""
			m.statusMsg = ""
			if cmdStr == "" {
				return m, nil
			}
			sel := m.activeP().Selected()
			var arg string
			if sel != nil {
				arg = sel.Path
			}
			shell := os.Getenv("SHELL")
			if shell == "" {
				shell = "sh"
			}
			cmd := exec.Command(shell, "-c", cmdStr+" "+arg)
			cmd.Dir = m.activeP().Path
			return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
				return nil
			})
		case tea.KeyBackspace, tea.KeyDelete:
			if len(m.cmdPromptBuf) > 0 {
				m.cmdPromptBuf = m.cmdPromptBuf[:len(m.cmdPromptBuf)-1]
				m.statusMsg = "! run: " + m.cmdPromptBuf
			}
			return m, nil
		default:
			if msg.Type == tea.KeyRunes {
				m.cmdPromptBuf += string(msg.Runes)
				m.statusMsg = "! run: " + m.cmdPromptBuf
			}
			return m, nil
		}
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

	// Handle two-step mark commands.
	if m.markPending {
		m.markPending = false
		r := []rune(key)
		if len(r) == 1 {
			m.marks[r[0]] = m.activeP().Path
			m.statusMsg = fmt.Sprintf("Marked '%c' → %s", r[0], m.activeP().Path)
		}
		return m, nil
	}
	if m.jumpPending {
		m.jumpPending = false
		r := []rune(key)
		if len(r) == 1 {
			if path, ok := m.marks[r[0]]; ok {
				return m, m.navigateTo(path)
			}
			m.statusMsg = fmt.Sprintf("No mark '%c'", r[0])
		}
		return m, nil
	}

	// Preview scroll keys — handled before action dispatch.
	if m.showPreview {
		// Route keys to preview search when open.
		if m.previewModel.SearchOpen() {
			switch key {
			case "n":
				m.previewModel.NextMatch()
				return m, nil
			case "N", "shift+n":
				m.previewModel.PrevMatch()
				return m, nil
			default:
				if m.previewModel.HandleSearchKey(msg) {
					return m, nil
				}
			}
		}
		// Open search on "/".
		if key == "/" {
			m.previewModel.OpenSearch()
			return m, nil
		}
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

	// "R" / "shift+r" — bulk rename. Belt-and-suspenders: registered in keymap
	// but some terminal/keyboard modes bypass the standard keymap lookup.
	if key == "R" || key == "shift+r" {
		return m, func() tea.Msg { return actions.BulkRenameMsg{} }
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

	if id, ok := m.keyMap[key]; ok {
		return id, ok
	}

	// Kitty keyboard protocol / enhanced keyboard mode reports shift+letter as
	// "shift+r" rather than "R". Normalise to the uppercase character so that
	// keybindings registered as e.g. "R" are found regardless of terminal.
	if strings.HasPrefix(key, "shift+") {
		remainder := key[len("shift+"):]
		if len(remainder) == 1 && remainder[0] >= 'a' && remainder[0] <= 'z' {
			upper := strings.ToUpper(remainder)
			if id, ok := m.keyMap[upper]; ok {
				return id, ok
			}
		}
	}

	return "", false
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
			m.updateWatchers()
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
			m.updateWatchers()
		}
	}
	return tea.Batch(m.updatePreview(), m.updateGitStatus())
}

// navigateTo navigates the active pane to the given path.
func (m *Model) navigateTo(path string) tea.Cmd {
	ap := m.activeP()
	ap.Path = path
	ap.Reload()
	m.store.Visit(path)
	_ = m.store.Save()
	m.updateWatchers()
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

// waitForWatchEvent returns a tea.Cmd that blocks until the watcher emits an
// event, then returns a watchEventMsg for the containing directory.
func waitForWatchEvent(w *fsnotify.Watcher) tea.Cmd {
	return func() tea.Msg {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			return watchEventMsg{dir: filepath.Dir(event.Name)}
		case _, ok := <-w.Errors:
			if !ok {
				return nil
			}
			// Swallow watcher errors and re-arm.
			return watchEventMsg{dir: ""}
		}
	}
}

// updateWatchers syncs the fsnotify watcher with the current pane directories.
// Only watches local (non-remote) pane paths.
func (m *Model) updateWatchers() {
	if m.watcher == nil {
		return
	}
	// Remove all existing watches.
	for _, p := range m.watcher.WatchList() {
		_ = m.watcher.Remove(p)
	}
	// Add current local pane directories.
	for _, p := range m.panes {
		if !p.Provider.Capabilities().IsRemote {
			_ = m.watcher.Add(p.Path)
		}
	}
}

// closeAllSFTP closes all open SFTP provider connections.
func (m *Model) closeAllSFTP() {
	for _, prov := range m.sftpProviders {
		prov.Close()
	}
	if m.watcher != nil {
		_ = m.watcher.Close()
		m.watcher = nil
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
		// Two-pane layout: splitRatio controls left share (default 50/50).
		usable := m.width - 1 // one divider
		ratio := m.splitRatio
		if ratio <= 0 || ratio >= 1 {
			ratio = 0.5
		}
		leftW := int(float64(usable) * ratio)
		if leftW < 10 {
			leftW = 10
		}
		if leftW > usable-10 {
			leftW = usable - 10
		}
		m.panes[0].Width = leftW
		m.panes[0].Height = paneH
		m.panes[1].Width = usable - leftW
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

	// Overlay search if open.
	if m.searchOpen {
		searchView := m.searchModel.View()
		screen = overlayCenter(screen, searchView, m.width, m.height)
	}

	// Overlay Huh form (delete confirm, rename, new file/dir) if active.
	if m.huhOverlay != nil {
		label := "Confirm"
		switch m.huhMode {
		case "delete":
			label = "Delete"
		case "rename":
			label = "Rename"
		case "new-file":
			label = "New File"
		case "new-dir":
			label = "New Directory"
		}
		header := m.theme.PaletteSelected.Render("  " + label + "  ")
		body := lipgloss.JoinVertical(lipgloss.Left, header, "", m.huhOverlay.View())
		box := m.theme.PaletteBox.Render(body)
		screen = overlayCenter(screen, box, m.width, m.height)
	}

	return screen
}

// renderHeader builds the branded header bar (1 row).
// Left: branding. Right: active pane indicator + key hints.
// Path is not shown here — the status bar breadcrumb is the canonical location.
func (m *Model) renderHeader() string {
	paneLabel := "⬡ left"
	if m.activePane == 1 {
		paneLabel = "⬡ right"
	}

	left := "  PELORUS"
	right := "   " + paneLabel + "   ctrl+p palette   g jump   c connect  "

	// Render each section with the appropriate theme style.
	leftPart := m.theme.HeaderTitle.Render(left)
	rightPart := m.theme.HeaderHint.Render(right)

	// Fill the gap with the plain header background.
	gapW := m.width - lipgloss.Width(leftPart) - lipgloss.Width(rightPart)
	if gapW < 0 {
		gapW = 0
	}
	gapPart := m.theme.Header.Width(gapW).Render("")

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPart, gapPart, rightPart)
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

// onHuhComplete is called when a Huh overlay form reaches StateCompleted.
func (m *Model) onHuhComplete() tea.Cmd {
	switch m.huhMode {
	case "delete":
		if m.huhConfirm {
			return m.executeDelete(m.huhDeleteItems)
		}
		m.statusMsg = "Cancelled"
		return nil
	case "rename":
		return m.executeRename(m.huhInputValue)
	case "new-file":
		return m.executeNewFile(m.huhInputValue)
	case "new-dir":
		return m.executeNewDir(m.huhInputValue)
	}
	return nil
}

// executeDelete enqueues deletion of the given items.
func (m *Model) executeDelete(items []fileinfo.FileInfo) tea.Cmd {
	ap := m.activeP()
	ap.ClearSelection()
	var cmds []tea.Cmd
	for _, fi := range items {
		job := m.queue.Add(ops.KindDelete, fi.Path, "")
		job.Status = ops.StatusRunning
		job.StartTime = time.Now()
		cmds = append(cmds, ops.StartJob(job, ap.Provider, ap.Provider))
	}
	m.statusMsg = fmt.Sprintf("Deleting %d item(s)…", len(items))
	cmds = append(cmds, m.startProgressTicker())
	return tea.Batch(cmds...)
}

func (m *Model) executeRename(newName string) tea.Cmd {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil
	}
	ap := m.activeP()
	sel := ap.Selected()
	if sel == nil {
		return nil
	}
	dst := filepath.Join(ap.Path, newName)
	if err := ap.Provider.Rename(sel.Path, dst); err != nil {
		m.statusMsg = "Rename failed: " + err.Error()
		return nil
	}
	ap.Reload()
	m.updateWatchers()
	return m.updatePreview()
}

func (m *Model) executeNewFile(name string) tea.Cmd {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	ap := m.activeP()
	target := filepath.Join(ap.Path, name)
	f, err := os.Create(target)
	if err != nil {
		m.statusMsg = "Create failed: " + err.Error()
		return nil
	}
	f.Close()
	ap.Reload()
	m.updateWatchers()
	return m.updatePreview()
}

func (m *Model) executeNewDir(name string) tea.Cmd {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	ap := m.activeP()
	if err := ap.Provider.MakeDir(filepath.Join(ap.Path, name)); err != nil {
		m.statusMsg = "MakeDir failed: " + err.Error()
		return nil
	}
	ap.Reload()
	m.updateWatchers()
	return m.updatePreview()
}

// editorRenameMsg is sent by tea.ExecProcess after the user exits the editor.
type editorRenameMsg struct {
	tmpPath   string
	origItems []fileinfo.FileInfo
}

// startEditorRename opens $EDITOR with a temp file of filenames for bulk rename.
func (m *Model) startEditorRename(items []fileinfo.FileInfo) tea.Cmd {
	tmp, err := os.CreateTemp("", "pelorus-rename-*.txt")
	if err != nil {
		m.statusMsg = "Rename failed: " + err.Error()
		return nil
	}
	header := "# Rename: edit filenames below, one per line.\n# Do not add or remove lines.\n# Save and exit to apply. Empty file or unchanged = no renames.\n\n"
	if _, err := tmp.WriteString(header); err != nil {
		tmp.Close()
		return nil
	}
	for _, item := range items {
		if _, err := tmp.WriteString(item.Name + "\n"); err != nil {
			tmp.Close()
			return nil
		}
	}
	tmp.Close()
	tmpPath := tmp.Name()

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

	origItems := make([]fileinfo.FileInfo, len(items))
	copy(origItems, items)

	return tea.ExecProcess(exec.Command(editor, tmpPath), func(err error) tea.Msg {
		if err != nil {
			os.Remove(tmpPath)
			return pane.ErrMsg{Err: fmt.Errorf("editor: %w", err)}
		}
		return editorRenameMsg{tmpPath: tmpPath, origItems: origItems}
	})
}

// applyEditorRename reads the temp file and applies the renames.
func (m *Model) applyEditorRename(msg editorRenameMsg) tea.Cmd {
	defer os.Remove(msg.tmpPath)
	data, err := os.ReadFile(msg.tmpPath)
	if err != nil {
		m.statusMsg = "Rename failed: " + err.Error()
		return nil
	}
	var newNames []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		newNames = append(newNames, line)
	}
	if len(newNames) != len(msg.origItems) {
		m.statusMsg = fmt.Sprintf("Rename cancelled: expected %d names, got %d", len(msg.origItems), len(newNames))
		return nil
	}
	ap := m.activeP()
	renamed := 0
	var errs []string
	for i, item := range msg.origItems {
		newName := strings.TrimSpace(newNames[i])
		if newName == item.Name || newName == "" {
			continue
		}
		dst := filepath.Join(ap.Path, newName)
		if err := ap.Provider.Rename(item.Path, dst); err != nil {
			errs = append(errs, err.Error())
		} else {
			renamed++
		}
	}
	ap.ClearSelection()
	ap.Reload()
	m.updateWatchers()
	if len(errs) > 0 {
		m.statusMsg = fmt.Sprintf("Renamed %d, %d errors: %s", renamed, len(errs), errs[0])
	} else if renamed > 0 {
		m.statusMsg = fmt.Sprintf("Renamed %d item(s)", renamed)
	} else {
		m.statusMsg = "No changes"
	}
	return m.updatePreview()
}

// huhTheme returns a Huh form theme derived from the active pelorus theme.
func (m *Model) huhTheme() *huh.Theme {
	accent := m.theme.StatusBarAccent.GetForeground()
	muted := m.theme.StatusBarMuted.GetForeground()
	text := m.theme.FileName.GetForeground()
	cursorBg := m.theme.Cursor.GetBackground()
	cursorFg := m.theme.Cursor.GetForeground()

	t := huh.ThemeBase()
	t.Focused.Title = lipgloss.NewStyle().Foreground(accent).Bold(true)
	t.Focused.Description = lipgloss.NewStyle().Foreground(muted)
	t.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(accent)
	t.Focused.TextInput.Text = lipgloss.NewStyle().Foreground(text)
	t.Focused.SelectedOption = lipgloss.NewStyle().Background(cursorBg).Foreground(cursorFg)
	t.Focused.UnselectedOption = lipgloss.NewStyle().Foreground(muted)
	t.Focused.ErrorMessage = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555"))
	t.Focused.ErrorIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555"))
	t.Blurred.Title = lipgloss.NewStyle().Foreground(muted)
	return t
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
