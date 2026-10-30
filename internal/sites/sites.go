// Package sites turns the folders in www/, and the project folders listed in
// stack.toml, into sites with DNS-safe names.
package sites

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/harsha2805/devstack/internal/config"
)

// Site is one project folder.
type Site struct {
	Name     string // folder name as on disk
	Slug     string // DNS label: lowercase, [a-z0-9-]
	Dir      string // absolute project folder
	Docroot  string // absolute folder served by the web server
	PHP      string // version constraint from .devstack.toml, "" = stack default
	Aliases  []string
	HasIndex bool // docroot has index.php, so unknown paths go to the front controller
	Project  bool // listed in stack.toml's [projects] rather than found in www
}

// Project is a folder outside www that stack.toml's [projects] serves under a
// name of its own.
type Project struct {
	Name string // the key in [projects], already a DNS label
	Dir  string // absolute folder
}

// DocrootCandidates are tried in order when .devstack.toml sets no docroot.
var DocrootCandidates = []string{"public", "web", "public_html"}

// Scan lists every listed project and every site under www. Folders in www
// starting with "." or "_" are skipped. A project's name is reserved before www
// is read, so a new folder in www never takes a project's address; a folder that
// is both listed and in www (linked in with mklink /J) is served once, as the
// project. Problems with one site (bad .devstack.toml, a project folder that is
// gone) are returned as warnings, and the rest are still served.
func Scan(www string, projects []Project) ([]Site, []string, error) {
	entries, err := os.ReadDir(www)
	if err != nil {
		return nil, nil, fmt.Errorf("www folder: %w", err)
	}
	var out []Site
	var warns []string
	taken := map[string]string{}
	var served []os.FileInfo // project folders, to spot the same folder in www
	for _, p := range projects {
		st, err := os.Stat(p.Dir)
		if err != nil || !st.IsDir() {
			warns = append(warns, fmt.Sprintf("%s: project folder %s not found, not serving it", p.Name, p.Dir))
			continue
		}
		if dup := sameAs(served, st); dup >= 0 {
			warns = append(warns, fmt.Sprintf("%s: %s is already served as %s", p.Name, p.Dir, out[dup].Slug))
			continue
		}
		served = append(served, st)
		taken[p.Name] = p.Dir
		s, w := load(p.Name, p.Name, p.Dir)
		s.Project = true
		out = append(out, s)
		warns = append(warns, w...)
	}
	for _, e := range entries {
		name := e.Name()
		if !dirEntry(www, e) || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		dir := filepath.Join(www, name)
		if len(served) > 0 {
			if st, err := os.Stat(dir); err == nil && sameAs(served, st) >= 0 {
				continue // a listed project linked into www
			}
		}
		slug := Slugify(name)
		if slug == "" {
			warns = append(warns, fmt.Sprintf("%s: skipped, name has no letters or digits", name))
			continue
		}
		base := slug
		for i := 2; taken[slug] != ""; i++ {
			slug = fmt.Sprintf("%s-%d", base, i)
		}
		if slug != base {
			warns = append(warns, fmt.Sprintf("%s: %s.* is already used by %s, using %s", name, base, taken[base], slug))
		}
		taken[slug] = name
		s, w := load(name, slug, dir)
		out = append(out, s)
		warns = append(warns, w...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, warns, nil
}

// load reads one site's folder: its .devstack.toml, docroot and front controller.
func load(name, slug, dir string) (Site, []string) {
	var warns []string
	s := Site{Name: name, Slug: slug, Dir: dir}
	over, _, err := config.LoadSite(dir)
	if err != nil {
		warns = append(warns, err.Error())
		over = config.Site{}
	}
	s.PHP = over.PHP
	for _, a := range over.Aliases {
		if l := Slugify(a); l != "" {
			s.Aliases = append(s.Aliases, l)
		}
	}
	s.Docroot = dir
	if over.Docroot != "" {
		s.Docroot = filepath.Join(dir, over.Docroot)
		if !isDir(s.Docroot) {
			warns = append(warns, fmt.Sprintf("%s: docroot %q does not exist", name, over.Docroot))
		}
	} else {
		for _, c := range DocrootCandidates {
			if p := filepath.Join(dir, c); isDir(p) {
				s.Docroot = p
				break
			}
		}
	}
	s.HasIndex = isFile(filepath.Join(s.Docroot, "index.php"))
	return s, warns
}

// sameAs is the index in infos of the folder st is, or -1.
func sameAs(infos []os.FileInfo, st os.FileInfo) int {
	for i, fi := range infos {
		if os.SameFile(fi, st) {
			return i
		}
	}
	return -1
}

// Slugify makes a DNS label: "IKEA_BIX_CDP" -> "ikea-bix-cdp".
// Anything outside [a-z0-9] becomes "-", runs collapse, edges are trimmed,
// and the result is capped at 63 characters.
func Slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if len(out) > 63 {
		out = strings.TrimRight(out[:63], "-")
	}
	return out
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// dirEntry reports whether e is a folder, following junctions and symlinks so a
// project linked in from elsewhere (mklink /J) is a site too. Go reports a
// Windows junction as irregular and a symlink as a link, never as a directory.
func dirEntry(www string, e fs.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
		return false
	}
	st, err := os.Stat(filepath.Join(www, e.Name()))
	return err == nil && st.IsDir()
}
