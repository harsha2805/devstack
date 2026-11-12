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
	// System is the administrators-only copy of the shims (SystemDir) when it
	// is what the system PATH runs. Only that copy may go on the system PATH:
	// Dir can be changed without administrator rights, and programs run as
	// administrator use the system PATH too.
	System string `json:"system,omitempty"`
	// OnPath is true when Dir, or System, is on the system or user PATH.
	OnPath bool `json:"on_path"`
	// Blocker is a folder ahead of the shims with its own php.exe, which
	// terminals run instead of the shim. Empty when the shim comes first.
	Blocker string `json:"blocker,omitempty"`
	// BlockerSystem is true when Blocker is on the system PATH, which comes
	// before every user entry, so moving the shims needs administrator rights.
	BlockerSystem bool `json:"blocker_system,omitempty"`
	// Unsafe is true when Dir itself is on the system PATH (devstack 1.4.1 and
	// earlier put it there).
	Unsafe bool `json:"unsafe,omitempty"`
	// Stale is true when System holds shims from another devstack version.
	Stale bool `json:"stale,omitempty"`
}

// OK reports whether `php` in a new terminal runs the shim, safely.
func (s PathStatus) OK() bool { return s.OnPath && s.Blocker == "" && !s.Unsafe && !s.Stale }

// NeedsAdmin reports whether fixing s means changing the system PATH or the
// administrators-only copy, so one administrator prompt.
func (s PathStatus) NeedsAdmin() bool { return s.BlockerSystem || s.Unsafe || s.Stale }

// Problem says what is wrong, for people; "" when OK.
func (s PathStatus) Problem() string {
	switch {
	case s.Blocker != "":
		where := "your user PATH"
		if s.BlockerSystem {
			where = "the system PATH"
		}
		return "Terminals run the PHP in " + s.Blocker + " (on " + where + ") instead of each site's PHP."
	case s.Unsafe:
		return "The system PATH runs devstack's php and composer from " + s.Dir + ", which isn't limited to administrators, so programs run as administrator could be made to run something else."
	case s.Stale:
		return "The copy of devstack's php and composer in " + s.System + " is from another devstack version."
	case !s.OnPath:
		return "devstack's php and composer (" + s.Dir + ") aren't on PATH, so terminals don't use each site's PHP."
	}
	return ""
}

// CheckPath works out s for dir and its administrators-only copy sysDir
// ("" for none) from the raw system and user PATH values (already expanded).
// It doesn't compare the copy with dir; Stale is the caller's.
func CheckPath(dir, sysDir, system, user string) PathStatus {
	s := PathStatus{Dir: dir}
	scan := func(path string, sys bool) bool {
		for _, e := range SplitPath(path) {
			if samePath(e, dir) || (sysDir != "" && samePath(e, sysDir)) {
				s.OnPath = true
				if samePath(e, dir) {
					s.Unsafe = sys
				} else {
					s.System = sysDir
				}
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

// Remove returns path without dir.
func Remove(path, dir string) string {
	var out []string
	for _, e := range SplitPath(path) {
		if !samePath(e, dir) {
			out = append(out, e)
		}
	}
	return strings.Join(out, ";")
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
