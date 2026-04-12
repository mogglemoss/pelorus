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

var rootCmd = &cobra.Command{
	Use:   "pelorus [path]",
	Short: "Pelorus — an opinionated TUI file manager",
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

	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return fmt.Errorf("cannot resolve path %q: %w", startDir, err)
	}

	info, err := os.Stat(absStart)
	if err != nil {
		return fmt.Errorf("path %q does not exist: %w", absStart, err)
	}
	if !info.IsDir() {
		// If a file is given, use its parent directory.
		absStart = filepath.Dir(absStart)
	}

	// Load config — use alternate path if --config was supplied.
	var cfg *config.Config
	if cfgFile != "" {
		cfg, err = config.LoadFrom(cfgFile)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		// Non-fatal; use defaults.
		cfg = config.Defaults()
	}

	// Set up provider.
	prov := local.New()

	// Set up theme.
	t := theme.Get(cfg.Theme.Name)

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

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("pelorus: %w", err)
	}
	return nil
}
