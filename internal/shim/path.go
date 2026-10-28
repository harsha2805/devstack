package shim

import (
	"os"
	"path/filepath"
	"strings"
)

// PathStatus is where the shims stand on the PATH new terminals get: the
// system PATH, then the user PATH.
type PathStatus struct {
	Dir string `json:"dir"` // the shims folder
	// OnPath is true when Dir is on the system or user PATH.
	OnPath bool `json:"on_path"`
	// Blocker is a folder ahead of Dir with its own php.exe, which terminals
	// run instead of the shim. Empty when the shim comes first.
	Blocker string `json:"blocker,omitempty"`
	// BlockerSystem is true when Blocker is on the system PATH, which comes
	// before every user entry, so moving Dir needs administrator rights.
	BlockerSystem bool `json:"blocker_system,omitempty"`
}

// OK reports whether `php` in a new terminal runs the shim.
func (s PathStatus) OK() bool { return s.OnPath && s.Blocker == "" }

// Problem says what is wrong, for people; "" when OK.
func (s PathStatus) Problem() string {
	switch {
	case s.Blocker != "":
		where := "your user PATH"
		if s.BlockerSystem {
			where = "the system PATH"
		}
		return "Terminals run the PHP in " + s.Blocker + " (on " + where + ") instead of each site's PHP."
	case !s.OnPath:
		return "devstack's php and composer (" + s.Dir + ") aren't on PATH, so terminals don't use each site's PHP."
	}
	return ""
}

// CheckPath works out s for dir from the raw system and user PATH values
// (already expanded).
func CheckPath(dir, system, user string) PathStatus {
	s := PathStatus{Dir: dir}
	scan := func(path string, sys bool) bool {
		for _, e := range SplitPath(path) {
			if samePath(e, dir) {
				s.OnPath = true
				return true
			}
			if s.Blocker == "" && hasPHP(e) {
				s.Blocker, s.BlockerSystem = e, sys
			}
		}
		return false
	}
	if !scan(system, true) {
		scan(user, false)
	}
	if !s.OnPath {
		s.Blocker, s.BlockerSystem = "", false // nothing to be ahead of
	}
	return s
}

// Prepend returns path with dir first and any other copy of it removed.
func Prepend(path, dir string) string {
	out := []string{dir}
	for _, e := range SplitPath(path) {
		if !samePath(e, dir) {
			out = append(out, e)
		}
	}
	return strings.Join(out, ";")
}

// SplitPath splits a PATH value, dropping empty entries and quotes.
func SplitPath(path string) []string {
	var out []string
	for _, e := range strings.Split(path, ";") {
		if e = strings.Trim(strings.TrimSpace(e), `"`); e != "" {
			out = append(out, e)
		}
	}
	return out
}

func samePath(a, b string) bool {
	clean := func(p string) string { return strings.ToLower(strings.TrimRight(filepath.Clean(p), `\/`)) }
	return clean(a) == clean(b)
}

func hasPHP(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "php.exe"))
	return err == nil
}
