package actions

import (
	tea "github.com/charmbracelet/bubbletea"
)

// --- Message types for built-in actions ---
// The app model listens for these and performs the actual state mutation.

// NavMsg is emitted by navigation actions.
type NavMsg struct{ Dir string } // "up", "down", "enter", "parent"

// SwitchPaneMsg tells the app to toggle the active pane.
type SwitchPaneMsg struct{}

// ToggleHiddenMsg tells the app to toggle hidden-file visibility.
type ToggleHiddenMsg struct{}

// DeleteSelectedMsg tells the app to initiate deletion of the selected file.
type DeleteSelectedMsg struct{}

// RenameSelectedMsg tells the app to initiate rename of the selected file.
type RenameSelectedMsg struct{}

// NewFileMsg tells the app to initiate new-file creation.
type NewFileMsg struct{}

// NewDirMsg tells the app to initiate new-directory creation.
type NewDirMsg struct{}

// CopySelectedMsg tells the app to copy selected file to the other pane.
type CopySelectedMsg struct{}

// MoveSelectedMsg tells the app to move selected file to the other pane.
type MoveSelectedMsg struct{}

// OpenPaletteMsg tells the app to open the command palette.
type OpenPaletteMsg struct{}

// QuitMsg tells the app to quit.
type QuitMsg struct{}

// TogglePreviewMsg tells the app to toggle the preview panel.
type TogglePreviewMsg struct{}

// OpenJumpMsg tells the app to open the jump list overlay.
type OpenJumpMsg struct{}

// BookmarkMsg tells the app to bookmark the current directory.
type BookmarkMsg struct{}

// OpenQueueMsg tells the app to open the job queue overlay.
type OpenQueueMsg struct{}

// GoHomeMsg tells the app to navigate the active pane to the home directory.
type GoHomeMsg struct{}

// GotoPathMsg tells the app to open the go-to-path prompt in the active pane.
type GotoPathMsg struct{}

// OpenConnectMsg tells the app to open the SSH connect palette.
type OpenConnectMsg struct{}

// OpenHelpMsg tells the app to open the keybinding help overlay.
type OpenHelpMsg struct{}

// ToggleSelectMsg tells the app to mark/unmark the cursor item for batch ops.
type ToggleSelectMsg struct{}

// OpenEditorMsg tells the app to open the selected file in the configured editor.
type OpenEditorMsg struct{}

// TrashMsg tells the app to move selected item(s) to the OS trash.
type TrashMsg struct{}

// CycleSortMsg tells the app to advance the sort mode on the active pane.
type CycleSortMsg struct{}

// CopyPathMsg tells the app to copy the selected item's full path to clipboard.
type CopyPathMsg struct{}

// CopyFilenameMsg tells the app to copy the selected item's filename to clipboard.
type CopyFilenameMsg struct{}

// BulkRenameMsg tells the app to bulk-rename all selected items.
type BulkRenameMsg struct{}

// RegisterBuiltins registers the standard set of built-in actions.
func RegisterBuiltins(r *Registry) {
	builtins := []Action{
		{
			ID:          "nav.up",
			Name:        "Move Up",
			Description: "Move cursor up",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "k",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return NavMsg{Dir: "up"} }
			},
		},
		{
			ID:          "nav.down",
			Name:        "Move Down",
			Description: "Move cursor down",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "j",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return NavMsg{Dir: "down"} }
			},
		},
		{
			ID:          "nav.enter",
			Name:        "Enter / Open",
			Description: "Enter directory or open file",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "l",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return NavMsg{Dir: "enter"} }
			},
		},
		{
			ID:          "nav.parent",
			Name:        "Go to Parent",
			Description: "Navigate to parent directory",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "h",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return NavMsg{Dir: "parent"} }
			},
		},
		{
			ID:          "nav.switch-pane",
			Name:        "Switch Pane",
			Description: "Switch to the other pane",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "tab",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return SwitchPaneMsg{} }
			},
		},
		{
			ID:          "view.toggle-hidden",
			Name:        "Toggle Hidden Files",
			Description: "Show or hide dotfiles",
			Category:    "View",
			Context:     CtxAlways,
			Keybinding:  ".",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return ToggleHiddenMsg{} }
			},
		},
		{
			ID:          "file.delete",
			Name:        "Delete",
			Description: "Delete the selected file or directory",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "d",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return DeleteSelectedMsg{} }
			},
		},
		{
			ID:          "file.rename",
			Name:        "Rename",
			Description: "Rename the selected file or directory",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "r",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return RenameSelectedMsg{} }
			},
		},
		{
			ID:          "file.new-file",
			Name:        "New File",
			Description: "Create a new file in the current directory",
			Category:    "File",
			Context:     CtxAlways,
			Keybinding:  "n",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return NewFileMsg{} }
			},
		},
		{
			ID:          "file.new-dir",
			Name:        "New Directory",
			Description: "Create a new directory in the current directory",
			Category:    "File",
			Context:     CtxAlways,
			Keybinding:  "N",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return NewDirMsg{} }
			},
		},
		{
			ID:          "file.copy",
			Name:        "Copy to Other Pane",
			Description: "Copy selected file to the other pane's directory",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "C",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return CopySelectedMsg{} }
			},
		},
		{
			ID:          "file.move",
			Name:        "Move to Other Pane",
			Description: "Move selected file to the other pane's directory",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "m",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return MoveSelectedMsg{} }
			},
		},
		{
			ID:          "palette.open",
			Name:        "Command Palette",
			Description: "Open the command palette",
			Category:    "App",
			Context:     CtxAlways,
			Keybinding:  "ctrl+p",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return OpenPaletteMsg{} }
			},
		},
		{
			ID:          "view.toggle-preview",
			Name:        "Toggle Preview",
			Description: "Show or hide the preview panel",
			Category:    "View",
			Context:     CtxAlways,
			Keybinding:  "p",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return TogglePreviewMsg{} }
			},
		},
		{
			ID:          "app.quit",
			Name:        "Quit",
			Description: "Quit Pelorus",
			Category:    "App",
			Context:     CtxAlways,
			Keybinding:  "q",
			Handler: func(_ AppState) tea.Cmd {
				return tea.Quit
			},
		},
		{
			ID:          "nav.jump",
			Name:        "Jump List",
			Description: "Open the frecency jump list",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "g",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return OpenJumpMsg{} }
			},
		},
		{
			ID:          "nav.bookmark",
			Name:        "Bookmark Directory",
			Description: "Bookmark the current directory",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "B",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return BookmarkMsg{} }
			},
		},
		{
			ID:          "nav.home",
			Name:        "Go Home",
			Description: "Navigate to home directory",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "~",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return GoHomeMsg{} }
			},
		},
		{
			ID:          "nav.goto",
			Name:        "Go to Path",
			Description: "Type a path to navigate to",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "ctrl+l",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return GotoPathMsg{} }
			},
		},
		{
			ID:          "view.jobs",
			Name:        "Job Queue",
			Description: "Open the background job queue overlay",
			Category:    "View",
			Context:     CtxAlways,
			Keybinding:  "J",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return OpenQueueMsg{} }
			},
		},
		{
			ID:          "nav.connect",
			Name:        "Connect to Host",
			Description: "Open SFTP connection to an SSH host",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "c",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return OpenConnectMsg{} }
			},
		},
		{
			ID:          "app.help",
			Name:        "Help",
			Description: "Show keybinding reference",
			Category:    "App",
			Context:     CtxAlways,
			Keybinding:  "?",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return OpenHelpMsg{} }
			},
		},
		{
			ID:          "file.select",
			Name:        "Toggle Selection",
			Description: "Mark or unmark item for batch operations",
			Category:    "File",
			Context:     CtxAlways,
			Keybinding:  " ",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return ToggleSelectMsg{} }
			},
		},
		{
			ID:          "file.open-editor",
			Name:        "Open in Editor",
			Description: "Open selected file in the configured editor",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "f4",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return OpenEditorMsg{} }
			},
		},
		{
			ID:          "file.copy-f5",
			Name:        "Copy (F5)",
			Description: "Copy selected item to the other pane",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "f5",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return CopySelectedMsg{} }
			},
		},
		{
			ID:          "file.move-f6",
			Name:        "Move (F6)",
			Description: "Move selected item to the other pane",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "f6",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return MoveSelectedMsg{} }
			},
		},
		{
			ID:          "file.new-dir-f7",
			Name:        "New Directory (F7)",
			Description: "Create a new directory in the current pane",
			Category:    "File",
			Context:     CtxAlways,
			Keybinding:  "f7",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return NewDirMsg{} }
			},
		},
		{
			ID:          "file.new-file-sf7",
			Name:        "New File (⇧F7)",
			Description: "Create a new file in the current pane",
			Category:    "File",
			Context:     CtxAlways,
			Keybinding:  "shift+f7",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return NewFileMsg{} }
			},
		},
		{
			ID:          "file.trash",
			Name:        "Move to Trash (F8)",
			Description: "Move selected item(s) to the OS trash",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "f8",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return TrashMsg{} }
			},
		},
		{
			ID:          "file.delete-sf8",
			Name:        "Delete Permanently (⇧F8)",
			Description: "Permanently delete selected item(s)",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "shift+f8",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return DeleteSelectedMsg{} }
			},
		},
		{
			ID:          "file.sort",
			Name:        "Cycle Sort",
			Description: "Cycle sort mode: name → size → date → extension",
			Category:    "View",
			Context:     CtxAlways,
			Keybinding:  "s",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return CycleSortMsg{} }
			},
		},
		{
			ID:          "file.copy-path",
			Name:        "Copy Path",
			Description: "Copy full path of selected item to clipboard",
			Category:    "File",
			Context:     CtxAlways,
			Keybinding:  "y",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return CopyPathMsg{} }
			},
		},
		{
			ID:          "file.copy-name",
			Name:        "Copy Filename",
			Description: "Copy filename of selected item to clipboard",
			Category:    "File",
			Context:     CtxAlways,
			Keybinding:  "Y",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return CopyFilenameMsg{} }
			},
		},
		{
			ID:          "file.bulk-rename",
			Name:        "Bulk Rename",
			Description: "Rename multiple selected items",
			Category:    "File",
			Context:     CtxAlways,
			Keybinding:  "R",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return BulkRenameMsg{} }
			},
		},
	}

	for _, a := range builtins {
		r.Register(a)
	}
}
