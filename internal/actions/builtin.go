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

// DuplicateSelectedMsg duplicates the selected item(s) with a "copy" suffix.
type DuplicateSelectedMsg struct{}

// SymlinkSelectedMsg creates a symlink to the selected item in the other pane.
type SymlinkSelectedMsg struct{}

// ExtractArchiveMsg extracts the selected archive into the other pane.
type ExtractArchiveMsg struct{}

// ChmodSelectedMsg opens the chmod editor for the selected item.
type ChmodSelectedMsg struct{}

// QuickInfoMsg opens an info modal with rich metadata for the selected item.
type QuickInfoMsg struct{}

// SelectByGlobMsg prompts for a glob pattern and multi-selects matching items.
type SelectByGlobMsg struct{}

// CalcDirSizeMsg kicks off a recursive size calculation for the cursor directory.
type CalcDirSizeMsg struct{}

// ShowVersionMsg flashes the pelorus version in the status bar.
type ShowVersionMsg struct{}

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

// RevealInFinderMsg tells the app to reveal the selected item in macOS Finder.
type RevealInFinderMsg struct{}

// SetMarkMsg tells the app to enter mark-set mode (waiting for key).
type SetMarkMsg struct{}

// JumpMarkMsg tells the app to enter mark-jump mode (waiting for key).
type JumpMarkMsg struct{}

// OpenSearchMsg tells the app to open the recursive search overlay.
type OpenSearchMsg struct{}

// PreviewSearchMsg tells the app to open the inline search bar in the preview pane.
type PreviewSearchMsg struct{}

// StartFilterMsg tells the app to start fuzzy-filter mode on the active pane.
type StartFilterMsg struct{}

// OpenShellMsg tells the app to drop into an interactive shell in the current directory.
type OpenShellMsg struct{}

// RunCommandMsg tells the app to run a shell command on the selected file.
type RunCommandMsg struct{}

// QuickLookMsg tells the app to open the selected file in macOS Quick Look.
type QuickLookMsg struct{}

// OpenWithMsg tells the app to open the selected file with the OS default application.
type OpenWithMsg struct{}

// ResizeSplitMsg tells the app to grow or shrink the left pane.
type ResizeSplitMsg struct{ Delta int } // negative = shrink, positive = grow

// DisconnectMsg tells the app to disconnect the active pane's remote session
// and revert it to the local provider.
type DisconnectMsg struct{}

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
			ID:               "file.delete",
			Name:             "Delete",
			Description:      "Permanently delete the selected file or directory",
			Category:         "File",
			Context:          CtxFileSelected,
			Keybinding:       "d",
			ExtraKeybindings: []string{"shift+f8"},
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
			ID:               "file.new-file",
			Name:             "New File",
			Description:      "Create a new file in the current directory",
			Category:         "File",
			Context:          CtxAlways,
			Keybinding:       "n",
			ExtraKeybindings: []string{"shift+f7"},
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return NewFileMsg{} }
			},
		},
		{
			ID:               "file.new-dir",
			Name:             "New Directory",
			Description:      "Create a new directory in the current directory",
			Category:         "File",
			Context:          CtxAlways,
			Keybinding:       "N",
			ExtraKeybindings: []string{"f7"},
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return NewDirMsg{} }
			},
		},
		{
			ID:               "file.copy",
			Name:             "Copy to Other Pane",
			Description:      "Copy selected file to the other pane's directory",
			Category:         "File",
			Context:          CtxFileSelected,
			Keybinding:       "C",
			ExtraKeybindings: []string{"f5"},
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return CopySelectedMsg{} }
			},
		},
		{
			ID:               "file.move",
			Name:             "Move to Other Pane",
			Description:      "Move selected file to the other pane's directory",
			Category:         "File",
			Context:          CtxFileSelected,
			Keybinding:       "M",
			ExtraKeybindings: []string{"f6"},
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
			ID:          "view.preview-search",
			Name:        "Search in Preview",
			Description: "Open inline search bar in the preview pane",
			Category:    "View",
			Context:     CtxAlways,
			Keybinding:  "ctrl+/",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return PreviewSearchMsg{} }
			},
		},
		{
			ID:          "nav.filter",
			Name:        "Filter Pane",
			Description: "Fuzzy-filter the active pane's contents",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "/",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return StartFilterMsg{} }
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
			Description: "Type a path to navigate to (tab completes filenames)",
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
			ID:               "file.open-editor",
			Name:             "Open in Editor",
			Description:      "Open selected file in the configured editor",
			Category:         "File",
			Context:          CtxFileSelected,
			Keybinding:       "f4",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return OpenEditorMsg{} }
			},
		},
		{
			ID:               "file.trash",
			Name:             "Move to Trash",
			Description:      "Move selected item(s) to the OS trash",
			Category:         "File",
			Context:          CtxFileSelected,
			Keybinding:       "f8",
			ExtraKeybindings: []string{"delete"},
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return TrashMsg{} }
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
		{
			ID:          "file.reveal-in-finder",
			Name:        "Reveal in Finder",
			Description: "Reveal selected item in macOS Finder (local panes only)",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "ctrl+r",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return RevealInFinderMsg{} }
			},
		},
		{
			ID:          "nav.set-mark",
			Name:        "Set Mark",
			Description: "Mark current directory with a key (then press any key)",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "m",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return SetMarkMsg{} }
			},
		},
		{
			ID:          "nav.jump-mark",
			Name:        "Jump to Mark",
			Description: "Jump to a marked directory (then press the mark key)",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "'",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return JumpMarkMsg{} }
			},
		},
		{
			ID:          "nav.search",
			Name:        "Search Files",
			Description: "Recursively search files in current directory",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "ctrl+f",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return OpenSearchMsg{} }
			},
		},
		{
			ID:          "app.shell",
			Name:        "Open Shell",
			Description: "Drop into an interactive shell in the current directory",
			Category:    "App",
			Context:     CtxAlways,
			Keybinding:  "S",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return OpenShellMsg{} }
			},
		},
		{
			ID:          "file.run-command",
			Name:        "Run Command",
			Description: "Run a shell command with the selected file as argument",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "!",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return RunCommandMsg{} }
			},
		},
		{
			ID:          "file.quick-look",
			Name:        "Quick Look",
			Description: "Preview file with macOS Quick Look",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "Q",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return QuickLookMsg{} }
			},
		},
		{
			ID:          "file.open-with",
			Name:        "Open With Default App",
			Description: "Open selected file with the OS default application",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "o",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return OpenWithMsg{} }
			},
		},
		{
			ID:          "view.split-grow",
			Name:        "Grow Left Pane",
			Description: "Increase the left pane width",
			Category:    "View",
			Context:     CtxAlways,
			Keybinding:  ">",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return ResizeSplitMsg{Delta: +5} }
			},
		},
		{
			ID:          "view.split-shrink",
			Name:        "Shrink Left Pane",
			Description: "Decrease the left pane width",
			Category:    "View",
			Context:     CtxAlways,
			Keybinding:  "<",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return ResizeSplitMsg{Delta: -5} }
			},
		},
		{
			ID:          "nav.disconnect",
			Name:        "Disconnect Remote Pane",
			Description: "Close SSH session and revert active pane to local filesystem",
			Category:    "Navigation",
			Context:     CtxAlways,
			Keybinding:  "ctrl+d",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return DisconnectMsg{} }
			},
		},
		{
			ID:          "file.duplicate",
			Name:        "Duplicate",
			Description: "Duplicate selected item in the same directory",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "D",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return DuplicateSelectedMsg{} }
			},
		},
		{
			ID:          "file.symlink",
			Name:        "Symlink to Other Pane",
			Description: "Create a symbolic link to the selected item in the other pane",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "L",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return SymlinkSelectedMsg{} }
			},
		},
		{
			ID:          "file.extract",
			Name:        "Extract Archive",
			Description: "Extract the selected archive into the other pane",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "x",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return ExtractArchiveMsg{} }
			},
		},
		{
			ID:          "file.chmod",
			Name:        "Change Permissions",
			Description: "Edit the unix permission bits of the selected item",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "cm",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return ChmodSelectedMsg{} }
			},
		},
		{
			ID:          "file.info",
			Name:        "Quick Info",
			Description: "Show rich metadata for the selected item",
			Category:    "File",
			Context:     CtxFileSelected,
			Keybinding:  "i",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return QuickInfoMsg{} }
			},
		},
		{
			ID:          "file.select-glob",
			Name:        "Select by Pattern",
			Description: "Multi-select files matching a glob pattern",
			Category:    "File",
			Context:     CtxAlways,
			Keybinding:  "V",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return SelectByGlobMsg{} }
			},
		},
		{
			ID:          "nav.dir-size",
			Name:        "Calculate Directory Size",
			Description: "Compute recursive size of the cursor directory",
			Category:    "Navigation",
			Context:     CtxFileSelected,
			Keybinding:  "Z",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return CalcDirSizeMsg{} }
			},
		},
		{
			ID:          "app.version",
			Name:        "Show Version",
			Description: "Display pelorus version and build info",
			Category:    "App",
			Context:     CtxAlways,
			Keybinding:  "",
			Handler: func(_ AppState) tea.Cmd {
				return func() tea.Msg { return ShowVersionMsg{} }
			},
		},
	}

	for _, a := range builtins {
		r.Register(a)
	}
}
