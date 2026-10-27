package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SetService sets services.<key> = "<value>" in root's stack.toml. Only that
// line changes (or one is added to [services]), so comments and layout
// survive. The result must still load; otherwise the file is left as it was.
func SetService(root, key, value string) error {
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
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	line := fmt.Sprintf("%-5s = %q", key, value)
	keyRe := regexp.MustCompile(`^(\s*` + regexp.QuoteMeta(key) + `\s*=\s*)"[^"]*"(.*)$`)

	section, last, done := false, -1, false
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") {
			if section {
				break // end of [services]
			}
			section = t == "[services]"
			if section {
				last = i
			}
			continue
		}
		if !section {
			continue
		}
		if m := keyRe.FindStringSubmatch(l); m != nil {
			lines[i] = m[1] + fmt.Sprintf("%q", value) + m[2]
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
		lines = append(lines, "", "[services]", line, "")
	}

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
