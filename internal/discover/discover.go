// Package discover finds installed server builds under the configured bin folders.
//
// Layout: <bin>/<service>/<any folder name>/... The folder name is ignored; the
// version comes from running the executable, so renamed folders still work.
package discover

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/harsha2805/devstack/internal/winproc"
)

// Kind names a service family.
type Kind string

const (
	Nginx Kind = "nginx"
	PHP   Kind = "php"
	MySQL Kind = "mysql"
)

// Build is one installed version of a service.
type Build struct {
	Kind    Kind
	Version Version
	Dir     string // install folder
	Exe     string // main executable (nginx.exe, php-cgi.exe, bin/mysqld.exe)
	Flavor  string // "mysql" or "mariadb" for MySQL; empty otherwise
}

func (b Build) String() string { return fmt.Sprintf("%s %s", b.Kind, b.Version) }

// layout says where the main executable lives in an install folder and how to read its version.
type layout struct {
	exe     string
	args    []string
	pattern *regexp.Regexp
}

var layouts = map[Kind]layout{
	Nginx: {"nginx.exe", []string{"-v"}, regexp.MustCompile(`nginx/(\d+\.\d+\.\d+)`)},
	PHP:   {"php-cgi.exe", []string{"-v"}, regexp.MustCompile(`PHP (\d+\.\d+\.\d+)`)},
	MySQL: {filepath.Join("bin", "mysqld.exe"), []string{"--version"}, regexp.MustCompile(`Ver (\d+\.\d+\.\d+)(-MariaDB)?`)},
}

// Result holds every build found, newest first per kind.
type Result struct {
	Builds   map[Kind][]Build
	Problems []string // folders that looked like installs but could not be probed
}

// Scan looks in every bin root for every kind and probes versions in parallel.
func Scan(ctx context.Context, binRoots []string) Result {
	type cand struct {
		kind Kind
		dir  string
	}
	var cands []cand
	seen := map[string]bool{}
	for _, root := range binRoots {
		for kind, l := range layouts {
			entries, err := os.ReadDir(filepath.Join(root, string(kind)))
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				dir := filepath.Join(root, string(kind), e.Name())
				key := strings.ToLower(dir)
				if seen[key] || !isFile(filepath.Join(dir, l.exe)) {
					continue
				}
				seen[key] = true
				cands = append(cands, cand{kind, dir})
			}
		}
	}

	res := Result{Builds: map[Kind][]Build{}}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, c := range cands {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, err := probe(ctx, c.kind, c.dir)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				res.Problems = append(res.Problems, err.Error())
				return
			}
			res.Builds[c.kind] = append(res.Builds[c.kind], b)
		}()
	}
	wg.Wait()
	for k := range res.Builds {
		bs := res.Builds[k]
		sort.SliceStable(bs, func(i, j int) bool { return bs[j].Version.Less(bs[i].Version) })
	}
	sort.Strings(res.Problems)
	return res
}

// probeCache remembers versions by executable path, size and mtime, so
// repeated scans (the app polls) don't keep launching nginx/php/mysqld.
var probeCache sync.Map // string -> Build

func probe(ctx context.Context, kind Kind, dir string) (Build, error) {
	l := layouts[kind]
	exe := filepath.Join(dir, l.exe)
	st, err := os.Stat(exe)
	if err != nil {
		return Build{}, err
	}
	key := fmt.Sprintf("%s|%d|%d", strings.ToLower(exe), st.Size(), st.ModTime().UnixNano())
	if b, ok := probeCache.Load(key); ok {
		return b.(Build), nil
	}
	b, err := probeExe(ctx, kind, dir, exe)
	if err == nil {
		probeCache.Store(key, b)
	}
	return b, err
}

func probeExe(ctx context.Context, kind Kind, dir, exe string) (Build, error) {
	l := layouts[kind]
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, l.args...)
	cmd.Dir = dir
	winproc.HideWindow(cmd)
	out, err := cmd.CombinedOutput()
	m := l.pattern.FindStringSubmatch(string(out))
	if m == nil {
		if err == nil {
			err = fmt.Errorf("unrecognized output %q", firstLine(string(out)))
		}
		return Build{}, fmt.Errorf("%s: cannot read version: %v", exe, err)
	}
	v, _ := ParseVersion(m[1])
	b := Build{Kind: kind, Version: v, Dir: dir, Exe: exe}
	if kind == MySQL {
		b.Flavor = "mysql"
		if len(m) > 2 && m[2] != "" {
			b.Flavor = "mariadb"
		}
	}
	return b, nil
}

// Pick returns the newest build of kind matching the constraint ("" = any, "8.3", "8.3.36").
func (r Result) Pick(kind Kind, constraint string) (Build, error) {
	want, err := ParseVersion(constraint)
	if constraint != "" && err != nil {
		return Build{}, err
	}
	for _, b := range r.Builds[kind] {
		if constraint == "" || b.Version.Matches(want) {
			return b, nil
		}
	}
	have := make([]string, 0, len(r.Builds[kind]))
	for _, b := range r.Builds[kind] {
		have = append(have, b.Version.String())
	}
	if len(have) == 0 {
		return Build{}, fmt.Errorf("no %s build found in paths.bin (looked for %s/<version>/%s)", kind, kind, layouts[kind].exe)
	}
	return Build{}, fmt.Errorf("no %s build matches %q (installed: %s)", kind, constraint, strings.Join(have, ", "))
}

// Version is a dotted numeric version. A constraint may have fewer parts than a build.
type Version []int

func ParseVersion(s string) (Version, error) {
	if s == "" {
		return nil, fmt.Errorf("empty version")
	}
	parts := strings.Split(s, ".")
	v := make(Version, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("invalid version %q", s)
		}
		v[i] = n
	}
	return v, nil
}

func (v Version) String() string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = strconv.Itoa(n)
	}
	return strings.Join(s, ".")
}

// MajorMinor returns "8.3" for 8.3.36.
func (v Version) MajorMinor() string {
	if len(v) < 2 {
		return v.String()
	}
	return fmt.Sprintf("%d.%d", v[0], v[1])
}

// Matches reports whether v starts with every part of the constraint c.
func (v Version) Matches(c Version) bool {
	if len(c) > len(v) {
		return false
	}
	for i := range c {
		if v[i] != c[i] {
			return false
		}
	}
	return true
}

func (v Version) Less(o Version) bool {
	for i := 0; i < len(v) && i < len(o); i++ {
		if v[i] != o[i] {
			return v[i] < o[i]
		}
	}
	return len(v) < len(o)
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
