package render

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/model"
)

// withMailpit is the fixture with Mailpit running on ports.smtp 2525.
func withMailpit(t *testing.T, change func(*config.Stack)) *model.Stack {
	t.Helper()
	st := fixtureWith(t, change)
	st.Mail = &model.Mail{Build: discover.Build{Kind: discover.Mailpit, Version: discover.Version{1, 33, 1}}, SMTPPort: 2525, InboxPort: 8025}
	return st
}

func TestPHPINISendsMailToMailpit(t *testing.T) {
	ini := func(st *model.Stack) string {
		files, err := Files(st)
		if err != nil {
			t.Fatal(err)
		}
		return string(files[st.PHP[0].INIPath])
	}
	if got := ini(fixture(t)); strings.Contains(got, "smtp_port") {
		t.Errorf("php.ini without Mailpit sets smtp_port:\n%s", got)
	}
	got := ini(withMailpit(t, nil))
	for _, want := range []string{"SMTP = 127.0.0.1\r\n", "smtp_port = 2525\r\n", `sendmail_from = "devstack@localhost"`} {
		if !strings.Contains(got, want) {
			t.Errorf("php.ini lacks %q:\n%s", want, got)
		}
	}
	// A sender the user set is theirs.
	got = ini(withMailpit(t, func(c *config.Stack) { c.PHP.INI["sendmail_from"] = "me@example.test" }))
	if strings.Contains(got, "devstack@localhost") {
		t.Errorf("php.ini overrides the user's sendmail_from:\n%s", got)
	}
}

func TestMailpitPortsClaimed(t *testing.T) {
	root := t.TempDir()
	found := discover.Result{Builds: map[discover.Kind][]discover.Build{
		discover.Nginx:   {{Kind: discover.Nginx, Version: discover.Version{1, 30, 4}, Dir: filepath.Join(root, "nginx")}},
		discover.PHP:     {{Kind: discover.PHP, Version: discover.Version{8, 3, 36}, Dir: filepath.Join(root, "php")}},
		discover.Mailpit: {{Kind: discover.Mailpit, Version: discover.Version{1, 33, 1}, Dir: filepath.Join(root, "mailpit")}},
	}}
	cfg := config.Default()
	cfg.Root = root
	cfg.Services.MySQL = config.Off
	cfg.PHP.Extensions = nil // the fake PHP folder has no DLLs to warn about
	st, err := model.Resolve(cfg, found, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mail == nil || st.Mail.SMTPPort != 1025 || st.Mail.InboxURL() != "http://127.0.0.1:8025" {
		t.Fatalf("Mail = %+v", st.Mail)
	}
	cfg.Ports.Inbox = cfg.Ports.HTTP
	if _, err := model.Resolve(cfg, found, nil, nil); err == nil || !strings.Contains(err.Error(), "ports.inbox") {
		t.Errorf("inbox on the HTTP port: err = %v", err)
	}
	cfg.Ports.Inbox = 8025
	cfg.Services.Mail = config.Off
	if st, _ := model.Resolve(cfg, found, nil, nil); st.Mail != nil {
		t.Error(`services.mail = "off" still runs Mailpit`)
	}
	// Not installed: nothing runs, and nothing to warn about.
	cfg.Services.Mail = ""
	delete(found.Builds, discover.Mailpit)
	if st, err := model.Resolve(cfg, found, nil, nil); err != nil || st.Mail != nil || len(st.Warnings) > 0 {
		t.Errorf("no Mailpit installed: Mail = %v, err = %v, warnings = %v", st.Mail, err, st.Warnings)
	}
}
