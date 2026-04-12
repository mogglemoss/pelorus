package connect

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	sftpprov "github.com/mogglemoss/pelorus/internal/provider/sftp"
	"github.com/mogglemoss/pelorus/internal/theme"
	"github.com/mogglemoss/pelorus/internal/tsutil"
)

// SSHHost holds the parsed details for a single SSH config entry.
type SSHHost struct {
	Alias         string
	HostName      string
	User          string
	Port          int
	IdentityFiles []string
}

// ConnectMsg is emitted when the user confirms connection details.
// Exactly one of Host or Node will be non-nil.
type ConnectMsg struct {
	Host     *SSHHost
	Node     *tsutil.Node
	Username string
	Password string
	Port     int
}

// CloseConnectMsg is emitted when the connect palette is closed without selection.
type CloseConnectMsg struct{}

// TsNodesMsg carries the result of the async Tailscale node fetch.
type TsNodesMsg struct{ Nodes []tsutil.Node }

// connectState tracks which screen of the connect overlay is visible.
type connectState int

const (
	stateList connectState = iota
	stateForm
)

// formField pairs a label with its textinput model.
type formField struct {
	label string
	input textinput.Model
}

// item is a unified selectable entry in the connect palette.
type item struct {
	kind    string // "ssh" or "ts"
	ssh     SSHHost
	ts      tsutil.Node
	display string // used for fuzzy matching
}

// Model is the Bubbletea model for the connect palette overlay.
type Model struct {
	Width  int
	Height int
	Theme  *theme.Theme

	hosts    []SSHHost
	tsNodes  []tsutil.Node
	tsLoaded bool

	allItems      []item // all items (unfiltered)
	filteredItems []item // items after applying filter
	input         textinput.Model
	cursor        int
	active        bool
	spinner       spinner.Model

	// Form state.
	state        connectState
	selectedHost *SSHHost
	selectedNode *tsutil.Node
	formFields   []formField // [0]=username [1]=port [2]=password
	formCursor   int
}

// ParseSSHConfig reads ~/.ssh/config and returns parsed SSH host entries.
// If the config file does not exist, returns empty slice (not an error).
func ParseSSHConfig() ([]SSHHost, error) {
	raw, err := sftpprov.ParseSSHConfig()
	if err != nil {
		return nil, err
	}

	hosts := make([]SSHHost, 0, len(raw))
	for _, h := range raw {
		hosts = append(hosts, SSHHost{
			Alias:         h.Alias,
			HostName:      h.HostName,
			User:          h.User,
			Port:          h.Port,
			IdentityFiles: h.IdentityFiles,
		})
	}
	return hosts, nil
}

// NewModel creates and initialises a connect palette model.
func NewModel(t *theme.Theme) (*Model, error) {
	hosts, err := ParseSSHConfig()
	if err != nil {
		hosts = nil
	}

	ti := textinput.New()
	ti.Placeholder = "Filter hosts…"
	ti.CharLimit = 128

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffd0"))

	m := &Model{
		Theme:   t,
		hosts:   hosts,
		input:   ti,
		spinner: s,
	}
	m.buildItems()
	return m, nil
}

// Open resets and activates the overlay. Returns a Cmd that fetches Tailscale nodes async.
func (m *Model) Open() tea.Cmd {
	m.input.SetValue("")
	m.input.Focus()
	m.cursor = 0
	m.tsLoaded = false
	m.tsNodes = nil
	m.state = stateList
	m.selectedHost = nil
	m.selectedNode = nil
	m.buildItems()
	m.active = true
	return tea.Batch(m.fetchTailscaleNodes(), m.spinner.Tick)
}

func (m *Model) fetchTailscaleNodes() tea.Cmd {
	return func() tea.Msg {
		nodes, _ := tsutil.Nodes(context.Background())
		return TsNodesMsg{Nodes: nodes}
	}
}

// buildItems rebuilds allItems and filteredItems from current hosts + tsNodes.
func (m *Model) buildItems() {
	m.allItems = nil
	for _, h := range m.hosts {
		h := h
		m.allItems = append(m.allItems, item{
			kind:    "ssh",
			ssh:     h,
			display: h.Alias + " " + h.HostName,
		})
	}
	for _, n := range m.tsNodes {
		n := n
		m.allItems = append(m.allItems, item{
			kind:    "ts",
			ts:      n,
			display: n.Name + " " + n.DNS + " " + n.OS,
		})
	}
	m.applyFilter()
}

// applyFilter recomputes filteredItems based on the current input value.
func (m *Model) applyFilter() {
	query := m.input.Value()
	if query == "" {
		m.filteredItems = make([]item, len(m.allItems))
		copy(m.filteredItems, m.allItems)
		m.cursor = 0
		return
	}

	targets := make([]string, len(m.allItems))
	for i, it := range m.allItems {
		targets[i] = it.display
	}

	matches := fuzzy.Find(query, targets)
	m.filteredItems = make([]item, 0, len(matches))
	for _, match := range matches {
		m.filteredItems = append(m.filteredItems, m.allItems[match.Index])
	}
	m.cursor = 0
}

// openForm transitions from list state to form state for the given item.
func (m *Model) openForm(selected item) {
	m.state = stateForm

	// Determine pre-fill values.
	username := ""
	port := 22
	if selected.kind == "ssh" {
		h := selected.ssh
		m.selectedHost = &h
		m.selectedNode = nil
		username = h.User
		port = h.Port
	} else {
		n := selected.ts
		m.selectedHost = nil
		m.selectedNode = &n
		// Username: fall back to OS user.
		username = osCurrentUser()
	}

	// Build form fields.
	userField := textinput.New()
	userField.Placeholder = "username"
	userField.SetValue(username)
	userField.CharLimit = 64
	userField.Focus()

	portField := textinput.New()
	portField.Placeholder = "22"
	portField.SetValue(fmt.Sprintf("%d", port))
	portField.CharLimit = 6

	passField := textinput.New()
	passField.Placeholder = "optional"
	passField.EchoMode = textinput.EchoPassword
	passField.CharLimit = 256

	m.formFields = []formField{
		{label: "Username", input: userField},
		{label: "Port    ", input: portField},
		{label: "Password", input: passField},
	}
	m.formCursor = 0
}

// focusFormField moves focus to the given field index.
func (m *Model) focusFormField(idx int) {
	for i := range m.formFields {
		if i == idx {
			m.formFields[i].input.Focus()
		} else {
			m.formFields[i].input.Blur()
		}
	}
	m.formCursor = idx
}

// submitForm emits a ConnectMsg with form values.
func (m *Model) submitForm() tea.Cmd {
	username := m.formFields[0].input.Value()
	portStr := m.formFields[1].input.Value()
	password := m.formFields[2].input.Value()

	port := 22
	if p, err := fmt.Sscanf(portStr, "%d", &port); p != 1 || err != nil {
		port = 22
	}

	m.active = false

	if m.selectedHost != nil {
		h := *m.selectedHost
		return func() tea.Msg {
			return ConnectMsg{Host: &h, Username: username, Password: password, Port: port}
		}
	}
	if m.selectedNode != nil {
		n := *m.selectedNode
		return func() tea.Msg {
			return ConnectMsg{Node: &n, Username: username, Password: password, Port: port}
		}
	}
	return func() tea.Msg { return CloseConnectMsg{} }
}

// Update handles key events and async messages while the overlay is active.
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	// Handle TsNodesMsg even if not active (it arrives async).
	if tsMsg, ok := msg.(TsNodesMsg); ok {
		m.tsNodes = tsMsg.Nodes
		m.tsLoaded = true
		m.buildItems()
		return m, nil
	}

	// Advance spinner while Tailscale nodes are loading.
	if tickMsg, ok := msg.(spinner.TickMsg); ok {
		if !m.tsLoaded {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(tickMsg)
			return m, cmd
		}
		return m, nil
	}

	if !m.active {
		return m, nil
	}

	// Route to the appropriate state handler.
	if m.state == stateForm {
		return m.updateForm(msg)
	}
	return m.updateList(msg)
}

// updateList handles key events in the host list state.
func (m *Model) updateList(msg tea.Msg) (*Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.Type {
	case tea.KeyEsc:
		m.active = false
		return m, func() tea.Msg { return CloseConnectMsg{} }

	case tea.KeyEnter:
		if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
			selected := m.filteredItems[m.cursor]
			m.openForm(selected)
			return m, nil
		}
		m.active = false
		return m, func() tea.Msg { return CloseConnectMsg{} }

	case tea.KeyDown:
		if m.cursor < len(m.filteredItems)-1 {
			m.cursor++
		}
		return m, nil

	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case tea.KeyRunes:
		ch := keyMsg.String()
		// vim-style navigation when input is empty.
		if m.input.Value() == "" {
			if ch == "j" {
				if m.cursor < len(m.filteredItems)-1 {
					m.cursor++
				}
				return m, nil
			}
			if ch == "k" {
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			}
			if ch == "q" {
				m.active = false
				return m, func() tea.Msg { return CloseConnectMsg{} }
			}
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.applyFilter()
		return m, cmd

	case tea.KeyBackspace:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.applyFilter()
		return m, cmd

	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.applyFilter()
		return m, cmd
	}
}

// updateForm handles key events in the connection detail form state.
func (m *Model) updateForm(msg tea.Msg) (*Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.Type {
	case tea.KeyEsc:
		// Go back to list state.
		m.state = stateList
		m.selectedHost = nil
		m.selectedNode = nil
		return m, nil

	case tea.KeyEnter:
		// Submit the form.
		cmd := m.submitForm()
		return m, cmd

	case tea.KeyTab, tea.KeyDown:
		// Move to next field.
		next := (m.formCursor + 1) % len(m.formFields)
		m.focusFormField(next)
		return m, nil

	case tea.KeyShiftTab, tea.KeyUp:
		// Move to previous field.
		prev := (m.formCursor - 1 + len(m.formFields)) % len(m.formFields)
		m.focusFormField(prev)
		return m, nil

	default:
		// Route keystrokes to the focused field.
		var cmds []tea.Cmd
		for i := range m.formFields {
			var cmd tea.Cmd
			m.formFields[i].input, cmd = m.formFields[i].input.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)
	}
}

// View renders the connect palette overlay.
func (m *Model) View() string {
	if m.state == stateForm {
		return m.viewForm()
	}
	return m.viewList()
}

// viewList renders the host selection list.
func (m *Model) viewList() string {
	boxW := 64
	if m.Width > 0 && boxW > m.Width-4 {
		boxW = m.Width - 4
	}
	maxItems := 16

	var sb strings.Builder

	// Header.
	header := m.Theme.PaletteSelected.Width(boxW - 4).Render("Connect")
	sb.WriteString(header)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", boxW-4))
	sb.WriteString("\n")

	// Input field.
	inputLine := m.Theme.PaletteInput.Width(boxW - 4).Render(m.input.View())
	sb.WriteString(inputLine)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", boxW-4))
	sb.WriteString("\n")

	// Separate filtered items by kind.
	var sshItems, tsItems []item
	var sshIndices, tsIndices []int
	for i, it := range m.filteredItems {
		if it.kind == "ssh" {
			sshIndices = append(sshIndices, i)
			sshItems = append(sshItems, it)
		} else {
			tsIndices = append(tsIndices, i)
			tsItems = append(tsItems, it)
		}
	}

	linesUsed := 0

	// SSH section.
	if len(m.hosts) > 0 || len(sshItems) > 0 {
		sectionHdr := m.Theme.PaletteItem.Width(boxW - 4).Render("SSH Config Hosts")
		sb.WriteString(sectionHdr)
		sb.WriteString("\n")
		linesUsed++

		if len(sshItems) == 0 && m.input.Value() != "" {
			// filtered out — show nothing
		} else if len(sshItems) == 0 {
			empty := m.Theme.PaletteItem.Width(boxW - 4).Render("  No hosts found in ~/.ssh/config")
			sb.WriteString(empty)
			sb.WriteString("\n")
			linesUsed++
		} else {
			for idx, globalIdx := range sshIndices {
				if linesUsed >= maxItems {
					break
				}
				h := sshItems[idx]
				label := "  " + hostLabel(h.ssh, boxW-6)
				var style lipgloss.Style
				if globalIdx == m.cursor {
					style = m.Theme.PaletteSelected.Width(boxW - 4)
				} else {
					style = m.Theme.PaletteItem.Width(boxW - 4)
				}
				sb.WriteString(style.Render(label))
				sb.WriteString("\n")
				linesUsed++
			}
		}
	}

	// Tailscale section.
	showTSSection := m.tsLoaded && len(m.tsNodes) > 0
	showTSLoading := !m.tsLoaded

	if showTSSection || showTSLoading {
		sb.WriteString("\n")

		sectionHdr := m.Theme.PaletteItem.Width(boxW - 4).Render("Tailscale Nodes")
		sb.WriteString(sectionHdr)
		sb.WriteString("\n")
		linesUsed += 2

		if showTSLoading {
			loading := m.Theme.PaletteItem.Width(boxW - 4).Render("  " + m.spinner.View() + " Fetching Tailscale nodes…")
			sb.WriteString(loading)
			sb.WriteString("\n")
			linesUsed++
		} else {
			// Show filtered ts items or all ts items.
			displayItems := tsItems
			displayIndices := tsIndices
			if m.input.Value() == "" {
				// No filter: show all ts nodes (not just filtered).
				displayItems = nil
				displayIndices = nil
				// Walk allItems to find ts entries and their notional global indices.
				// Global index = position in filteredItems (which equals allItems when no filter).
				for i, it := range m.filteredItems {
					if it.kind == "ts" {
						displayIndices = append(displayIndices, i)
						displayItems = append(displayItems, it)
					}
				}
			}

			for idx, globalIdx := range displayIndices {
				if linesUsed >= maxItems {
					break
				}
				n := displayItems[idx].ts
				dot := "○"
				if n.Online {
					dot = "●"
				}
				label := fmt.Sprintf("  %s %-16s %-8s %s", dot, n.Name, n.OS, onlineLabel(n.Online))
				if lipgloss.Width(label) > boxW-4 {
					label = label[:boxW-5] + "…"
				}
				var style lipgloss.Style
				if globalIdx == m.cursor {
					style = m.Theme.PaletteSelected.Width(boxW - 4)
				} else {
					style = m.Theme.PaletteItem.Width(boxW - 4)
				}
				sb.WriteString(style.Render(label))
				sb.WriteString("\n")
				linesUsed++
			}
		}
	}

	// No results at all.
	if len(m.filteredItems) == 0 && m.tsLoaded {
		empty := m.Theme.PaletteItem.Width(boxW - 4).Render("No matching hosts")
		sb.WriteString(empty)
		sb.WriteString("\n")
	}

	// Footer hints.
	sb.WriteString("\n")
	footerStyle := m.Theme.PaletteItem.Copy().Faint(true)
	sb.WriteString(footerStyle.Width(boxW - 4).Render("  enter select · esc close"))

	content := strings.TrimRight(sb.String(), "\n")
	return m.Theme.PaletteBox.Width(boxW).Render(content)
}

// viewForm renders the connection detail form.
func (m *Model) viewForm() string {
	boxW := 64
	if m.Width > 0 && boxW > m.Width-4 {
		boxW = m.Width - 4
	}

	var sb strings.Builder

	// Header.
	header := m.Theme.PaletteSelected.Width(boxW - 4).Render("Connection Details")
	sb.WriteString(header)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", boxW-4))
	sb.WriteString("\n")

	// Host info line.
	hostInfo := ""
	if m.selectedHost != nil {
		hostInfo = fmt.Sprintf("%s (%s)", m.selectedHost.Alias, m.selectedHost.HostName)
	} else if m.selectedNode != nil {
		hostInfo = fmt.Sprintf("%s (Tailscale)", m.selectedNode.Name)
	}
	if hostInfo != "" {
		hostLine := m.Theme.PaletteItem.Width(boxW - 4).Render("  Host:     " + hostInfo)
		sb.WriteString(hostLine)
		sb.WriteString("\n\n")
	}

	// Form fields.
	for i, f := range m.formFields {
		var style lipgloss.Style
		if i == m.formCursor {
			style = m.Theme.PaletteSelected.Width(boxW - 4)
		} else {
			style = m.Theme.PaletteItem.Width(boxW - 4)
		}
		line := fmt.Sprintf("  %s: %s", f.label, f.input.View())
		sb.WriteString(style.Render(line))
		sb.WriteString("\n")
	}

	// Footer hints.
	sb.WriteString("\n")
	footerStyle := m.Theme.PaletteItem.Copy().Faint(true)
	sb.WriteString(footerStyle.Width(boxW - 4).Render("  tab next · enter connect · esc back"))

	content := strings.TrimRight(sb.String(), "\n")
	return m.Theme.PaletteBox.Width(boxW).Render(content)
}

// hostLabel builds the display string for an SSH host entry.
func hostLabel(h SSHHost, maxW int) string {
	label := fmt.Sprintf("%s (%s@%s:%d)", h.Alias, h.User, h.HostName, h.Port)
	if lipgloss.Width(label) > maxW {
		label = label[:maxW-1] + "…"
	}
	return label
}

func onlineLabel(online bool) string {
	if online {
		return "online"
	}
	return "offline"
}

// osCurrentUser returns the current OS username by checking environment variables.
func osCurrentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u := os.Getenv("LOGNAME"); u != "" {
		return u
	}
	return ""
}
