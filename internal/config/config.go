// Package config loads stack.toml and per-site .devstack.toml files.
//
// The types here mirror the files on disk. Paths are kept as written; model.Resolve
// turns them into absolute paths against the root.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/harsha2805/devstack/internal/appconf"
	"github.com/harsha2805/devstack/internal/winproc"
)

// FileName is the name of the stack config file. Its folder is the stack root.
const FileName = "stack.toml"

// SiteFileName is the optional per-site override file.
const SiteFileName = ".devstack.toml"

// Off disables an optional service, e.g. services.mysql = "off".
const Off = "off"

type Stack struct {
	WWW      string   `toml:"www"`
	TLD      string   `toml:"tld"`
	Paths    Paths    `toml:"paths"`
	Services Services `toml:"services"`
	PHP      PHP      `toml:"php"`
	Ports    Ports    `toml:"ports"`
	// Projects are folders outside www, by the name they're served under:
	// acme = "E:/clients/acme" serves acme.localhost and acme.<tld>.
	Projects map[string]string `toml:"projects"`

	// Root is the folder holding stack.toml. Not read from the file.
	Root string `toml:"-"`
}

type Paths struct {
	Bin []string `toml:"bin"`
}

type Services struct {
	Web   string `toml:"web"`
	MySQL string `toml:"mysql"`
	DNS   string `toml:"dns"`   // "on": answer *.<tld> on 127.0.0.1:53 (see package dns)
	HTTPS string `toml:"https"` // "on": serve every site over HTTPS too (see package certs)
}

type PHP struct {
	Default     string            `toml:"default"`
	Workers     int               `toml:"workers"`
	MaxRequests int               `toml:"max_requests"`
	Extensions  []string          `toml:"extensions"`
	INI         map[string]string `toml:"ini"`
}

type Ports struct {
	HTTP  int `toml:"http"`
	HTTPS int `toml:"https"`
	MySQL int `toml:"mysql"`
	PHP   int `toml:"php"`
}

// Site is the optional www/<site>/.devstack.toml.
type Site struct {
	Docroot string   `toml:"docroot"`
	PHP     string   `toml:"php"`
	Aliases []string `toml:"aliases"`
}

// DefaultExtensions are enabled when php.extensions is not set.
var DefaultExtensions = []string{
	"curl", "fileinfo", "gd", "intl", "mbstring", "mysqli", "openssl",
	"pdo_mysql", "pdo_sqlite", "sqlite3", "sodium", "zip", "exif",
}

// Default returns a config with every field set to its default.
func Default() Stack {
	return Stack{
		WWW:      "www",
		TLD:      "test",
		Paths:    Paths{Bin: []string{"bin"}},
		Services: Services{Web: "nginx", DNS: Off, HTTPS: Off},
		PHP: PHP{
			Workers:     4,
			MaxRequests: 500,
			Extensions:  append([]string(nil), DefaultExtensions...),
			INI:         map[string]string{},
		},
		Ports: Ports{HTTP: 80, HTTPS: 443, MySQL: 3306, PHP: 9000},
	}
}

// Load reads <root>/stack.toml on top of the defaults and validates it.
func Load(root string) (Stack, error) {
	cfg := Default()
	path := filepath.Join(root, FileName)
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	if undec := md.Undecoded(); len(undec) > 0 {
		keys := make([]string, len(undec))
		for i, k := range undec {
			keys[i] = k.String()
		}
		sort.Strings(keys)
		return cfg, fmt.Errorf("%s: unknown keys: %s", path, strings.Join(keys, ", "))
	}
	if cfg.PHP.INI == nil {
		cfg.PHP.INI = map[string]string{}
	}
	cfg.Root = winproc.LongPath(root)
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

var (
	tldRe     = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	nameRe    = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	versionRe = regexp.MustCompile(`^\d+(\.\d+){0,2}$`)
	extRe     = regexp.MustCompile(`^[a-z0-9_]+$`)
)

// Validate reports every problem at once, each prefixed with its key.
func (c Stack) Validate() error {
	var errs []error
	bad := func(key, format string, args ...any) {
		errs = append(errs, fmt.Errorf("%s: %s", key, fmt.Sprintf(format, args...)))
	}

	if strings.TrimSpace(c.WWW) == "" {
		bad("www", "must not be empty")
	}
	if !tldRe.MatchString(c.TLD) {
		bad("tld", "%q is not a valid top-level domain", c.TLD)
	} else if c.TLD == "localhost" {
		bad("tld", "localhost is always served; pick another, e.g. \"test\"")
	}
	if len(c.Paths.Bin) == 0 {
		bad("paths.bin", "needs at least one folder")
	}
	if c.Services.Web != "nginx" {
		bad("services.web", "%q is not supported yet (only \"nginx\")", c.Services.Web)
	}
	if v := c.Services.MySQL; v != "" && v != Off && !versionRe.MatchString(v) {
		bad("services.mysql", "%q must be a version like \"8.4\", \"\" for newest, or \"off\"", v)
	}
	if v := c.Services.DNS; v != "on" && v != Off {
		bad("services.dns", "%q must be \"on\" or \"off\"", v)
	}
	if v := c.Services.HTTPS; v != "on" && v != Off {
		bad("services.https", "%q must be \"on\" or \"off\"", v)
	}
	if v := c.PHP.Default; v != "" && !versionRe.MatchString(v) {
		bad("php.default", "%q must be a version like \"8.3\"", v)
	}
	if c.PHP.Workers < 1 || c.PHP.Workers > 10 {
		bad("php.workers", "must be 1..10, got %d", c.PHP.Workers)
	}
	if c.PHP.MaxRequests < 0 {
		bad("php.max_requests", "must be >= 0, got %d", c.PHP.MaxRequests)
	}
	for _, e := range c.PHP.Extensions {
		if !extRe.MatchString(e) {
			bad("php.extensions", "%q is not an extension name (use \"curl\", not \"php_curl.dll\")", e)
		}
	}
	for name, path := range c.Projects {
		if !ValidName(name) {
			bad("projects."+name, "a name must be lowercase letters, digits and dashes, like \"my-app\"")
		}
		if strings.TrimSpace(path) == "" {
			bad("projects."+name, "needs a folder")
		}
	}
	checkPort := func(key string, p int) {
		if p < 1 || p > 65535 {
			bad(key, "must be 1..65535, got %d", p)
		}
	}
	checkPort("ports.http", c.Ports.HTTP)
	checkPort("ports.https", c.Ports.HTTPS)
	checkPort("ports.mysql", c.Ports.MySQL)
	if c.Ports.PHP < 1024 || c.Ports.PHP > 65535-1000 {
		bad("ports.php", "must be 1024..64535, got %d", c.Ports.PHP)
	}
	return errors.Join(errs...)
}

// ValidName reports whether name can be a project name: a DNS label.
func ValidName(name string) bool { return nameRe.MatchString(name) }

// LoadSite reads dir/.devstack.toml. A missing file is not an error.
func LoadSite(dir string) (Site, bool, error) {
	var s Site
	path := filepath.Join(dir, SiteFileName)
	md, err := toml.DecodeFile(path, &s)
	if errors.Is(err, os.ErrNotExist) {
		return s, false, nil
	}
	if err != nil {
		return s, true, fmt.Errorf("%s: %w", path, err)
	}
	if undec := md.Undecoded(); len(undec) > 0 {
		return s, true, fmt.Errorf("%s: unknown key %s", path, undec[0])
	}
	if s.PHP != "" && !versionRe.MatchString(s.PHP) {
		return s, true, fmt.Errorf("%s: php: %q must be a version like \"8.3\"", path, s.PHP)
	}
	if filepath.IsAbs(s.Docroot) || strings.Contains(filepath.ToSlash(s.Docroot), "..") {
		return s, true, fmt.Errorf("%s: docroot must be a folder inside the site", path)
	}
	return s, true, nil
}

// FindRoot picks the stack root: explicit flag, then DEVSTACK_ROOT, then the
// nearest parent of cwd holding stack.toml, then the folder of the executable,
// then the root saved by the desktop app.
// The result is always a long path (no 8.3 short names; nginx rejects them).
func FindRoot(flag string) (string, error) {
	root, err := findRoot(flag)
	if err != nil {
		return "", err
	}
	return winproc.LongPath(root), nil
}

func findRoot(flag string) (string, error) {
	if flag != "" {
		return filepath.Abs(flag)
	}
	if env := os.Getenv("DEVSTACK_ROOT"); env != "" {
		return filepath.Abs(env)
	}
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; dir = filepath.Dir(dir) {
			if fileExists(filepath.Join(dir, FileName)) {
				return dir, nil
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if fileExists(filepath.Join(dir, FileName)) {
			return dir, nil
		}
	}
	// The stack the desktop app manages, so the CLI works from any folder.
	if s, err := appconf.Load(); err == nil && s.Root != "" && fileExists(filepath.Join(s.Root, FileName)) {
		return s.Root, nil
	}
	return "", fmt.Errorf("no %s found in this folder or its parents; run `devstack init` or pass --root", FileName)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
