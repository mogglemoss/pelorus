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
	"github.com/charmbracelet/bubbles/textinput"
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
	"github.com/mogglemoss/pelorus/internal/mascot"
	"github.com/mogglemoss/pelorus/internal/ops"
	"github.com/mogglemoss/pelorus/internal/palette"
	"github.com/mogglemoss/pelorus/internal/pane"
	"github.com/mogglemoss/pelorus/internal/preview"
	"github.com/mogglemoss/pelorus/internal/provider"
	localprov "github.com/mogglemoss/pelorus/internal/provider/local"
	sftpprov "github.com/mogglemoss/pelorus/internal/provider/sftp"
	"github.com/mogglemoss/pelorus/internal/search"
	"github.com/mogglemoss/pelorus/internal/theme"
	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

// tickMsg is sent by the progress ticker every 200ms.
type tickMsg struct{}

// watchEventMsg is sent when the filesystem watcher detects a change.
type watchEventMsg struct{ dir string }

// footerItem records the position and action of one shortcut hint in the footer.
type footerItem struct {
	startX   int
	endX     int
	actionID string
}

// headerTaglines rotates through these in row 1 of the header.
var headerTaglines = []string{
	"NAVIGATIONAL INSTRUMENT",
	"COURSE CORRECTION ACTIVE",
	"BEARING NOMINAL",
	"HEADING ACQUISITION SYSTEM",
	"CELESTIAL FIX IN PROGRESS",
}

// numHeaderRows is the fixed height of the 3-row header.
const numHeaderRows = 3

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
	connecting    bool // true while an SFTP/SSH connection is being established

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
	cmdPromptOpen  bool
	cmdPromptInput textinput.Model

	statusMsg string // transient message shown in status bar
	gitBranch string // current git branch for active pane's repo

	// Mascot animation state.
	mascotFrame    int
	mascotActive   bool // true while any async work is in flight
	footerHoverIdx int  // -1 = no hover; index into footerItems
	footerItems    []footerItem
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
	m.panes[0].Label = "LOCAL"
	m.panes[1].Label = "LOCAL"

	m.paletteModel = palette.New(reg, t)
	m.previewModel = preview.New(t)
	m.queue = ops.NewQueue()
	m.queueModel = ops.NewQueueModel(m.queue, t)
	m.connectModel, _ = connect.NewModel(t) // ignore error (empty hosts ok)
	m.sftpProviders = make(map[string]*sftpprov.Provider)
	m.helpModel = help.New(reg, t)
	m.marks = make(map[rune]string)
	m.footerHoverIdx = -1
	m.searchModel = search.New(t)
	cmdTi := textinput.New()
	cmdTi.Placeholder = "command…"
	cmdTi.CharLimit = 256
	m.cmdPromptInput = cmdTi

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
		return mascot.Tick()
	}
	m.updateWatchers()
	return tea.Batch(waitForWatchEvent(m.watcher), mascot.Tick())
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

	case tea.MouseMsg:
		return m, m.handleMouse(msg)
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

	case actions.PreviewSearchMsg:
		if sel := m.activeP().Selected(); sel != nil && !sel.IsDir {
			if !m.showPreview {
				m.showPreview = true
				m.layoutPanes()
			}
			m.previewModel.OpenSearch()
		}
		return m, nil

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
		if m.activeP().Provider.Capabilities().IsRemote {
			m.statusMsg = "Open shell not available on remote panes"
			return m, nil
		}
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
		if m.activeP().Provider.Capabilities().IsRemote {
			m.statusMsg = "Run command not available on remote panes"
			return m, nil
		}
		m.cmdPromptOpen = true
		m.cmdPromptInput.SetValue("")
		m.cmdPromptInput.Focus()
		return m, textinput.Blink

	case actions.QuickLookMsg:
		if m.activeP().Provider.Capabilities().IsRemote {
			m.statusMsg = "Quick Look not available on remote panes"
			return m, nil
		}
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
		if sel.IsDir {
			// Directories: use l to enter, ctrl+r to reveal in Finder.
			m.statusMsg = "Use l to enter directories"
			return m, nil
		}
		if m.activeP().Provider.Capabilities().IsRemote {
			m.statusMsg = "Open with not available on remote panes"
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
		m.connecting = true
		return m, m.connectToHost(msg)

	case connectErrMsg:
		m.connecting = false
		m.statusMsg = "Connect failed: " + msg.err.Error()
		return m, nil

	case connectedMsg:
		m.connecting = false
		ip := m.inactiveP()
		if old, ok := m.sftpProviders[msg.alias]; ok {
			old.Close()
		}
		m.sftpProviders[msg.alias] = msg.prov
		ip.Provider = msg.prov
		ip.Path = "/"
		ip.Reload()
		ip.Label = strings.ToUpper(msg.alias)
		m.updateWatchers()
		return m, nil

	case actions.DisconnectMsg:
		ap := m.activeP()
		if !ap.Provider.Capabilities().IsRemote {
			m.statusMsg = "Active pane is not a remote session"
			return m, nil
		}
		// Find and close the SFTP provider.
		for alias, prov := range m.sftpProviders {
			if prov == ap.Provider {
				prov.Close()
				delete(m.sftpProviders, alias)
				break
			}
		}
		// Revert to local provider navigated to home dir.
		home, err := os.UserHomeDir()
		if err != nil {
			home = "/"
		}
		localProv := m.panes[1-m.activePane].Provider // borrow local prov from the other pane if it's local
		if localProv.Capabilities().IsRemote {
			// Both panes remote — use a fresh local provider instance from the other pane's original.
			// Fallback: navigate to home with a basic local provider.
			localProv = localprov.New()
		}
		ap.Provider = localProv
		ap.Path = home
		ap.Reload()
		ap.Label = "LOCAL"
		m.updateWatchers()
		return m, tea.Batch(m.updatePreview(), m.updateGitStatus())

	// --- Queue messages ---
	case actions.OpenQueueMsg:
		m.queueOpen = true
		m.queueModel.Width = m.width
		m.queueModel.Height = m.height
		return m, nil

	case ops.CloseQueueMsg:
		m.queueOpen = false
		return m, nil

	case mascot.TickMsg:
		working := m.previewModel.IsLoading() || len(m.queue.Running()) > 0 || m.connecting
		if working || m.mascotFrame != 0 {
			// Keep animating until the current cycle completes (frame wraps to 0).
			m.mascotActive = true
			m.mascotFrame = (m.mascotFrame + 1) % 6
		} else {
			m.mascotActive = false
		}
		return m, mascot.Tick()

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

	// If run-command prompt is open, route keys to the textinput.
	if m.cmdPromptOpen {
		switch msg.Type {
		case tea.KeyEsc:
			m.cmdPromptOpen = false
			m.cmdPromptInput.Blur()
			return m, nil
		case tea.KeyEnter:
			cmdStr := strings.TrimSpace(m.cmdPromptInput.Value())
			m.cmdPromptOpen = false
			m.cmdPromptInput.Blur()
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
		default:
			var cmd tea.Cmd
			m.cmdPromptInput, cmd = m.cmdPromptInput.Update(msg)
			return m, cmd
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

	// "/" always opens preview content search — opens the preview pane first if
	// it isn't visible. No-op when no file is selected.
	if key == "/" {
		if sel := m.activeP().Selected(); sel != nil && !sel.IsDir {
			if !m.showPreview {
				m.showPreview = true
				m.layoutPanes()
			}
			m.previewModel.OpenSearch()
		}
		return m, nil
	}

	// Preview scroll and search-navigation keys.
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
	// Only trigger for chars that can plausibly start a filename: letters, digits,
	// dot (hidden files), underscore, hyphen. Symbols such as ] [ > < ! ' are
	// reserved for actions and must never fall through to the filter, even when
	// their action isn't currently active (e.g. ] scrolls preview, but shouldn't
	// start a filter when the preview pane is closed).
	if len(key) == 1 {
		ch := key[0]
		isFilenameStart := (ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '.' || ch == '_' || ch == '-'
		if isFilenameStart {
			ap.StartFilter()
			ap.FilterStr = key
			ap.ApplyFilterPublic()
			return m, nil
		}
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

// handleMouse processes mouse events: scroll wheel on panes/preview,
// and left-click to focus the pane under the cursor.
func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	// Route scroll to whichever overlay is open.
	if m.paletteOpen {
		m.paletteModel, _ = m.paletteModel.Update(msg)
		return nil
	}
	if m.jumpOpen {
		m.jumpModel, _ = m.jumpModel.Update(msg)
		return nil
	}
	if m.connectOpen && m.connectModel != nil {
		m.connectModel, _ = m.connectModel.Update(msg)
		return nil
	}
	if m.searchOpen {
		m.searchModel, _ = m.searchModel.Update(msg)
		return nil
	}
	// Ignore mouse for other overlays (huh, help, queue).
	if m.huhOverlay != nil || m.helpOpen || m.queueOpen {
		return nil
	}

	// Layout geometry (mirrors layoutPanes).
	// Rows 0–(numHeaderRows-1) = header; panes start at row numHeaderRows.
	footerRow := m.height - 1 // last row
	paneRow := msg.Y - numHeaderRows // row within the pane area

	left0 := 0
	left1 := m.panes[0].Width // column where divider sits
	right0 := left1 + 1       // first column of right pane
	right1 := right0 + m.panes[1].Width

	// Determine which zone the click/scroll is in.
	inLeft := msg.X >= left0 && msg.X < left1
	inRight := msg.X >= right0 && msg.X < right1
	inPreview := m.showPreview && msg.X >= right1+1
	inFooter := msg.Y == footerRow

	// Mouse motion and release: update footer hover highlight.
	if msg.Action == tea.MouseActionMotion || msg.Action == tea.MouseActionRelease {
		newHover := -1
		if inFooter {
			for i, item := range m.footerItems {
				if msg.X >= item.startX && msg.X <= item.endX {
					newHover = i
					break
				}
			}
		}
		if newHover != m.footerHoverIdx {
			m.footerHoverIdx = newHover
		}
		// Fire footer action on release (terminals send release for a click).
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft && newHover >= 0 {
			item := m.footerItems[newHover]
			if item.actionID != "" {
				if a, ok := m.registry.ByID(item.actionID); ok {
					return a.Handler(m.buildAppState())
				}
			}
		}
		return nil
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if inPreview {
			m.previewModel.ScrollUp(3)
			return nil
		}
		if inLeft || (!inRight && m.activePane == 0) {
			m.panes[0].ScrollUp(3)
		} else {
			m.panes[1].ScrollUp(3)
		}
		return m.updatePreview()

	case tea.MouseButtonWheelDown:
		if inPreview {
			m.previewModel.ScrollDown(3)
			return nil
		}
		if inLeft || (!inRight && m.activePane == 0) {
			m.panes[0].ScrollDown(3)
		} else {
			m.panes[1].ScrollDown(3)
		}
		return m.updatePreview()

	case tea.MouseButtonLeft:
		// Footer shortcut click: find the item by X position directly.
		if inFooter {
			for _, item := range m.footerItems {
				if msg.X >= item.startX && msg.X <= item.endX {
					if item.actionID != "" {
						if a, ok := m.registry.ByID(item.actionID); ok {
							return a.Handler(m.buildAppState())
						}
					}
					break
				}
			}
		}
		if paneRow < 0 {
			return nil // click in header
		}
		if inLeft && m.activePane != 0 {
			m.panes[m.activePane].IsActive = false
			m.activePane = 0
			m.panes[0].IsActive = true
			return m.updatePreview()
		}
		if inRight && m.activePane != 1 {
			m.panes[m.activePane].IsActive = false
			m.activePane = 1
			m.panes[1].IsActive = true
			return m.updatePreview()
		}
	}
	return nil
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
	// Reserve 3 rows for header + 1 row for footer.
	paneH := m.height - numHeaderRows - 1

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

	// Footer shortcut bar.
	footer := m.renderFooter()

	screen := lipgloss.JoinVertical(lipgloss.Left, header, panesRow, footer)

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

// renderHeader builds the branded header bar (3 rows).
//
//	Row 1: "PELORUS" title (left) + mascot antenna (right)
//	Row 2: breadcrumb path + git branch (left) + mascot head (right)
//	Row 3: perms / size / selection count (left) + mascot face (right)
func (m *Model) renderHeader() string {
	ap := m.activeP()
	hdrBg := m.theme.HeaderBg

	mascotView := mascot.View(m.mascotFrame, m.mascotActive, hdrBg)
	mascotW := mascot.Width()
	mascotLines := strings.Split(mascotView, "\n")
	mascotLine := func(n int) string {
		if n < len(mascotLines) {
			return mascotLines[n]
		}
		return strings.Repeat(" ", mascotW)
	}

	// ---- Row 1: title (left) + mascot antenna (right) ----

	titlePart := m.theme.HeaderTitle.Render(" PELORUS")
	titleW := lipgloss.Width(titlePart)
	row1GapW := m.width - titleW - mascotW
	if row1GapW < 0 {
		row1GapW = 0
	}
	row1Gap := m.theme.Header.Width(row1GapW).Render("")
	row1 := lipgloss.JoinHorizontal(lipgloss.Top, titlePart, row1Gap, mascotLine(0))

	// ---- Row 2: breadcrumb + git branch (left) + mascot head (right) ----

	home, _ := os.UserHomeDir()
	path := ap.Path
	if home != "" && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	breadcrumb := strings.ReplaceAll(path, string(filepath.Separator), " › ")
	if breadcrumb == "" {
		breadcrumb = "/"
	}
	if ap.FilterStr != "" {
		breadcrumb += fmt.Sprintf("  /%s", ap.FilterStr)
	}

	// Use header background for rows 2 & 3 — StatusBarAccent/Muted carry the
	// status bar background which would create a colored bar in the header.
	hdrFgAccent := lipgloss.NewStyle().Background(lipgloss.Color(hdrBg)).Foreground(m.theme.StatusBarAccent.GetForeground())
	hdrFgMuted := lipgloss.NewStyle().Background(lipgloss.Color(hdrBg)).Foreground(m.theme.StatusBarMuted.GetForeground())

	pathPart := hdrFgAccent.Render(" " + breadcrumb)
	gitPart := ""
	if m.gitBranch != "" {
		gitPart = hdrFgMuted.Render("  ⎇ " + m.gitBranch)
	}
	breadW2 := m.width - mascotW
	if breadW2 < 4 {
		breadW2 = 4
	}
	usedW := lipgloss.Width(pathPart) + lipgloss.Width(gitPart)
	padW := breadW2 - usedW
	if padW < 0 {
		padW = 0
	}
	breadSeg := pathPart + gitPart + m.theme.Header.Width(padW).Render("")
	row2 := lipgloss.JoinHorizontal(lipgloss.Top, breadSeg, mascotLine(1))

	// ---- Row 3: perms / size / selection (left) + mascot face (right) ----

	permsStr := ""
	if sel := ap.Selected(); sel != nil {
		permsStr = sel.Mode.String() + "  " + fileinfo.HumanSize(sel.Size)
	}
	if len(ap.MultiSel) > 0 {
		permsStr = fmt.Sprintf("%d selected  ", len(ap.MultiSel)) + permsStr
	}

	permsW := m.width - mascotW
	if permsW < 1 {
		permsW = 1
	}
	permsSeg := hdrFgMuted.Width(permsW).Render(" " + permsStr)
	row3 := lipgloss.JoinHorizontal(lipgloss.Top, permsSeg, mascotLine(2))

	return lipgloss.JoinVertical(lipgloss.Left, row1, row2, row3)
}

// renderFooter builds the footer shortcut bar.
//
// Priority overrides (full-width):
//  1. Run-command prompt (cmdPromptOpen)
//  2. Transient status message (statusMsg)
//
// Default: shortcut hints justified across the full width with key chips in
// accent color and descriptions in muted color. Mouse hover is tracked via
// footerHoverIdx / footerItems.
func (m *Model) renderFooter() string {
	// Run-command prompt: replace footer with a full-width textinput.
	if m.cmdPromptOpen {
		prompt := m.theme.StatusBarAccent.Render(" ! ")
		input := m.theme.StatusBar.Width(m.width - lipgloss.Width(prompt)).Render(m.cmdPromptInput.View())
		return lipgloss.JoinHorizontal(lipgloss.Top, prompt, input)
	}

	// Transient status message.
	if m.statusMsg != "" {
		return m.theme.StatusBar.Width(m.width).Render(" " + m.statusMsg)
	}

	// Build the shortcut list. Always-present shortcuts first, then context
	// shortcuts based on what is currently selected.
	type shortcut struct {
		key      string
		desc     string
		actionID string
	}
	shortcuts := []shortcut{
		{"tab", "switch", "nav.switch-pane"},
		{"/", "search", "view.preview-search"},
		{"^p", "palette", "palette.open"},
		{"?", "help", "app.help"},
		{"q", "quit", "app.quit"},
	}
	ap := m.activeP()
	if sel := ap.Selected(); sel != nil {
		if sel.IsDir {
			shortcuts = append(shortcuts, shortcut{"l", "enter", "nav.enter"})
		} else {
			shortcuts = append(shortcuts, shortcut{"e", "edit", "file.open-editor"})
		}
	}
	if len(ap.MultiSel) > 0 {
		shortcuts = append(shortcuts, shortcut{"C", "copy", "file.copy"})
		shortcuts = append(shortcuts, shortcut{"M", "move", "file.move"})
	}

	// Measure each shortcut's rendered width: " key desc "
	type rendered struct {
		text  string
		width int
		start int
		end   int
	}
	parts := make([]rendered, len(shortcuts))
	totalW := 0
	for i, sc := range shortcuts {
		text := " " + sc.key + " " + sc.desc + " "
		w := lipgloss.Width(text)
		parts[i] = rendered{text: text, width: w}
		totalW += w
	}

	// Distribute remaining space as gaps between items.
	gaps := len(shortcuts) - 1
	extraW := m.width - totalW
	if extraW < 0 {
		extraW = 0
	}
	gapW := 0
	if gaps > 0 {
		gapW = extraW / gaps
	}

	bg := lipgloss.Color(m.theme.FooterBg)
	baseBg := lipgloss.NewStyle().Background(bg)

	var sb strings.Builder
	newItems := make([]footerItem, len(shortcuts))
	x := 0
	for i, sc := range shortcuts {
		p := parts[i]
		// Split into key chip and description.
		keyText := " " + sc.key + " "
		descText := sc.desc + " "

		var rendered string
		if m.footerHoverIdx == i {
			rendered = m.theme.FooterHover.Render(keyText + descText)
		} else {
			rendered = m.theme.FooterKey.Render(keyText) + m.theme.FooterDesc.Render(descText)
		}
		sb.WriteString(rendered)
		newItems[i] = footerItem{startX: x, endX: x + p.width - 1, actionID: sc.actionID}
		x += p.width

		// Gap between items (not after last).
		if i < len(shortcuts)-1 && gapW > 0 {
			sb.WriteString(baseBg.Width(gapW).Render(""))
			x += gapW
		}
	}

	// Fill any remaining space.
	if x < m.width {
		sb.WriteString(baseBg.Width(m.width - x).Render(""))
	}

	// Cache items for mouse hit-testing.
	m.footerItems = newItems

	return sb.String()
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
