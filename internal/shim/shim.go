// Package shim lets plain `php` and `composer` in a terminal run the PHP a
// site is served with. devstack installs a small php.exe and composer.exe in
// <app>\shims, first on PATH. Each looks up the folder it was started in, in a
// map devstack writes on every render, and runs that site's PHP build with
// devstack's php.ini; outside every site it runs the default PHP.
//
// This package is shared by devstack (which writes the map) and the shim
// executable (which reads it), so it stays small: no config, model or
// discovery code.
package shim

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MapFile is the map's path under the stack root.
var MapFile = filepath.Join("data", "devstack", "php-shims.json")

// Map is what the shims need to pick a PHP for a folder.
type Map struct {
	Default string           `json:"default"` // key into PHP, "8.3"
	PHP     map[string]Build `json:"php"`
	Sites   []Site           `json:"sites"`
}

// Build is one PHP version in use.
type Build struct {
	Version string `json:"version"` // "8.5.12"
	Exe     string `json:"exe"`     // php.exe
	INI     string `json:"ini"`     // the php.ini devstack generates for it
}

// Site is a project folder and the PHP key it runs on.
type Site struct {
	Name string `json:"name"`
	Dir  string `json:"dir"`
	PHP  string `json:"php"`
}

// Write saves m to root's map file, leaving the file alone when unchanged.
func Write(root string, m Map) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	path := filepath.Join(root, MapFile)
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, b) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load reads root's map file.
func Load(root string) (Map, error) {
	var m Map
	b, err := os.ReadFile(filepath.Join(root, MapFile))
	if errors.Is(err, os.ErrNotExist) {
		return m, fmt.Errorf("devstack hasn't set up PHP in %s yet; open the devstack app, or run `devstack render`", root)
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("%s: %w", filepath.Join(root, MapFile), err)
	}
	return m, nil
}

// Pick returns the PHP for a command started in dir: the PHP of the site
// that holds dir, else the default. force ("8.3", from DEVSTACK_PHP) wins
// over both. site is nil outside every site.
func (m Map) Pick(dir, force string) (Build, *Site, error) {
	if force != "" {
		for key, b := range m.PHP {
			if key == force || b.Version == force {
				return b, nil, nil
			}
		}
		return Build{}, nil, fmt.Errorf("DEVSTACK_PHP=%s, but devstack runs PHP %s", force, strings.Join(m.keys(), ", "))
	}
	if s := m.siteOf(dir); s != nil {
		if b, ok := m.PHP[s.PHP]; ok {
			return b, s, nil
		}
	}
	b, ok := m.PHP[m.Default]
	if !ok {
		return Build{}, nil, errors.New("devstack has no PHP installed; add one in the devstack app")
	}
	return b, nil, nil
}

func (m Map) keys() []string {
	var out []string
	for k := range m.PHP {
		out = append(out, k)
	}
	return out
}

// siteOf finds the site whose folder holds dir. Paths are compared first;
// then, for folders reached through a junction or symlink (a project linked
// into www, or www linked elsewhere), the folders themselves.
func (m Map) siteOf(dir string) *Site {
	dir = filepath.Clean(dir)
	best := -1
	for i, s := range m.Sites {
		if within(dir, s.Dir) && (best < 0 || len(s.Dir) > len(m.Sites[best].Dir)) {
			best = i
		}
	}
	if best >= 0 {
		return &m.Sites[best]
	}
	infos := make([]os.FileInfo, len(m.Sites))
	for i, s := range m.Sites {
		infos[i], _ = os.Stat(s.Dir)
	}
	for d := dir; ; d = filepath.Dir(d) {
		if fi, err := os.Stat(d); err == nil {
			for i, si := range infos {
				if si != nil && os.SameFile(fi, si) {
					return &m.Sites[i]
				}
			}
		}
		if filepath.Dir(d) == d {
			return nil
		}
	}
}

// within reports whether dir is base or inside it, ignoring case as Windows does.
func within(dir, base string) bool {
	dir, base = strings.ToLower(dir), strings.ToLower(filepath.Clean(base))
	return dir == base || strings.HasPrefix(dir, strings.TrimSuffix(base, string(filepath.Separator))+string(filepath.Separator))
}

// FindRoot is the stack whose map a shim started in dir should use:
// DEVSTACK_ROOT, else the nearest folder above dir holding a map, else
// fallback (the app's stack).
func FindRoot(dir, env, fallback string) string {
	if env != "" {
		return env
	}
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, MapFile)); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return fallback
		}
	}
}

// Dir is the shims folder that belongs to the running devstack executable
// (<app>\shims), or "" when it has none (a development build without them).
func Dir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	d := filepath.Join(filepath.Dir(exe), "shims")
	if _, err := os.Stat(filepath.Join(d, "php.exe")); err != nil {
		return ""
	}
	return d
}
