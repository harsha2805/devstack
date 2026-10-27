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
	"strings"

	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/model"
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
	w("- Browsers and curl resolve `*.localhost` on their own. If another tool can't, use `http://127.0.0.1:%d` with the header `Host: %s`.", st.HTTPPort, host)

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
		w("- php.ini: `%s`. It is generated from the `[php]` section of `%s`, which other projects share: ask before changing it, then reload.", p.INIPath, filepath.Join(st.Root, config.FileName))
		if len(p.Extensions) > 0 {
			w("- Extensions: %s", strings.Join(p.Extensions, ", "))
		}
		w("- Errors: `%s`", filepath.Join(st.Logs, p.Name+"-error.log"))
	}

	if m := st.MySQL; m != nil {
		w("")
		w("## Database (MySQL %s)", m.Build.Version)
		w("")
		w("- Host `127.0.0.1`, port `%d`, user `root`", m.Port)
		w("- Password: run `%s db creds`. Don't write it into committed files; `.env` is the place for it.", cmd)
		w("- Create databases with any client, for example `CREATE DATABASE %s;`", strings.ReplaceAll(site.Slug, "-", "_"))
		w("- Log: `%s`", filepath.Join(st.Logs, "mysql.log"))
	} else {
		w("")
		w("## Database")
		w("")
		w("MySQL is turned off in this stack. Ask before adding one.")
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
