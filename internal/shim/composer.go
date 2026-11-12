package shim

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FindComposer returns the newest installed composer.phar under binRoot
// (bin\composer\<version>\composer.phar). It lives here, not in package
// packages, so the shim doesn't link the downloader: every `php` command
// starts the shim.
func FindComposer(binRoot string) (string, bool) {
	dir := filepath.Join(binRoot, "composer")
	entries, _ := os.ReadDir(dir)
	var best []int
	path := ""
	for _, e := range entries {
		v, ok := parseDotted(e.Name())
		phar := filepath.Join(dir, e.Name(), "composer.phar")
		if _, err := os.Stat(phar); !ok || err != nil {
			continue
		}
		if path == "" || less(best, v) {
			best, path = v, phar
		}
	}
	return path, path != ""
}

// parseDotted reads "2.8.12" as [2 8 12].
func parseDotted(s string) ([]int, bool) {
	var out []int
	for _, p := range strings.Split(s, ".") {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, false
		}
		out = append(out, n)
	}
	return out, len(out) > 0
}

func less(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}
