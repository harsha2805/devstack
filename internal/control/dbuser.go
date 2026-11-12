package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/harsha2805/devstack/internal/aiguide"
	"github.com/harsha2805/devstack/internal/services"
	"github.com/harsha2805/devstack/internal/stack"
)

// SiteUser is the MySQL login SiteDBUser made for a site.
type SiteUser struct {
	User     string `json:"user"`
	Database string `json:"database"`
	// ConfigCached: Laravel's bootstrap/cache/config.php exists, so the app
	// keeps the old login until `php artisan config:clear`.
	ConfigCached bool `json:"config_cached"`
}

// SiteDBUser gives a site a MySQL user of its own: named after the site, with
// a new random password and every privilege on the database its .env names,
// and nothing else. The database is created if missing. The login goes into
// the site's .env as DB_USERNAME and DB_PASSWORD. Run again, it sets a new
// password for the same user.
func SiteDBUser(ctx context.Context, root, slug string) (SiteUser, error) {
	l, err := stack.Load(ctx, root)
	if err != nil {
		return SiteUser{}, err
	}
	st := l.Stack
	if st == nil || st.MySQL == nil {
		return SiteUser{}, errors.New("MySQL is off; turn it on first")
	}
	var dir string
	for _, s := range st.Sites {
		if s.Slug == slug {
			dir = s.Dir
		}
	}
	if dir == "" {
		return SiteUser{}, fmt.Errorf("no site %q", slug)
	}
	env, ok := aiguide.EnvDatabase(dir)
	if !ok || !env.UsesMySQL(st.MySQL.Port) {
		return SiteUser{}, fmt.Errorf("%s's .env doesn't point at devstack's MySQL (127.0.0.1:%d)", slug, st.MySQL.Port)
	}
	if env.Name == "" {
		return SiteUser{}, fmt.Errorf("%s's .env names no database; set DB_DATABASE first", slug)
	}
	if !dbNameRe.MatchString(env.Name) {
		return SiteUser{}, fmt.Errorf("devstack can only make users for database names of letters, digits, _ and $ (.env has %q)", env.Name)
	}
	user := SiteUserName(slug)
	// A site named "root", or a .env naming a system schema, must not hand the
	// site MySQL's own accounts or tables.
	if reservedDBUsers[strings.ToLower(user)] {
		return SiteUser{}, fmt.Errorf("%q is one of MySQL's own users; rename the site to give it a user", user)
	}
	if systemSchemas[strings.ToLower(env.Name)] {
		return SiteUser{}, fmt.Errorf("%s is one of MySQL's own databases; set DB_DATABASE to the site's database", env.Name)
	}
	pw := services.RandomPassword(24)

	db, err := services.RootDB(ctx, st)
	if err != nil {
		return SiteUser{}, fmt.Errorf("connect to MySQL as root: %w", err)
	}
	defer db.Close()
	// Names are checked above and the password is letters and digits, so
	// they're safe to put in the statements as they are.
	stmts := []string{"CREATE DATABASE IF NOT EXISTS `" + env.Name + "`"}
	for _, host := range []string{"localhost", "127.0.0.1"} {
		account := "'" + user + "'@'" + host + "'"
		stmts = append(stmts,
			"CREATE USER IF NOT EXISTS "+account+" IDENTIFIED BY '"+pw+"'",
			"ALTER USER "+account+" IDENTIFIED BY '"+pw+"'",
			// _ and % are wildcards in a database-level grant: escape them.
			"GRANT ALL PRIVILEGES ON `"+grantName(env.Name)+"`.* TO "+account,
		)
	}
	for _, q := range stmts {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return SiteUser{}, fmt.Errorf("%s: %w", strings.SplitN(q, " IDENTIFIED", 2)[0], err)
		}
	}
	if err := SetEnv(filepath.Join(dir, ".env"), map[string]string{"DB_USERNAME": user, "DB_PASSWORD": pw}); err != nil {
		return SiteUser{}, fmt.Errorf("user %s is ready, but writing .env failed: %w", user, err)
	}
	_, cacheErr := os.Stat(filepath.Join(dir, "bootstrap", "cache", "config.php"))
	return SiteUser{User: user, Database: env.Name, ConfigCached: cacheErr == nil}, nil
}

// envOrder is the order missing keys are added in, Laravel's own.
var envOrder = []string{"DB_USERNAME", "DB_PASSWORD", "MAIL_MAILER", "MAIL_SCHEME", "MAIL_HOST", "MAIL_PORT"}

var (
	reservedDBUsers = map[string]bool{"root": true, "mysql.sys": true, "mysql.session": true, "mysql.infoschema": true, "mariadb.sys": true}
	systemSchemas   = map[string]bool{"mysql": true, "sys": true, "information_schema": true, "performance_schema": true}
)

// dbNameRe is what SiteDBUser accepts as a database name.
var dbNameRe = regexp.MustCompile(`^[A-Za-z0-9_$]{1,64}$`)

// SiteUserName is the MySQL user for a site: its slug with _ for -, at most
// 32 characters (MySQL's limit). A longer slug keeps its first 23 characters
// and adds _ and 8 hex digits of its hash, so two long slugs that start alike
// get users of their own.
func SiteUserName(slug string) string {
	u := strings.ReplaceAll(slug, "-", "_")
	if len(u) > 32 {
		sum := sha256.Sum256([]byte(slug))
		u = u[:23] + "_" + hex.EncodeToString(sum[:4])
	}
	return u
}

func grantName(db string) string {
	return strings.NewReplacer(`_`, `\_`, `%`, `\%`).Replace(db)
}

// SetEnv sets KEY=value lines in a .env file: each key's existing line is
// replaced in place, and missing keys go after the last line of their group
// (DB_ keys after the last DB_ line, MAIL_ after MAIL_), or at the end.
// Comments, order and line endings stay as they were.
func SetEnv(path string, values map[string]string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(b)
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	done := map[string]bool{}
	// The keys' shared prefix, "DB_"; "" when they don't share one.
	group, first := "", true
	for k := range values {
		g, _, _ := strings.Cut(k, "_")
		if first {
			group, first = g+"_", false
		} else if g+"_" != group {
			group = ""
		}
	}
	lastInGroup := -1
	for i, l := range lines {
		t := strings.TrimPrefix(strings.TrimSpace(l), "export ")
		k, _, ok := strings.Cut(t, "=")
		k = strings.TrimSpace(k)
		if !ok || strings.HasPrefix(k, "#") {
			continue
		}
		if group != "" && strings.HasPrefix(k, group) {
			lastInGroup = i
		}
		if v, want := values[k]; want && !done[k] {
			lines[i] = k + "=" + v
			done[k] = true
		}
	}
	var add []string
	for _, k := range envOrder { // a stable order for the keys devstack sets
		if v, ok := values[k]; ok && !done[k] {
			add = append(add, k+"="+v)
			done[k] = true
		}
	}
	var rest []string
	for k := range values {
		if !done[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	for _, k := range rest {
		add = append(add, k+"="+values[k])
	}
	if len(add) > 0 {
		at := lastInGroup + 1
		if lastInGroup < 0 {
			at = len(lines)
			for at > 0 && lines[at-1] == "" {
				at--
			}
		}
		lines = append(lines[:at], append(add, lines[at:]...)...)
	}
	out := strings.Join(lines, nl)
	if !strings.HasSuffix(out, nl) {
		out += nl
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), info.Mode().Perm())
}
