// Package model resolves config, discovered builds and sites into one concrete
// Stack: absolute paths, chosen versions, ports. It does no I/O beyond checking
// which PHP extension DLLs exist, so renderers and services can trust it.
package model

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/sites"
)

type Stack struct {
	Root      string
	WWW       string
	Etc       string // generated, rewritten on every render
	EtcCustom string // user-owned, included but never touched
	Data      string
	Logs      string
	Tmp       string

	TLD      string
	DNS      bool // services.dns: *.<TLD> resolves for every program, so URLs use it
	HTTPPort int
	// HTTPS is services.https: every site is also served on HTTPSPort with a
	// certificate from devstack's local CA (package certs), kept in CertDir.
	HTTPS     bool
	HTTPSPort int
	CertDir   string
	CRLFile   string // the CA's revocation list, published by the landing page (set when rendering)
	// IPv6 is true when [::1] can be bound. Browsers resolve *.localhost to ::1
	// first, and on Windows a refused loopback connect stalls ~200ms before
	// falling back to 127.0.0.1, so we listen on both.
	IPv6 bool

	Nginx      discover.Build
	PHP        []*PHPPool // one per PHP version in use, sorted by version
	DefaultPHP *PHPPool
	MySQL      *MySQL // nil when disabled or not installed

	Sites    []Site
	Warnings []string
}

type PHPPool struct {
	Build       discover.Build
	Key         string // "8.3"
	Name        string // "php-8.3", the unit name
	Upstream    string // "php83", the nginx upstream name
	Ports       []int
	INIPath     string
	ExtDir      string
	Extensions  []string // requested and present
	Opcache     bool
	MaxRequests int
	INI         map[string]string
}

type MySQL struct {
	Build   discover.Build
	Port    int
	DataDir string
	INIPath string
}

type Site struct {
	sites.Site
	PHP       *PHPPool
	Hostnames []string // server_name entries, wildcards included
	AccessLog string
	CertFile  string // set when HTTPS is on
	KeyFile   string
}

// URL is the site's address: <slug>.<tld> when devstack's DNS is on, else
// <slug>.localhost, which browsers resolve with no setup; https when HTTPS is on.
func (s Stack) URL(site Site) string {
	host := site.Slug + ".localhost"
	if s.DNS {
		host = site.Slug + "." + s.TLD
	}
	scheme, port, def := "http", s.HTTPPort, 80
	if s.HTTPS {
		scheme, port, def = "https", s.HTTPSPort, 443
	}
	if port != def {
		return fmt.Sprintf("%s://%s:%d", scheme, host, port)
	}
	return scheme + "://" + host
}

// RootURL is the landing page.
func (s Stack) RootURL() string {
	if s.HTTPPort != 80 {
		return fmt.Sprintf("http://localhost:%d", s.HTTPPort)
	}
	return "http://localhost"
}

// PHPPort gives the port of worker i for PHP version X.Y: base + X*100 + Y*10 + i.
func PHPPort(base int, v discover.Version, i int) int {
	minor := 0
	if len(v) > 1 {
		minor = v[1]
	}
	return base + v[0]*100 + minor*10 + i
}

// Resolve builds the Stack. siteList and siteWarns come from sites.Scan.
func Resolve(cfg config.Stack, found discover.Result, siteList []sites.Site, siteWarns []string) (*Stack, error) {
	abs := func(p string) string {
		if filepath.IsAbs(p) {
			return filepath.Clean(p)
		}
		return filepath.Join(cfg.Root, p)
	}
	st := &Stack{
		Root:      cfg.Root,
		WWW:       abs(cfg.WWW),
		Etc:       filepath.Join(cfg.Root, "etc"),
		EtcCustom: filepath.Join(cfg.Root, "etc-custom"),
		Data:      filepath.Join(cfg.Root, "data"),
		Logs:      filepath.Join(cfg.Root, "logs"),
		Tmp:       filepath.Join(cfg.Root, "tmp"),
		TLD:       cfg.TLD,
		DNS:       cfg.Services.DNS == "on",
		HTTPS:     cfg.Services.HTTPS == "on",
		HTTPSPort: cfg.Ports.HTTPS,
		CertDir:   filepath.Join(cfg.Root, "data", "devstack", "certs"),
		HTTPPort:  cfg.Ports.HTTP,
		IPv6:      ipv6Loopback(),
		Warnings:  append([]string(nil), siteWarns...),
	}
	var errs []error

	nginx, err := found.Pick(discover.Nginx, "")
	if err != nil {
		errs = append(errs, err)
	}
	st.Nginx = nginx

	pools := map[string]*PHPPool{}
	pool := func(b discover.Build) *PHPPool {
		key := b.Version.MajorMinor()
		if p, ok := pools[key]; ok {
			return p
		}
		p := newPool(cfg, st, b)
		pools[key] = p
		return p
	}
	if def, err := found.Pick(discover.PHP, cfg.PHP.Default); err != nil {
		errs = append(errs, fmt.Errorf("php.default: %w", err))
	} else {
		st.DefaultPHP = pool(def)
	}

	if v := cfg.Services.MySQL; v != config.Off {
		b, err := found.Pick(discover.MySQL, v)
		switch {
		case err == nil:
			st.MySQL = &MySQL{
				Build:   b,
				Port:    cfg.Ports.MySQL,
				DataDir: filepath.Join(st.Data, b.Flavor, b.Version.MajorMinor()),
				INIPath: filepath.Join(st.Etc, "mysql", "my.ini"),
			}
		case v == "":
			st.Warnings = append(st.Warnings, "mysql: not installed, skipping ("+err.Error()+")")
		default:
			errs = append(errs, fmt.Errorf("services.mysql: %w", err))
		}
	}

	for _, s := range siteList {
		site := Site{Site: s, AccessLog: filepath.Join(st.Logs, "sites", s.Slug+".access.log")}
		site.PHP = st.DefaultPHP
		if s.PHP != "" {
			if b, err := found.Pick(discover.PHP, s.PHP); err != nil {
				st.Warnings = append(st.Warnings, fmt.Sprintf("%s: %v; using the default PHP", s.Name, err))
			} else {
				site.PHP = pool(b)
			}
		}
		for _, tld := range []string{st.TLD, "localhost"} {
			site.Hostnames = append(site.Hostnames, s.Slug+"."+tld, "*."+s.Slug+"."+tld)
		}
		if st.HTTPS {
			site.CertFile = filepath.Join(st.CertDir, s.Slug+".crt")
			site.KeyFile = filepath.Join(st.CertDir, s.Slug+".key")
		}
		st.Sites = append(st.Sites, site)
	}

	for _, p := range pools {
		st.PHP = append(st.PHP, p)
	}
	sort.Slice(st.PHP, func(i, j int) bool { return st.PHP[i].Build.Version.Less(st.PHP[j].Build.Version) })

	if err := checkPorts(st); err != nil {
		errs = append(errs, err)
	}
	return st, errors.Join(errs...)
}

func newPool(cfg config.Stack, st *Stack, b discover.Build) *PHPPool {
	key := b.Version.MajorMinor()
	p := &PHPPool{
		Build:       b,
		Key:         key,
		Name:        "php-" + key,
		Upstream:    "php" + strings.ReplaceAll(key, ".", ""),
		INIPath:     filepath.Join(st.Etc, "php", key, "php.ini"),
		ExtDir:      filepath.Join(b.Dir, "ext"),
		MaxRequests: cfg.PHP.MaxRequests,
		INI:         cfg.PHP.INI,
	}
	for i := 0; i < cfg.PHP.Workers; i++ {
		p.Ports = append(p.Ports, PHPPort(cfg.Ports.PHP, b.Version, i))
	}
	var missing []string
	for _, e := range cfg.PHP.Extensions {
		if exists(filepath.Join(p.ExtDir, "php_"+e+".dll")) {
			p.Extensions = append(p.Extensions, e)
		} else {
			missing = append(missing, e)
		}
	}
	if len(missing) > 0 {
		st.Warnings = append(st.Warnings, fmt.Sprintf("%s: extensions not found in %s: %s", p.Name, p.ExtDir, strings.Join(missing, ", ")))
	}
	p.Opcache = exists(filepath.Join(p.ExtDir, "php_opcache.dll"))
	return p
}

func checkPorts(st *Stack) error {
	used := map[int]string{st.HTTPPort: "ports.http"}
	var errs []error
	claim := func(port int, who string) {
		if prev, ok := used[port]; ok {
			errs = append(errs, fmt.Errorf("port %d is used by both %s and %s", port, prev, who))
			return
		}
		used[port] = who
	}
	if st.HTTPS {
		claim(st.HTTPSPort, "ports.https")
	}
	if st.MySQL != nil {
		claim(st.MySQL.Port, "ports.mysql")
	}
	for _, p := range st.PHP {
		for _, port := range p.Ports {
			claim(port, p.Name)
		}
	}
	return errors.Join(errs...)
}

// Ports lists every port the stack listens on, with its owner.
func (s Stack) Ports() map[int]string {
	out := map[int]string{s.HTTPPort: "nginx"}
	if s.HTTPS {
		out[s.HTTPSPort] = "nginx"
	}
	if s.MySQL != nil {
		out[s.MySQL.Port] = "mysql"
	}
	for _, p := range s.PHP {
		for _, port := range p.Ports {
			out[port] = p.Name
		}
	}
	return out
}

// ipv6Loopback reports whether this machine can listen on [::1].
var ipv6Loopback = func() bool {
	ln, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
