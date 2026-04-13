package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/mogglemoss/pelorus/internal/actions"
	"github.com/mogglemoss/pelorus/internal/app"
	"github.com/mogglemoss/pelorus/internal/config"
	"github.com/mogglemoss/pelorus/internal/provider/local"
	"github.com/mogglemoss/pelorus/internal/theme"
)

var cfgFile string
var themeName string

var rootCmd = &cobra.Command{
	Use:     "pelorus [path]",
	Short:   "Pelorus — an opinionated TUI file manager",
	Version: "1.0.0",
	Long: `Pelorus is a dual-pane TUI file manager with a retrofuture subaquatic aesthetic.

Usage:
  pelorus           # start in current directory
  pelorus ~/path    # start in specified directory`,
	Args:         cobra.MaximumNArgs(1),
	RunE:         run,
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "f", "", "config file (default: XDG config dir)")
	rootCmd.PersistentFlags().StringVarP(&themeName, "theme", "t", "", "theme override (pelorus, gruvbox, nord, light, dracula, omarchy)")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Resolve starting directory.
	startDir := "."
	if len(args) > 0 {
		startDir = args[0]
	}

	// Load config first so we can read start_dir from it.
	var err error
	var cfg *config.Config
	if cfgFile != "" {
		cfg, err = config.LoadFrom(cfgFile)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		cfg = config.Defaults()
	}

	// If no path argument was given, apply the config start_dir.
	if len(args) == 0 && cfg.General.StartDir != "" && cfg.General.StartDir != "." {
		startDir = cfg.General.StartDir
	}

	// "last" restores the previous session directory.
	if startDir == "last" {
		if last, lerr := config.LoadLastDir(); lerr == nil {
			startDir = last
		} else {
			startDir = "."
		}
	}

	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return fmt.Errorf("cannot resolve path %q: %w", startDir, err)
	}

	info, err := os.Stat(absStart)
	if err != nil {
		return fmt.Errorf("path %q does not exist: %w", absStart, err)
	}
	if !info.IsDir() {
		absStart = filepath.Dir(absStart)
	}

	// Set up provider.
	prov := local.New()

	// Set up theme — flag takes precedence over config.
	tName := cfg.Theme.Name
	if themeName != "" {
		tName = themeName
	}
	t := theme.Get(tName)

	// Set up action registry.
	reg := actions.NewRegistry()
	actions.RegisterBuiltins(reg)
	actions.RegisterCustomActions(reg, cfg.Actions.Custom)
	actions.ApplyKeybindings(reg, cfg.Keybindings)

	// Build the app model.
	model := app.New(absStart, absStart, prov, reg, cfg, &t)

	// Run the Bubbletea program.
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	finalModel, runErr := p.Run()
	if runErr != nil {
		return fmt.Errorf("pelorus: %w", runErr)
	}

	// Save the last-used directory for "start_dir = last".
	if m, ok := finalModel.(*app.Model); ok {
		_ = config.SaveLastDir(m.ActivePath())
	}

	return nil
}
