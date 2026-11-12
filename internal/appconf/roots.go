package appconf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Trusted roots are the stack folders this Windows account has set up or
// started. The CLI and the php/composer shims find a stack by walking up from
// the current folder, and a cloned project could ship its own stack.toml or
// php-shims.json naming programs to run; they only use a folder found that way
// when it is listed here (or is Settings.Root).
//
// The list is its own file, not part of Settings, because the app keeps a copy
// of Settings and saves it whole: a root trusted meanwhile from the CLI would be
// lost.

func rootsPath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "roots.json"), nil
}

// TrustedRoots returns the listed roots (not Settings.Root).
func TrustedRoots() []string {
	p, err := rootsPath()
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var roots []string
	if json.Unmarshal(b, &roots) != nil {
		return nil
	}
	return roots
}

// Trusted reports whether root is a listed root or the app's stack.
func Trusted(root string) bool {
	if root == "" {
		return false
	}
	if s, err := Load(); err == nil && SamePath(s.Root, root) {
		return true
	}
	for _, r := range TrustedRoots() {
		if SamePath(r, root) {
			return true
		}
	}
	return false
}

// Trust adds root to the list.
func Trust(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	roots := TrustedRoots()
	for _, r := range roots {
		if SamePath(r, abs) {
			return nil
		}
	}
	p, err := rootsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(append(roots, abs), "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// SamePath compares folder paths as Windows does: cleaned, ignoring case and a
// trailing separator.
func SamePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	clean := func(p string) string { return strings.ToLower(strings.TrimRight(filepath.Clean(p), `\/`)) }
	return clean(a) == clean(b)
}
