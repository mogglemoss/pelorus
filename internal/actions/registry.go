package actions

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mogglemoss/pelorus/internal/config"
	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

// ActionContext describes in which situation an action is applicable.
type ActionContext uint32

const (
	CtxAlways       ActionContext = iota
	CtxFileSelected ActionContext = iota
	CtxDirSelected  ActionContext = iota
	CtxRemote       ActionContext = iota
	CtxLocal        ActionContext = iota
)

// AppState is the read-only view of the application state passed to action handlers.
type AppState struct {
	// ActivePane is the index of the active pane (0 = left, 1 = right).
	ActivePane int
	// Selected is the currently selected file in the active pane (nil if empty dir).
	Selected *fileinfo.FileInfo
	// ActivePath is the current directory of the active pane.
	ActivePath string
	// InactivePath is the current directory of the inactive pane.
	InactivePath string
	// ShowHidden indicates whether hidden files are being shown.
	ShowHidden bool
}

// Action represents a named, bindable operation.
type Action struct {
	ID          string
	Name        string
	Description string
	Category    string
	Handler     func(state AppState) tea.Cmd
	Context     ActionContext
	Keybinding  string
}

// Registry stores and retrieves actions.
type Registry struct {
	actions []Action
	byID    map[string]Action
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		byID: make(map[string]Action),
	}
}

// Register adds an action to the registry.
// If an action with the same ID already exists it is replaced.
func (r *Registry) Register(a Action) {
	if _, exists := r.byID[a.ID]; exists {
		// Replace in slice too.
		for i, existing := range r.actions {
			if existing.ID == a.ID {
				r.actions[i] = a
				r.byID[a.ID] = a
				return
			}
		}
	}
	r.actions = append(r.actions, a)
	r.byID[a.ID] = a
}

// All returns all registered actions in registration order.
func (r *Registry) All() []Action {
	out := make([]Action, len(r.actions))
	copy(out, r.actions)
	return out
}

// ByID looks up an action by its ID. Returns false if not found.
func (r *Registry) ByID(id string) (Action, bool) {
	a, ok := r.byID[id]
	return a, ok
}

// RegisterCustomActions registers user-defined custom shell actions from config.
func RegisterCustomActions(r *Registry, customs []config.CustomAction) {
	for _, ca := range customs {
		ca := ca // capture loop variable
		ctx := parseContext(ca.Context)
		r.Register(Action{
			ID:          ca.ID,
			Name:        ca.Name,
			Description: ca.Description,
			Category:    ca.Category,
			Context:     ctx,
			Handler:     makeCustomHandler(ca.Command),
		})
	}
}

// ApplyKeybindings overrides action keybindings from config.
// bindings maps actionID -> key; config bindings take precedence over built-in defaults.
func ApplyKeybindings(r *Registry, bindings map[string]string) {
	for actionID, key := range bindings {
		if a, ok := r.byID[actionID]; ok {
			a.Keybinding = key
			r.byID[actionID] = a
			// Also update in slice.
			for i, existing := range r.actions {
				if existing.ID == actionID {
					r.actions[i] = a
					break
				}
			}
		}
	}
}

// BuildKeyMap builds a key -> actionID map from the registry.
// This replaces the hardcoded builtinMap in app.go and incorporates
// any keybinding overrides applied via ApplyKeybindings.
func BuildKeyMap(r *Registry) map[string]string {
	km := make(map[string]string)
	for _, a := range r.All() {
		if a.Keybinding != "" {
			km[a.Keybinding] = a.ID
		}
	}
	return km
}

// parseContext converts a context string from config to an ActionContext value.
func parseContext(s string) ActionContext {
	switch strings.ToLower(s) {
	case "file":
		return CtxFileSelected
	case "dir":
		return CtxDirSelected
	case "remote":
		return CtxRemote
	case "local":
		return CtxLocal
	default:
		return CtxAlways
	}
}

// makeCustomHandler returns a tea.Cmd handler that runs a shell command.
// The command is executed via "sh -c" to support shell pipelines.
func makeCustomHandler(command string) func(AppState) tea.Cmd {
	return func(state AppState) tea.Cmd {
		return func() tea.Msg {
			expanded := expandCommandTemplate(command, state)
			cmd := exec.Command("sh", "-c", expanded)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
			return nil
		}
	}
}

// expandCommandTemplate replaces {path}, {name}, and {dir} placeholders
// in a command string with values derived from the current AppState.
func expandCommandTemplate(command string, state AppState) string {
	path := state.ActivePath
	name := filepath.Base(path)
	dir := state.ActivePath

	if state.Selected != nil {
		path = state.Selected.Path
		name = state.Selected.Name
		if state.Selected.IsDir {
			dir = state.Selected.Path
		} else {
			dir = filepath.Dir(state.Selected.Path)
		}
	}

	r := strings.NewReplacer(
		"{path}", path,
		"{name}", name,
		"{dir}", dir,
	)
	return r.Replace(command)
}
