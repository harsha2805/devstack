package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SetService sets services.<key> = "<value>" in root's stack.toml.
func SetService(root, key, value string) error {
	return SetKey(root, "services", key, fmt.Sprintf("%q", value))
}

// SetKey sets <section>.<key> to literal (TOML syntax: `"on"`, `443`) in
// root's stack.toml. Only that line changes (or one is added to the section),
// so comments and layout survive. The result must still load; otherwise the
// file is left as it was.
func SetKey(root, section, key, literal string) error {
	return editLines(root, func(lines []string) []string { return setLine(lines, section, key, literal) })
}

// SetExtensions sets php.extensions to names in root's stack.toml, replacing
// the whole array even when it spans several lines.
func SetExtensions(root string, names []string) error {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = fmt.Sprintf("%q", n)
	}
	literal := "[" + strings.Join(quoted, ", ") + "]"
	return editLines(root, func(lines []string) []string {
		inSection := false
		for i := 0; i < len(lines); i++ {
			t := strings.TrimSpace(lines[i])
			if strings.HasPrefix(t, "[") {
				if inSection {
					break
				}
				inSection = t == "[php]"
				continue
			}
			m := arrayKeyRe.FindStringSubmatch(lines[i])
			if !inSection || m == nil {
				continue
			}
			// Extension names can't contain "]" or "#", so the first "]"
			// outside a comment closes the array.
			end := i
			for end < len(lines) && !strings.Contains(code(lines[end]), "]") {
				end++
			}
			if end == len(lines) {
				return lines // unterminated: Load reports it
			}
			_, rest, _ := strings.Cut(lines[end], "]")
			lines[i] = m[1] + literal + rest
			return append(lines[:i+1], lines[end+1:]...)
		}
		return setLine(lines, "php", "extensions", literal)
	})
}

var arrayKeyRe = regexp.MustCompile(`^(\s*extensions\s*=\s*)\[`)

// code is l without a trailing # comment.
func code(l string) string {
	before, _, _ := strings.Cut(l, "#")
	return before
}

// editLines rewrites root's stack.toml through fn, keeping its BOM and line
// endings. The result must still load; otherwise the file is left as it was.
func editLines(root string, fn func([]string) []string) error {
	path := filepath.Join(root, FileName)
	old, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(old)
	bom := ""
	if strings.HasPrefix(text, "\xef\xbb\xbf") {
		bom, text = "\xef\xbb\xbf", strings.TrimPrefix(text, "\xef\xbb\xbf")
	}
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	lines := fn(strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"))

	out := bom + strings.Join(lines, nl)
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return err
	}
	if _, err := Load(root); err != nil {
		os.WriteFile(path, old, 0o644)
		return err
	}
	return nil
}

// setLine sets <section>.<key> = literal on its line, or adds the line to the
// section (or the section to the end).
func setLine(lines []string, section, key, literal string) []string {
	line := fmt.Sprintf("%-5s = %s", key, literal)
	keyRe := regexp.MustCompile(`^(\s*` + regexp.QuoteMeta(key) + `\s*=\s*)("[^"]*"|[^\s#]+)(.*)$`)

	inSection, last, done := false, -1, false
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") {
			if inSection {
				break // end of the section
			}
			inSection = t == "["+section+"]"
			if inSection {
				last = i
			}
			continue
		}
		if !inSection {
			continue
		}
		if m := keyRe.FindStringSubmatch(l); m != nil {
			lines[i] = m[1] + literal + m[3]
			done = true
			break
		}
		if t != "" && !strings.HasPrefix(t, "#") {
			last = i
		}
	}
	switch {
	case done:
	case last >= 0:
		lines = append(lines[:last+1], append([]string{line}, lines[last+1:]...)...)
	default:
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, "", "["+section+"]", line, "")
	}
	return lines
}
