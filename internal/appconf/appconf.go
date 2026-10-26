// Package appconf holds per-user app settings in %LOCALAPPDATA%\devstack:
// which stack root the app manages, and app preferences. stack.toml stays
// the source of truth for the stack itself.
package appconf

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Settings struct {
	// Root is the stack folder (holding stack.toml). Empty until first-run setup.
	Root string `json:"root"`
	// StartServers starts the stack whenever the app starts.
	StartServers bool `json:"start_servers"`
}

// Dir is %LOCALAPPDATA%\devstack, or DEVSTACK_APPDATA when set (tests, previews).
func Dir() (string, error) {
	if d := os.Getenv("DEVSTACK_APPDATA"); d != "" {
		return d, nil
	}
	base, err := os.UserCacheDir() // %LOCALAPPDATA% on Windows
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "devstack"), nil
}

func path() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "app.json"), nil
}

// Load returns saved settings, or defaults if none are saved yet.
func Load() (Settings, error) {
	s := Settings{StartServers: true}
	p, err := path()
	if err != nil {
		return s, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(b, &s)
}

func Save(s Settings) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
