// Package aiguide writes a guide for AI coding agents into a site's project
// folder: how this machine serves the project (URL, PHP binary, database,
// logs, devstack commands) and what not to touch.
//
// The details are machine-specific, so they go in .devstack/ai-guide.md, which
// ignores itself in git (.devstack/.gitignore is "*"). AGENTS.md gets a short
// pointer block that is safe to commit, and CLAUDE.md imports AGENTS.md for
// Claude Code. devstack only ever edits between its markers, so a project's
// own instructions in those files are left alone.
package aiguide

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/harsha2805/devstack/internal/certs"
	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/packages"
	"github.com/harsha2805/devstack/internal/winproc"
)

const (
	// Dir is the per-project folder devstack owns.
	Dir = ".devstack"
	// File is the guide itself, inside Dir.
	File = "ai-guide.md"

	begin     = "<!-- devstack:begin -->"
	end       = "<!-- devstack:end -->"
	gitignore = "# Written by devstack: machine-specific, not for git.\n*\n"
)

// pointer is the AGENTS.md block: no paths, so it is right on every machine.
const pointer = `## Local environment (devstack)

This project is served locally by [devstack](https://devstack.holostack.in). Before running PHP, Composer,
database or web server commands, read ` + "`.devstack/ai-guide.md`" + ` if it exists. It has this machine's site
URL, PHP binary, database connection, log files and devstack commands. It is not committed, and devstack rewrites
it whenever the local setup changes.`

// Installed reports whether the project in dir has the guide.
func Installed(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, Dir, File))
	return err == nil
}

// Install writes the guide for site and adds the pointer blocks. Running it
// again refreshes everything.
func Install(st *model.Stack, site model.Site, cli string) error {
	if err := writeGuide(st, site, cli); err != nil {
		return err
	}
	if err := upsertBlock(filepath.Join(site.Dir, "AGENTS.md"), pointer); err != nil {
		return err
	}
	// Claude Code reads CLAUDE.md, not AGENTS.md; an import line bridges them.
	claude := filepath.Join(site.Dir, "CLAUDE.md")
	if b, err := os.ReadFile(claude); err == nil && strings.Contains(stripBlock(string(b)), "@AGENTS.md") {
		return nil
	}
	return upsertBlock(claude, "@AGENTS.md")
}

// Remove takes out everything Install added: the guide, its folder if empty,
// and the marked blocks. A file left empty is deleted.
func Remove(dir string) error {
	var errs []error
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		errs = append(errs, removeBlock(filepath.Join(dir, name)))
	}
	guideDir := filepath.Join(dir, Dir)
	errs = append(errs, removeFile(filepath.Join(guideDir, File)))
	ignore := filepath.Join(guideDir, ".gitignore")
	if b, err := os.ReadFile(ignore); err == nil && string(b) == gitignore {
		errs = append(errs, removeFile(ignore))
	}
	if entries, err := os.ReadDir(guideDir); err == nil && len(entries) == 0 {
		errs = append(errs, os.Remove(guideDir))
	}
	return errors.Join(errs...)
}

// Sync rewrites the guide of every site that has one, so it follows PHP
// version, port and path changes. It returns the slugs it changed.
func Sync(st *model.Stack, cli string) ([]string, error) {
	var changed []string
	var errs []error
	for _, site := range st.Sites {
		if !Installed(site.Dir) {
			continue
		}
		path := filepath.Join(site.Dir, Dir, File)
		old, _ := os.ReadFile(path)
		if string(old) == Render(st, site, cli) {
			continue
		}
		if err := writeGuide(st, site, cli); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", site.Slug, err))
			continue
		}
		changed = append(changed, site.Slug)
	}
	return changed, errors.Join(errs...)
}

// CLI is how the guide calls devstack: "devstack" when its folder is on PATH
// (a normal install), else the full path of devstack.exe next to the running
// program.
func CLI() string {
	exe, err := os.Executable()
	if err != nil {
		return "devstack"
	}
	dir := filepath.Dir(exe)
	p := filepath.Join(dir, "devstack.exe")
	if !fileExists(p) {
		return "devstack"
	}
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.EqualFold(strings.TrimRight(filepath.Clean(d), `\`), strings.TrimRight(dir, `\`)) {
			return "devstack"
		}
	}
	return p
}

func writeGuide(st *model.Stack, site model.Site, cli string) error {
	dir := filepath.Join(site.Dir, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ignore := filepath.Join(dir, ".gitignore")
	if !fileExists(ignore) {
		if err := os.WriteFile(ignore, []byte(gitignore), 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, File), []byte(Render(st, site, cli)), 0o644)
}

// Render is the guide's text for site.
func Render(st *model.Stack, site model.Site, cli string) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	url := st.URL(site)
	host := site.Slug + ".localhost"
	cmd := "devstack"

	w("# Local environment for %s", site.Name)
	w("")
	w("Written by devstack for this machine and rewritten when the setup changes. Don't edit it; it is not committed.")
	if cli != "" && cli != "devstack" {
		w("")
		w("`devstack` is not on PATH here. Run it as `%s` wherever this guide says `devstack`.", cli)
	}
	w("")
	w("## Site")
	w("")
	w("- URL: %s", url)
	for _, a := range site.Aliases {
		w("- Also answers to: %s", a)
	}
	w("- Project folder: `%s`", site.Dir)
	w("- Document root: `%s`", site.Docroot)
	if site.HasIndex {
		w("- Requests for files that don't exist go to `index.php` (front controller).")
	}
	w("- The site is already being served by nginx %s. Don't start another server (`php -S`, `artisan serve`, `npm run serve` for PHP).", st.Nginx.Version)
	if st.HTTPS {
		plain := strings.Replace(url, "https://", "http://", 1)
		plain = strings.Replace(plain, fmt.Sprintf(":%d", st.HTTPSPort), "", 1)
		if st.HTTPPort != 80 {
			plain += fmt.Sprintf(":%d", st.HTTPPort)
		}
		ca := "the devstack CA"
		if d, err := certs.Dir(); err == nil {
			ca = "`" + filepath.Join(d, "ca.crt") + "`"
		}
		w("- HTTPS uses devstack's local CA, which Windows programs trust. PHP, Node and Python don't by default: from code, call %s, or trust %s.", plain, ca)
	}
	if st.DNS {
		w("- `*.%s` names resolve for every program on this machine while devstack runs.", st.TLD)
	} else {
		w("- Browsers and curl resolve `*.localhost` on their own. If another tool can't, use `http://127.0.0.1:%d` with the header `Host: %s`.", st.HTTPPort, host)
	}

	if p := site.PHP; p != nil {
		php := filepath.Join(p.Build.Dir, "php.exe")
		w("")
		w("## PHP %s", p.Build.Version)
		w("")
		w("The site runs PHP %s (`php-cgi`). For command-line PHP and Composer use the same build with the same php.ini,", p.Build.Version)
		w("not whatever `php` is on PATH:")
		w("")
		w("```powershell")
		w("$env:Path = \"%s;\" + $env:Path", p.Build.Dir)
		w("$env:PHPRC = \"%s\"", filepath.Dir(p.INIPath))
		w("php -v; composer install")
		w("```")
		w("")
		w("```bash")
		w("export PATH=\"%s:$PATH\" PHPRC='%s'", msysPath(p.Build.Dir), filepath.Dir(p.INIPath))
		w("php -v; composer install")
		w("```")
		w("")
		w("Without PHPRC, `php.exe` loads no php.ini and has no extensions. For a single command, pass the ini instead:")
		w("`\"%s\" -c \"%s\" artisan migrate` (prefix `&` in PowerShell).", php, p.INIPath)
		w("")
		flavor := "NTS"
		if p.TS {
			flavor = "TS"
		}
		w("- Build: %s x64. Extension DLLs from anywhere else must be %s x64 builds for PHP %s.", flavor, flavor, p.Key)
		w("- php.ini: `%s`. It is generated from the `[php]` section of `%s`, which other projects share: ask before changing it, then reload.", p.INIPath, filepath.Join(st.Root, config.FileName))
		if len(p.Extensions) > 0 {
			w("- Extensions: %s", strings.Join(p.Extensions, ", "))
		}
		if len(p.Missing) > 0 {
			w("- Requested in `%s` but not loaded, because `%s` has no DLL for them: %s. Code that needs them fails on this PHP.%s",
				config.FileName, p.ExtDir, strings.Join(p.Missing, ", "), installHint(cmd, p.Missing))
		}
		w("- Errors: `%s`", filepath.Join(st.Logs, p.Name+"-error.log"))
	}

	w("")
	w("## Database")
	w("")
	env := readEnv(filepath.Join(site.Dir, ".env"))
	conn := env["DB_CONNECTION"]
	if conn != "" {
		where := ""
		if conn == "sqlite" {
			if db := env["DB_DATABASE"]; db != "" {
				where = fmt.Sprintf(", file `%s`", db)
			}
		} else if h := env["DB_HOST"]; h != "" {
			if port := env["DB_PORT"]; port != "" {
				h += ":" + port
			}
			where = fmt.Sprintf(" on `%s`", h)
		}
		w("- This project's `.env` sets `DB_CONNECTION=%s`%s.", conn, where)
		if keys := placeholders(env, conn); len(keys) > 0 {
			w("- `.env` still has placeholder values for %s. Don't connect, run migrations or seed until the user fills them in.", strings.Join(keys, ", "))
		}
		if drv, ok := pdoDriver[conn]; ok && site.PHP != nil {
			w("- %s", driverLine(site.PHP, drv, cmd))
		}
		if conn == "sqlsrv" {
			for _, l := range odbcLines(odbcDrivers()) {
				w("- %s", l)
			}
		}
	}
	localMySQL := (conn == "mysql" || conn == "mariadb") && isLocal(env["DB_HOST"])
	if m := st.MySQL; m != nil {
		w("- devstack's MySQL %s: host `127.0.0.1`, port `%d`, user `root`", m.Build.Version, m.Port)
		if port := env["DB_PORT"]; localMySQL && port != "" && port != strconv.Itoa(m.Port) {
			w("- `.env` has `DB_PORT=%s`, but devstack's MySQL listens on %d.", port, m.Port)
		}
		w("- Password: run `%s db creds`. Don't write it into committed files; `.env` is the place for it.", cmd)
		w("- Create databases with any client, for example `CREATE DATABASE %s;`", strings.ReplaceAll(site.Slug, "-", "_"))
		w("- MySQL log: `%s`", filepath.Join(st.Logs, "mysql.log"))
	} else if conn == "" || localMySQL {
		w("- devstack's MySQL is turned off. Ask the user before turning it on or adding one.")
	} else {
		w("- devstack's MySQL is turned off. This project's database is elsewhere, so it doesn't need it.")
	}

	w("")
	w("## Logs")
	w("")
	w("- Requests to this site: `%s`", site.AccessLog)
	w("- nginx errors: `%s`", filepath.Join(st.Logs, "nginx-error.log"))
	w("- devstack itself: `%s`", filepath.Join(st.Logs, "devstack.log"))

	w("")
	w("## devstack commands")
	w("")
	w("- `%s status`: what is running", cmd)
	w("- `%s reload`: apply changes to `stack.toml` or `.devstack.toml` without downtime", cmd)
	if site.PHP != nil {
		w("- `%s restart %s`: restart PHP for this site", cmd, site.PHP.Name)
		w("- `%s logs %s -n 50`: recent PHP errors", cmd, site.PHP.Name)
	}
	w("- `%s logs %s -n 50`: recent requests", cmd, site.Slug)

	w("")
	w("## Rules")
	w("")
	w("- Per-site settings go in `.devstack.toml` in the project folder (`docroot`, `php`, `aliases`), then reload.")
	w("- Never edit `%s`: it is regenerated on every reload. Extra nginx config goes in `%s`.", st.Etc, filepath.Join(st.EtcCustom, "nginx"))
	w("- Don't install, download or upgrade PHP, nginx or MySQL yourself; ask the user to do it in devstack.")
	w("- Ask before editing `stack.toml` or stopping the servers: other projects use them too.")
	return b.String()
}

// pdoDriver is the PHP extension each Laravel DB_CONNECTION needs.
var pdoDriver = map[string]string{
	"mysql": "pdo_mysql", "mariadb": "pdo_mysql", "pgsql": "pdo_pgsql", "sqlite": "pdo_sqlite", "sqlsrv": "pdo_sqlsrv",
}

// odbcDrivers lists the installed ODBC Drivers for SQL Server; tests replace it.
var odbcDrivers = winproc.ODBCSQLServerDrivers

// odbcLines says which ODBC driver pdo_sqlsrv connects through, and how that
// can differ on a machine with other drivers.
func odbcLines(installed []string) []string {
	used, ok := winproc.PHPODBCDriver(installed)
	if !ok {
		return []string{fmt.Sprintf("Microsoft's ODBC Driver for SQL Server isn't installed, so connections fail even with `pdo_sqlsrv` loaded. Ask the user to install it from %s (it needs administrator rights).", packages.ODBCDriverURL)}
	}
	out := []string{fmt.Sprintf("`pdo_sqlsrv` connects through %s, the only ODBC driver for SQL Server installed.", used)}
	if len(installed) > 1 {
		out[0] = fmt.Sprintf("Installed ODBC drivers: %s. `pdo_sqlsrv` connects through %s: with no `Driver=` in the connection it tries 17, then 18, then 13.",
			strings.Join(installed, ", "), used)
	}
	if !strings.Contains(used, " 18 ") {
		out = append(out, "ODBC Driver 18 encrypts by default and rejects a server certificate Windows doesn't trust, so a connection that works here can fail where only 18 is installed. "+
			"Set encryption explicitly in the connection (Laravel: `encrypt` and `trust_server_certificate` in the sqlsrv connection) so both behave the same.")
	}
	return out
}

// placeholder matches values left from a template: REPLACE_ME, changeme,
// your_password, <host>, xxx.
var placeholder = regexp.MustCompile(`(?i)^(replace[_-]?me|change[_-]?me|todo|tbd|x{3,}|your[_-].+|<.+>|placeholder)$`)

// placeholders lists the DB_ keys in env that still hold a placeholder.
func placeholders(env map[string]string, conn string) []string {
	keys := []string{"DB_HOST", "DB_PORT", "DB_DATABASE", "DB_USERNAME", "DB_PASSWORD"}
	if conn == "sqlite" {
		keys = []string{"DB_DATABASE"}
	}
	var out []string
	for _, k := range keys {
		if placeholder.MatchString(env[k]) {
			out = append(out, "`"+k+"`")
		}
	}
	return out
}

// downloadable lists the extensions devstack can download (sqlsrv, pdo_sqlsrv).
var downloadable = sync.OnceValue(func() []string {
	c, err := packages.Default()
	if err != nil {
		return nil
	}
	return c.ExtensionNames()
})

// driverLine says whether p loads drv, and how to get it when it doesn't.
func driverLine(p *model.PHPPool, drv, cmd string) string {
	if slices.ContainsFunc(p.Extensions, func(e string) bool { return model.ExtensionBase(e) == drv }) {
		return fmt.Sprintf("PHP %s loads `%s`, the driver it needs.", p.Build.Version, drv)
	}
	fix := installHint(cmd, []string{drv})
	if _, ok := model.ExtensionFile(p.Build, drv); ok {
		fix = fmt.Sprintf(" `%s` has it: ask the user to add `%s` to php.extensions in `%s`, then reload.", p.ExtDir, drv, config.FileName)
	} else if fix == "" {
		fix = " Ask the user to add it to PHP."
	}
	return fmt.Sprintf("PHP %s doesn't load `%s`, so database calls fail with \"could not find driver\".%s", p.Build.Version, drv, fix)
}

// installHint is the sentence asking the user to download the extensions in
// names that devstack can fetch, or "" when it can fetch none of them.
func installHint(cmd string, names []string) string {
	var get []string
	for _, n := range names {
		if b := model.ExtensionBase(n); slices.Contains(downloadable(), b) && !slices.Contains(get, b) {
			get = append(get, b)
		}
	}
	if len(get) == 0 {
		return ""
	}
	them := "them"
	if len(get) == 1 {
		them = "it"
	}
	return fmt.Sprintf(" Ask the user to run `%s pkg install ext %s`, which downloads %s for every PHP version.", cmd, strings.Join(get, " "), them)
}

// readEnv reads KEY=value lines from a .env file; a missing file reads as empty.
func readEnv(path string) map[string]string {
	env := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return env
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimPrefix(strings.TrimSpace(line), "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(k, "#") {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		} else if i := strings.Index(v, " #"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		env[strings.TrimSpace(k)] = v
	}
	return env
}

func isLocal(host string) bool {
	switch host {
	case "", "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

// msysPath turns C:\a\b into /c/a/b for Git Bash.
func msysPath(p string) string {
	p = filepath.ToSlash(p)
	if len(p) >= 2 && p[1] == ':' {
		p = "/" + strings.ToLower(p[:1]) + p[2:]
	}
	return p
}

// ---- marked blocks -------------------------------------------------------------

// upsertBlock puts body between devstack's markers in path, replacing an old
// block or appending one, and keeps the file's line endings.
func upsertBlock(path, body string) error {
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	text := string(old)
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	block := strings.ReplaceAll(begin+"\n"+body+"\n"+end, "\n", nl)
	if i, j, ok := blockSpan(text); ok {
		text = text[:i] + block + text[j:]
	} else {
		trimmed := strings.TrimRight(text, "\r\n")
		if trimmed != "" {
			trimmed += nl + nl
		}
		text = trimmed + block + nl
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func removeBlock(path string) error {
	old, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	text := stripBlock(string(old))
	if text == string(old) {
		return nil
	}
	if strings.TrimSpace(text) == "" {
		return os.Remove(path)
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// stripBlock removes the marked block and the blank lines it leaves behind.
func stripBlock(text string) string {
	i, j, ok := blockSpan(text)
	if !ok {
		return text
	}
	before := strings.TrimRight(text[:i], "\r\n")
	after := strings.TrimLeft(text[j:], "\r\n")
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	switch {
	case before == "":
		return after
	case after == "":
		return before + nl
	}
	return before + nl + nl + after
}

// blockSpan finds the block, markers included.
func blockSpan(text string) (int, int, bool) {
	i := strings.Index(text, begin)
	if i < 0 {
		return 0, 0, false
	}
	j := strings.Index(text[i:], end)
	if j < 0 {
		return 0, 0, false
	}
	return i, i + j + len(end), true
}

func removeFile(p string) error {
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
