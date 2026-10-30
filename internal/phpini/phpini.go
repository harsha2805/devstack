// Package phpini reads and edits php.ini text line by line, keeping every
// comment and blank line, so a file the user edits by hand stays theirs.
package phpini

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Lines splits text into lines without their line endings.
func Lines(text string) []string {
	return strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
}

func join(lines []string) string { return strings.Join(lines, "\n") }

// Full reports whether text is a whole php.ini (it has a [PHP] section)
// rather than a few settings to add to one.
func Full(text string) bool {
	for _, l := range Lines(text) {
		if strings.EqualFold(strings.TrimSpace(l), "[PHP]") {
			return true
		}
	}
	return false
}

// directive matches "key = value" or a commented-out ";key = value". PHP's
// own php.ini comments out settings with a bare ";" and indents the examples
// in its prose ";   extension=mysqli", which must not count as settings.
var directive = regexp.MustCompile(`^\s*(;)?([A-Za-z0-9_.\-]+)\s*=(.*)$`)

// parse returns the key and value of a directive line, and whether it's
// commented out. ok is false for anything else.
func parse(line string) (key, value string, commented, ok bool) {
	m := directive.FindStringSubmatch(line)
	if m == nil {
		return "", "", false, false
	}
	return m[2], stripComment(m[3]), m[1] != "", true
}

// stripComment drops a trailing "; comment" outside quotes and trims.
func stripComment(v string) string {
	quoted := false
	for i, r := range v {
		switch {
		case r == '"':
			quoted = !quoted
		case r == ';' && !quoted:
			return strings.TrimSpace(v[:i])
		}
	}
	return strings.TrimSpace(v)
}

// Settings lists the active "key = value" lines in order, extensions excluded.
func Settings(text string) [][2]string {
	var out [][2]string
	for _, l := range Lines(text) {
		k, v, commented, ok := parse(l)
		if ok && !commented && !isExtKey(k) {
			out = append(out, [2]string{k, v})
		}
	}
	return out
}

// Set makes key = value take effect: it rewrites the last active line for key,
// or else the first commented-out one, or else appends a line.
func Set(text, key, value string) string {
	lines := Lines(text)
	active, commented := -1, -1
	for i, l := range lines {
		k, _, c, ok := parse(l)
		if !ok || !strings.EqualFold(k, key) {
			continue
		}
		if !c {
			active = i
		} else if commented < 0 {
			commented = i
		}
	}
	line := key + " = " + value
	switch {
	case active >= 0:
		lines[active] = line
	case commented >= 0:
		lines[commented] = line
	default:
		lines = appendLine(lines, line)
	}
	return join(lines)
}

func appendLine(lines []string, line string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return append(lines, line, "")
}

func isExtKey(k string) bool {
	return strings.EqualFold(k, "extension") || strings.EqualFold(k, "zend_extension")
}

// ExtName is the name an extension= value loads: "gd" for gd, php_gd.dll,
// "ext/php_gd.dll" or "C:\php\ext\php_gd.dll".
func ExtName(value string) string {
	v := strings.Trim(strings.TrimSpace(value), `"'`)
	v = filepath.Base(strings.ReplaceAll(v, `\`, "/"))
	v = strings.TrimSuffix(strings.TrimSuffix(v, ".dll"), ".so")
	return strings.TrimPrefix(v, "php_")
}

// ext is one extension= line: its index, name and whether it's turned off.
type ext struct {
	i         int
	name      string
	commented bool
	zend      bool
}

func extLines(lines []string) []ext {
	var out []ext
	for i, l := range lines {
		k, v, c, ok := parse(l)
		if ok && isExtKey(k) && v != "" {
			out = append(out, ext{i, ExtName(v), c, strings.EqualFold(k, "zend_extension")})
		}
	}
	return out
}

// Extensions lists the extensions text loads with extension=, in order.
func Extensions(text string) []string {
	var out []string
	for _, e := range extLines(Lines(text)) {
		if !e.commented && !e.zend {
			out = append(out, e.name)
		}
	}
	return out
}

// Listed reports whether text has an extension= line for name, on or off.
func Listed(text, name string) bool {
	for _, e := range extLines(Lines(text)) {
		if !e.zend && e.name == name {
			return true
		}
	}
	return false
}

// Enable turns on extension name (zend_extension when zend): it removes the ;
// from its line, or adds a line after the last one of its kind.
func Enable(text, name string, zend bool) string {
	lines := Lines(text)
	exts := extLines(lines)
	for _, e := range exts {
		if e.name == name && e.zend == zend && !e.commented {
			return text
		}
	}
	for _, e := range exts {
		if e.name == name && e.zend == zend {
			lines[e.i] = strings.TrimLeft(strings.TrimSpace(lines[e.i]), "; \t")
			return join(lines)
		}
	}
	line := "extension=" + name
	if zend {
		line = "zend_extension=" + name
	}
	at, last := -1, -1 // after the last line of the same kind, else the last extension line
	for _, e := range exts {
		last = e.i + 1
		if e.zend == zend {
			at = e.i + 1
		}
	}
	if at < 0 {
		at = last
	}
	if at < 0 {
		return join(appendLine(lines, line))
	}
	return join(append(lines[:at], append([]string{line}, lines[at:]...)...))
}

// Disable comments out every extension= line for which match(name) is true.
func Disable(text string, match func(name string) bool) string {
	lines := Lines(text)
	for _, e := range extLines(lines) {
		if !e.commented && !e.zend && match(e.name) {
			lines[e.i] = ";" + strings.TrimSpace(lines[e.i])
		}
	}
	return join(lines)
}

// AddOff adds ";extension=name" after the last extension= line, so the
// extension is listed but off.
func AddOff(text, name string) string {
	return Disable(Enable(text, name, false), func(n string) bool { return n == name })
}

// Unset turns key off by commenting out every active line for it, so PHP's
// built-in default applies and the line stays in place to turn back on.
func Unset(text, key string) string {
	lines := Lines(text)
	for i, l := range lines {
		if k, _, c, ok := parse(l); ok && !c && strings.EqualFold(k, key) {
			lines[i] = ";" + strings.TrimSpace(l)
		}
	}
	return join(lines)
}

// Ext is an extension= line's name and whether it's on.
type Ext struct {
	Name string `json:"name"`
	On   bool   `json:"on"`
}

// ExtensionStates lists every extension text names with extension=, on or
// off, once each in file order. One listed both ways counts as on.
func ExtensionStates(text string) []Ext {
	var out []Ext
	at := map[string]int{}
	for _, e := range extLines(Lines(text)) {
		if e.zend {
			continue
		}
		if i, ok := at[e.name]; ok {
			out[i].On = out[i].On || !e.commented
			continue
		}
		at[e.name] = len(out)
		out = append(out, Ext{e.name, !e.commented})
	}
	return out
}
