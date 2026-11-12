package control

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetEnv(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	in := "APP_NAME=Shop\r\n\r\nDB_CONNECTION=mysql\r\nDB_HOST=127.0.0.1\r\nDB_DATABASE=shop\r\nDB_USERNAME=root\r\n# DB_PASSWORD=old\r\n\r\nMAIL_MAILER=log\r\n"
	os.WriteFile(p, []byte(in), 0o644)
	if err := SetEnv(p, map[string]string{"DB_USERNAME": "shop_app", "DB_PASSWORD": "abc123"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	want := "APP_NAME=Shop\r\n\r\nDB_CONNECTION=mysql\r\nDB_HOST=127.0.0.1\r\nDB_DATABASE=shop\r\nDB_USERNAME=shop_app\r\nDB_PASSWORD=abc123\r\n# DB_PASSWORD=old\r\n\r\nMAIL_MAILER=log\r\n"
	if string(b) != want {
		t.Errorf("got\n%q\nwant\n%q", b, want)
	}
}

func TestSiteUserName(t *testing.T) {
	if got := SiteUserName("chess-analysis"); got != "chess_analysis" {
		t.Errorf("got %q", got)
	}
	a, b := SiteUserName("a-very-long-site-name-that-goes-past-the-limit"), SiteUserName("a-very-long-site-name-that-goes-past-the-other")
	if a == b {
		t.Errorf("two long slugs with the same start share user %s", a)
	}
	if got := SiteUserName("a-very-long-site-name-that-goes-past-the-limit"); len(got) != 32 {
		t.Errorf("not cut to 32: %q", got)
	}
	if got := grantName("my_db%x"); got != `my\_db\%x` {
		t.Errorf("grantName = %q", got)
	}
}
