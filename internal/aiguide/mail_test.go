package aiguide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/model"
)

func TestGuideMail(t *testing.T) {
	st, site := testStack(t)
	if guide := Render(st, site, "devstack"); !strings.Contains(guide, "devstack's Mailpit is off") {
		t.Error("the guide doesn't say Mailpit is off")
	}

	st.Mail = &model.Mail{Build: discover.Build{Version: discover.Version{1, 33, 1}}, SMTPPort: 1025, InboxPort: 8025}
	env := filepath.Join(site.Dir, ".env")
	os.WriteFile(env, []byte("MAIL_MAILER=log\n"), 0o644)
	guide := Render(st, site, "devstack")
	for _, want := range []string{
		"devstack runs Mailpit 1.33.1, which catches mail",
		"`.env` sends mail with `MAIL_MAILER=log`, not to Mailpit",
		"`MAIL_MAILER=smtp`, `MAIL_HOST=127.0.0.1`, `MAIL_PORT=1025`",
		"`GET http://127.0.0.1:8025/api/v1/message/latest`",
		"mailpit.log",
	} {
		if !strings.Contains(guide, want) {
			t.Errorf("guide lacks %q", want)
		}
	}

	os.WriteFile(env, []byte("MAIL_MAILER=smtp\nMAIL_HOST=127.0.0.1\nMAIL_PORT=1025\nMAIL_SCHEME=null\n"), 0o644)
	if guide := Render(st, site, "devstack"); !strings.Contains(guide, "`.env` sends mail to Mailpit") {
		t.Error("the guide doesn't see the .env sends to Mailpit")
	}
}

func TestUsesMailpit(t *testing.T) {
	for _, c := range []struct {
		m    EnvMailer
		want bool
	}{
		{EnvMailer{Mailer: "smtp", Host: "127.0.0.1", Port: "1025"}, true},
		{EnvMailer{Mailer: "smtp", Host: "localhost", Port: "1025"}, true},
		{EnvMailer{Mailer: "smtp", Host: "127.0.0.1", Port: "1025", Scheme: "smtps"}, false},
		{EnvMailer{Mailer: "smtp", Host: "mailpit", Port: "1025"}, false},
		{EnvMailer{Mailer: "smtp", Host: "127.0.0.1", Port: "2525"}, false},
		{EnvMailer{Mailer: "log"}, false},
	} {
		if got := c.m.UsesMailpit(1025); got != c.want {
			t.Errorf("%+v: got %v", c.m, got)
		}
	}
}
