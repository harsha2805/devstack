package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/harsha2805/devstack/internal/aiguide"
	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/stack"
)

// SetMail turns Mailpit on (the newest installed version) or off, and
// reloads a running stack, which starts or stops it. PHP restarts too: its
// php.ini sends mail() to Mailpit only while Mailpit runs.
func SetMail(ctx context.Context, root string, on bool) error {
	value := config.Off
	if on {
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}
		if _, err := discover.Scan(ctx, stack.BinRoots(cfg)).Pick(discover.Mailpit, ""); err != nil {
			return errors.New("Mailpit isn't installed; install it first")
		}
		value = ""
	}
	if err := config.SetService(root, "mail", value); err != nil {
		return err
	}
	return reloadIfRunning(ctx, root)
}

// SiteMailResult is what SiteMail changed.
type SiteMailResult struct {
	// ConfigCached: Laravel's bootstrap/cache/config.php exists, so the app
	// keeps sending the old way until `php artisan config:clear`.
	ConfigCached bool `json:"config_cached"`
}

// SiteMail points a site's .env at Mailpit: MAIL_MAILER=smtp, MAIL_HOST and
// MAIL_PORT, and MAIL_SCHEME=null in place of smtps. A login is left as it
// is, since Mailpit accepts any.
func SiteMail(ctx context.Context, root, slug string) (SiteMailResult, error) {
	l, err := stack.Load(ctx, root)
	if err != nil {
		return SiteMailResult{}, err
	}
	st := l.Stack
	if st == nil || st.Mail == nil {
		return SiteMailResult{}, errors.New("Mailpit is off; turn it on first")
	}
	var dir string
	for _, s := range st.Sites {
		if s.Slug == slug {
			dir = s.Dir
		}
	}
	if dir == "" {
		return SiteMailResult{}, fmt.Errorf("no site %q", slug)
	}
	env := filepath.Join(dir, ".env")
	if _, err := os.Stat(env); err != nil {
		return SiteMailResult{}, fmt.Errorf("%s has no .env file", slug)
	}
	values := map[string]string{"MAIL_MAILER": "smtp", "MAIL_HOST": "127.0.0.1", "MAIL_PORT": strconv.Itoa(st.Mail.SMTPPort)}
	if m, _ := aiguide.EnvMail(dir); m.Scheme == "smtps" {
		values["MAIL_SCHEME"] = "null" // TLS from the first byte; Mailpit takes plain SMTP
	}
	if err := SetEnv(env, values); err != nil {
		return SiteMailResult{}, err
	}
	_, cacheErr := os.Stat(filepath.Join(dir, "bootstrap", "cache", "config.php"))
	return SiteMailResult{ConfigCached: cacheErr == nil}, nil
}
