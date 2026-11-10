package control

import (
	"os"
	"path/filepath"
	"testing"
)

// The MAIL_ keys SiteMail sets land in the MAIL_ group: replaced where they
// are, missing ones after the last MAIL_ line.
func TestSetEnvMailGroup(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	in := "DB_CONNECTION=mysql\n\nMAIL_MAILER=log\nMAIL_FROM_ADDRESS=\"hello@example.com\"\n\nAPP_KEY=x\n"
	os.WriteFile(p, []byte(in), 0o644)
	if err := SetEnv(p, map[string]string{"MAIL_MAILER": "smtp", "MAIL_HOST": "127.0.0.1", "MAIL_PORT": "1025"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	want := "DB_CONNECTION=mysql\n\nMAIL_MAILER=smtp\nMAIL_FROM_ADDRESS=\"hello@example.com\"\nMAIL_HOST=127.0.0.1\nMAIL_PORT=1025\n\nAPP_KEY=x\n"
	if string(b) != want {
		t.Errorf("got\n%q\nwant\n%q", b, want)
	}
}
