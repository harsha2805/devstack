// Package setup creates a new stack: it checks the folder, downloads the
// servers that are missing, picks free ports, and writes stack.toml plus a
// sample site. `devstack init` and the app's first-run screen both use it.
package setup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/packages"
	"github.com/harsha2805/devstack/internal/winproc"
)

// VCRedistURL is Microsoft's permanent link to the latest VC++ x64 runtime.
const VCRedistURL = "https://aka.ms/vs/17/release/vc_redist.x64.exe"

// Defaults downloaded when nothing suitable is installed.
const (
	DefaultPHP   = "8.4"
	DefaultMySQL = "8.4" // LTS
)

type Options struct {
	Root      string
	WWW       string   // projects folder; default "www"
	ExtraBins []string // folders with existing builds, searched after <root>/bin
	Download  bool     // download what is missing
	Force     bool     // overwrite an existing stack.toml

	// Version wishes: "" means "anything installed, else the default".
	Nginx string
	PHP   string
	MySQL string // or config.Off
}

// Events report progress; every field is optional.
type Events struct {
	Download  func(e packages.Entry, done, total int64)
	Installed func(e packages.Entry, dir string)
	// Port is called for each port; wanted != picked when wanted was taken.
	Port func(name string, wanted, picked int, takenBy string)
}

type Result struct {
	ConfigPath string
	Found      discover.Result
	HTTPPort   int
	MySQLPort  int
	PHPBase    int
	SampleSite string // created sample site folder, if any
	VCRuntime  bool   // the Visual C++ runtime PHP and MySQL need is present
}

// DefaultRoot suggests C:\devstack (short, ASCII, outside the profile), or
// %USERPROFILE%\devstack when C:\ isn't writable.
func DefaultRoot() string {
	c := `C:\devstack`
	if st, err := os.Stat(c); err == nil && st.IsDir() {
		return c
	}
	if err := os.Mkdir(c, 0o755); err == nil {
		os.Remove(c)
		return c
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(winproc.LongPath(home), "devstack")
	}
	return c
}

// CheckRoot rejects folders the servers handle badly on Windows: non-ASCII
// characters and spaces break nginx, some PHP extensions and MySQL.
func CheckRoot(root string) error {
	if !filepath.IsAbs(root) {
		return fmt.Errorf("\"%s\" is not a full path, e.g. C:\\devstack", root)
	}
	for _, r := range root {
		if r > 127 || r == ' ' {
			return fmt.Errorf("\"%s\" has spaces or non-English letters; nginx, PHP and MySQL on Windows break on such paths. Use for example C:\\devstack", root)
		}
	}
	return nil
}

func binRoots(o Options) []string {
	out := []string{filepath.Join(o.Root, "bin")}
	for _, b := range o.ExtraBins {
		if !filepath.IsAbs(b) {
			b = filepath.Join(o.Root, b)
		}
		out = append(out, b)
	}
	return out
}

// Plan returns the catalog entries Init would download.
func Plan(ctx context.Context, o Options) ([]packages.Entry, error) {
	return plan(o, discover.Scan(ctx, binRoots(o)))
}

func plan(o Options, found discover.Result) ([]packages.Entry, error) {
	cat, err := packages.Default()
	if err != nil {
		return nil, err
	}
	wishes := []struct {
		kind discover.Kind
		want string
		def  string
	}{
		{discover.Nginx, o.Nginx, stableNginx(cat)},
		{discover.PHP, o.PHP, DefaultPHP},
		{discover.MySQL, o.MySQL, DefaultMySQL},
	}
	var todo []packages.Entry
	for _, w := range wishes {
		if w.want == config.Off {
			continue
		}
		if _, err := found.Pick(w.kind, w.want); err == nil {
			continue
		}
		c := w.want
		if c == "" {
			c = w.def
		}
		e, err := cat.Find(w.kind, c)
		if err != nil {
			return nil, err
		}
		todo = append(todo, e)
	}
	return todo, nil
}

func stableNginx(c packages.Catalog) string {
	for _, e := range c.Entries {
		if e.Kind == discover.Nginx && e.Label == "stable" {
			return e.Version
		}
	}
	return ""
}

// Init creates the stack described by o.
func Init(ctx context.Context, o Options, ev Events) (Result, error) {
	var res Result
	if o.WWW == "" {
		o.WWW = "www"
	}
	if err := CheckRoot(o.Root); err != nil {
		return res, err
	}
	res.ConfigPath = filepath.Join(o.Root, config.FileName)
	if _, err := os.Stat(res.ConfigPath); err == nil && !o.Force {
		return res, fmt.Errorf("%s already exists", res.ConfigPath)
	}
	if err := os.MkdirAll(o.Root, 0o755); err != nil {
		return res, err
	}

	bins := binRoots(o)
	found := discover.Scan(ctx, bins)
	if o.Download {
		todo, err := plan(o, found)
		if err != nil {
			return res, err
		}
		for _, e := range todo {
			var progress packages.Progress
			if ev.Download != nil {
				progress = func(done, total int64) { ev.Download(e, done, total) }
			}
			dir, err := packages.Install(ctx, bins[0], e, progress)
			if err != nil && !errors.Is(err, packages.ErrInstalled) {
				return res, err
			}
			if ev.Installed != nil {
				ev.Installed(e, dir)
			}
		}
		if len(todo) > 0 {
			found = discover.Scan(ctx, bins)
		}
	}
	res.Found = found

	if _, err := found.Pick(discover.Nginx, o.Nginx); err != nil {
		return res, err
	}
	php, err := found.Pick(discover.PHP, o.PHP)
	if err != nil {
		return res, err
	}
	mysqlVer := config.Off
	if o.MySQL != config.Off {
		if m, err := found.Pick(discover.MySQL, o.MySQL); err == nil {
			mysqlVer = m.Version.MajorMinor()
		} else if o.MySQL != "" {
			return res, err
		}
	}
	_, res.VCRuntime = winproc.VCRuntime()

	res.HTTPPort = pickPort(ev, "http", 80, 8080, 8000, 8888, 8081)
	res.MySQLPort = pickPort(ev, "mysql", 3306, 3307, 3308, 3309)
	res.PHPBase = pickPHPBase(php.Version, config.Default().PHP.Workers)

	binList := []string{"bin"}
	for _, b := range o.ExtraBins {
		binList = append(binList, b)
	}
	data := map[string]any{
		"WWW": filepath.ToSlash(o.WWW), "Bins": quoteList(binList),
		"PHP": php.Version.MajorMinor(), "MySQL": mysqlVer,
		"HTTP": res.HTTPPort, "MySQLPort": res.MySQLPort, "PHPBase": res.PHPBase,
		"Extensions": quoteList(config.DefaultExtensions),
	}
	var sb strings.Builder
	if err := stackTemplate.Execute(&sb, data); err != nil {
		return res, err
	}
	if err := os.WriteFile(res.ConfigPath, []byte(strings.ReplaceAll(sb.String(), "\n", "\r\n")), 0o644); err != nil {
		return res, err
	}

	www := o.WWW
	if !filepath.IsAbs(www) {
		www = filepath.Join(o.Root, www)
	}
	res.SampleSite, err = ensureSampleSite(www)
	return res, err
}

// pickPort returns the first free candidate.
func pickPort(ev Events, name string, candidates ...int) int {
	pick := candidates[0]
	for _, p := range candidates {
		if pid, err := winproc.PortOwner(p); err == nil && pid == 0 {
			pick = p
			break
		}
	}
	if ev.Port != nil {
		takenBy := ""
		if pick != candidates[0] {
			pid, _ := winproc.PortOwner(candidates[0])
			takenBy = fmt.Sprintf("%s pid %d", winproc.ProcessName(pid), pid)
		}
		ev.Port(name, candidates[0], pick, takenBy)
	}
	return pick
}

// pickPHPBase returns the first ports.php base whose worker ports for PHP v
// are all free (another stack or tool may already use 9830...).
func pickPHPBase(v discover.Version, workers int) int {
	for base := 9000; base <= 19000; base += 1000 {
		free := true
		for i := 0; i < workers && free; i++ {
			pid, err := winproc.PortOwner(model.PHPPort(base, v, i))
			free = err == nil && pid == 0
		}
		if free {
			return base
		}
	}
	return 9000
}

func quoteList(items []string) string {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = `"` + filepath.ToSlash(s) + `"`
	}
	return strings.Join(q, ", ")
}

func ensureSampleSite(www string) (string, error) {
	entries, err := os.ReadDir(www)
	if err == nil && len(entries) > 0 {
		return "", nil
	}
	dir := filepath.Join(www, "hello")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, os.WriteFile(filepath.Join(dir, "index.php"), []byte(sampleIndex), 0o644)
}

const sampleIndex = `<?php
// Sample site created by devstack. Delete this folder whenever you like.
$checks = [
    'PHP'        => PHP_VERSION,
    'SAPI'       => PHP_SAPI,
    'php.ini'    => php_ini_loaded_file() ?: '(none)',
    'pdo_mysql'  => extension_loaded('pdo_mysql') ? 'loaded' : 'missing',
    'opcache'    => function_exists('opcache_get_status') && opcache_get_status(false) ? 'on' : 'off',
    'Host'       => $_SERVER['HTTP_HOST'] ?? '',
    'Docroot'    => $_SERVER['DOCUMENT_ROOT'] ?? '',
];
?><!doctype html>
<meta charset="utf-8">
<title>hello from devstack</title>
<style>body{font:15px/1.5 system-ui,sans-serif;max-width:640px;margin:40px auto;padding:0 16px}td{padding:4px 12px 4px 0}</style>
<h1>It works.</h1>
<table>
<?php foreach ($checks as $k => $v): ?>
<tr><td><?= htmlspecialchars($k) ?></td><td><code><?= htmlspecialchars((string) $v) ?></code></td></tr>
<?php endforeach ?>
</table>
`

var stackTemplate = template.Must(template.New("stack.toml").Parse(`# devstack configuration. This file is the only source of truth:
# everything in etc/ is generated from it on each ` + "`devstack up`" + ` / ` + "`reload`" + `.

# Project folders. Each subfolder becomes <name>.localhost (browsers and curl)
# and, with services.dns = "on", <name>.<tld> (every program).
www = "{{.WWW}}"
tld = "test"

[paths]
# Searched in order for <service>/<any folder>/ (nginx.exe, php-cgi.exe, bin/mysqld.exe).
bin = [{{.Bins}}]

[services]
web   = "nginx"
# "" = newest installed, "8.4" = newest 8.4.x, "off" = don't run MySQL.
mysql = "{{.MySQL}}"
# "on" answers *.<tld> for every program on this machine. Turn it on from the
# app or with ` + "`devstack dns on`" + `: it needs one administrator approval.
dns   = "off"

[php]
default      = "{{.PHP}}"
workers      = 4      # php-cgi processes per PHP version
max_requests = 500    # each worker restarts after this many requests
# What a PHP version's php.ini starts with. After that, its php.ini decides:
# Servers > php.ini in the app (etc-custom\php\X.Y\php.ini).
extensions   = [{{.Extensions}}]

[php.ini]
# Any php.ini setting, for every PHP version; wins over each version's php.ini.
# Quote keys that contain a dot.
# memory_limit = "1G"
# "date.timezone" = "Asia/Kolkata"

[ports]
http  = {{.HTTP}}
mysql = {{.MySQLPort}}
php   = {{.PHPBase}}          # PHP X.Y workers listen on php + X*100 + Y*10 + n (9000 -> 8.3 on 9830...)

# Project folders kept outside www, by the name each is served under. Add them
# from the app (Sites > Add folder) or with ` + "`devstack project add <folder>`" + `.
# [projects]
# acme = "E:/clients/acme"
`))
