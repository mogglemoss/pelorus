package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

type session struct {
	LastDir string `json:"last_dir"`
}

func sessionPath() (string, error) {
	return xdg.DataFile("pelorus/session.json")
}

// LoadLastDir reads the last-used directory from the session file.
// Returns an error if the file doesn't exist or has no last_dir recorded.
func LoadLastDir() (string, error) {
	p, err := sessionPath()
	if err != nil {
		return "", fmt.Errorf("session path: %w", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	var s session
	if err := json.Unmarshal(data, &s); err != nil {
		return "", err
	}
	if s.LastDir == "" {
		return "", fmt.Errorf("no last dir recorded")
	}
	return s.LastDir, nil
}

// SaveLastDir writes the last-used directory to the session file.
func SaveLastDir(dir string) error {
	p, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(session{LastDir: dir})
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
