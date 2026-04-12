package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
)

// Config is the root configuration structure.
type Config struct {
	General     GeneralConfig     `toml:"general"`
	Layout      LayoutConfig      `toml:"layout"`
	Theme       ThemeConfig       `toml:"theme"`
	Keybindings map[string]string `toml:"keybindings"` // actionID -> key
	Actions     ActionsConfig     `toml:"actions"`
}

// GeneralConfig holds general application settings.
type GeneralConfig struct {
	StartDir      string `toml:"start_dir"`
	ShowHidden    bool   `toml:"show_hidden"`
	ConfirmDelete bool   `toml:"confirm_delete"`
	Editor        string `toml:"editor"`
}

// LayoutConfig holds layout settings.
type LayoutConfig struct {
	Ratio        string `toml:"ratio"`
	ShowPreview  bool   `toml:"show_preview"`
	PreviewWidth int    `toml:"preview_width"`
}

// ThemeConfig holds theme settings.
type ThemeConfig struct {
	Name string `toml:"name"`
}

// ActionsConfig holds custom action definitions.
type ActionsConfig struct {
	Custom []CustomAction `toml:"custom"`
}

// CustomAction defines a user-configured shell action.
type CustomAction struct {
	ID          string `toml:"id"`
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Category    string `toml:"category"`
	Command     string `toml:"command"`
	// Context is one of: "always", "file", "dir", "remote", "local"
	Context string `toml:"context"`
}

// defaultConfigPath returns the XDG config path for pelorus/config.toml.
// Returns empty string on error.
func defaultConfigPath() string {
	p, err := xdg.ConfigFile("pelorus/config.toml")
	if err != nil {
		return ""
	}
	return p
}

// Load loads configuration from the XDG config dir.
// If the file doesn't exist, it writes a default config file first,
// then returns the default Config struct.
func Load() (*Config, error) {
	configPath := defaultConfigPath()
	return LoadFrom(configPath)
}

// LoadFrom loads configuration from the specified path.
// If path is empty, returns defaults without writing anything.
// If the file doesn't exist, it writes the default config template first.
func LoadFrom(path string) (*Config, error) {
	cfg := Defaults()

	if path == "" {
		return cfg, nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Write the default config file so the user has a fully-commented template.
		if err := os.MkdirAll(filepath.Dir(path), 0755); err == nil {
			_ = os.WriteFile(path, []byte(GenerateDefaultConfig()), 0644)
		}
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	if _, err := toml.Decode(string(data), cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// Save writes the config to the XDG config dir.
func Save(cfg *Config) error {
	configPath := defaultConfigPath()
	if configPath == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	f, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(cfg)
}
