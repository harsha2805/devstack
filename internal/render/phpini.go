package render

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/phpini"
)

// Owned are the php.ini settings devstack always sets itself, after the
// user's php.ini, because they point into the stack's folders.
var Owned = []string{"extension_dir", "error_log", "sys_temp_dir", "upload_tmp_dir", "session.save_path"}

// defaults are devstack's development settings, written into a new php.ini.
var defaults = [][2]string{
	{"engine", "On"},
	{"short_open_tag", "Off"},
	{"expose_php", "Off"},
	{"default_charset", `"UTF-8"`},
	{"variables_order", `"EGPCS"`},
	{"request_order", `"GP"`},
	{"memory_limit", "512M"},
	{"max_execution_time", "120"},
	{"max_input_time", "120"},
	{"max_input_vars", "5000"},
	{"post_max_size", "256M"},
	{"upload_max_filesize", "256M"},
	{"max_file_uploads", "50"},
	{"error_reporting", "E_ALL"},
	{"display_errors", "On"},
	{"display_startup_errors", "On"},
	{"log_errors", "On"},
	{"html_errors", "On"},
	{"cgi.fix_pathinfo", "1"},
	{"user_ini.cache_ttl", "5"}, // a site's .user.ini applies within seconds, not PHP's 5 minutes
	{"date.timezone", "UTC"},
	{"session.use_strict_mode", "1"},
	{"session.cookie_httponly", "1"},
	{"opcache.enable", "1"},
	{"opcache.enable_cli", "0"},
	{"opcache.memory_consumption", "128"},
	{"opcache.max_accelerated_files", "20000"},
	{"opcache.validate_timestamps", "1"},
	{"opcache.revalidate_freq", "0"}, // check file times on every request so edits show up at once
}

// UserINI is p's own php.ini: the file in etc-custom when it's a whole
// php.ini, or else a new one. fresh says it isn't on disk yet.
//
// A new one is PHP's own php.ini-development with devstack's settings, and
// every extension in p's ext folder listed: the ones p loads (php.extensions,
// at first) as extension=name, the rest as ;extension=name. 0.2.7's short
// add-on file, if there is one, is merged into it.
func UserINI(p *model.PHPPool) (text string, fresh bool) {
	old := model.UserINI(p)
	if phpini.Full(old) {
		return old, false
	}
	base := "[PHP]\n"
	if b, err := os.ReadFile(filepath.Join(p.Build.Dir, "php.ini-development")); err == nil {
		base = string(b)
	}
	text = userHeader(p) + strings.ReplaceAll(base, "\r\n", "\n")
	for _, kv := range defaults {
		text = phpini.Set(text, kv[0], kv[1])
	}
	listed := map[string]bool{}
	for _, e := range p.Extensions {
		text = phpini.Enable(text, e, false)
		listed[model.ExtensionBase(e)] = true
	}
	for _, e := range available(p) {
		if !listed[model.ExtensionBase(e)] && !phpini.Listed(text, e) {
			text = phpini.AddOff(text, e)
		}
		listed[model.ExtensionBase(e)] = true
	}
	if p.Opcache {
		text = phpini.Enable(text, "opcache", true)
	}
	for _, kv := range phpini.Settings(old) {
		text = phpini.Set(text, kv[0], kv[1])
	}
	return text, true
}

// available lists the extensions in p's ext folder by the name php.ini loads
// them with, one per extension ("sqlsrv", not also "sqlsrv_83_nts_x64").
// opcache (a zend_extension) and PHP's own test extensions are left out.
func available(p *model.PHPPool) []string {
	m, _ := filepath.Glob(filepath.Join(p.ExtDir, "php_*.dll"))
	sort.Strings(m)
	var out []string
	for _, f := range m {
		n := phpini.ExtName(f)
		if n == "opcache" || n == "dl_test" || n == "zend_test" {
			continue
		}
		if r, ok := model.ExtensionFile(p.Build, model.ExtensionBase(n)); ok {
			n = r
		}
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

func userHeader(p *model.PHPPool) string {
	return `; php.ini for PHP ` + p.Key + `: every site on PHP ` + p.Key + ` uses it, and so do php and
; composer in their terminals. It's yours to edit. Save it and PHP ` + p.Key + ` restarts
; with it within a few seconds while the servers run.
;
; Extensions: remove the ; in front of an extension=... line to turn it on, or
; put one there to turn it off. They're listed under "Dynamic Extensions".
;
; devstack sets these itself, after this file, because they point into its
; folders: ` + strings.Join(Owned, ", ") + `.
; [php.ini] in stack.toml applies to every PHP version and wins over this file.
; For one site only, put a .user.ini in that site's docroot (public\ for
; Laravel) instead.
;
; Delete this file to start over from devstack's defaults.

`
}

// writeUserINIs creates the php.ini of each PHP version that has none yet.
func writeUserINIs(st *model.Stack) error {
	for _, p := range st.PHP {
		if text, fresh := UserINI(p); fresh {
			if err := WriteUserINI(p, text); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteUserINI saves text as p's php.ini, with Windows line endings. A short
// 0.2.7 file it replaces is kept as php.ini.old.
func WriteUserINI(p *model.PHPPool, text string) error {
	if old := model.UserINI(p); !phpini.Full(old) && strings.TrimSpace(old) != "" {
		if err := os.WriteFile(p.CustomINI+".old", []byte(old), 0o644); err != nil {
			return err
		}
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", "\r\n")
	return writeAtomic(p.CustomINI, []byte(text))
}
