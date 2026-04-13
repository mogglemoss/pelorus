package cmd

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/mogglemoss/pelorus/internal/actions"
	"github.com/mogglemoss/pelorus/internal/app"
	"github.com/mogglemoss/pelorus/internal/config"
	"github.com/mogglemoss/pelorus/internal/demo"
	"github.com/mogglemoss/pelorus/internal/provider/local"
	"github.com/mogglemoss/pelorus/internal/quotes"
	"github.com/mogglemoss/pelorus/internal/theme"
)

var cfgFile string
var themeName string
var demoMode bool

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
	rootCmd.PersistentFlags().StringVarP(&themeName, "theme", "t", "", "theme override (haruspex, pelorus, gruvbox, nord, light, dracula, catppuccin, omarchy)")
	rootCmd.PersistentFlags().BoolVar(&demoMode, "demo", false, "start with a sandboxed demo filesystem (for recordings and screenshots)")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Demo mode: build a sandboxed temp filesystem and use it as the start dir.
	if demoMode {
		demoRoot, rawCleanup, err := demo.Setup()
		if err != nil {
			return fmt.Errorf("demo setup: %w", err)
		}
		// Wrap in sync.Once so cleanup is safe to call from both defer and the
		// signal handler without risk of double-removal.
		var once sync.Once
		cleanup := func() { once.Do(rawCleanup) }
		defer cleanup()

		// Ensure ~/pelorus-demo is removed even when the process is killed with
		// ctrl+c or SIGTERM (os.Exit skips deferred calls).
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			cleanup()
			os.Exit(0)
		}()

		// Point into the axiom project inside the demo root.
		args = []string{filepath.Join(demoRoot, "axiom")}
	}

	// Resolve starting directory.
	startDir := "."
	if len(args) > 0 {
		startDir = args[0]
		// Handle pelorus:// URL scheme: strip the scheme and URL-decode the path.
		// e.g. pelorus:///Users/scott/Documents → /Users/scott/Documents
		if strings.HasPrefix(startDir, "pelorus://") {
			raw := strings.TrimPrefix(startDir, "pelorus://")
			if decoded, err := url.PathUnescape(raw); err == nil {
				startDir = decoded
			} else {
				startDir = raw
			}
		}
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
		tea.WithMouseAllMotion(),
	)

	finalModel, runErr := p.Run()
	if runErr != nil {
		return fmt.Errorf("pelorus: %w", runErr)
	}

	// Save the last-used directory for "start_dir = last".
	if m, ok := finalModel.(*app.Model); ok {
		_ = config.SaveLastDir(m.ActivePath())
	}

	fmt.Println(quotes.Random())
	return nil
}
