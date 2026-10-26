package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/harsha2805/devstack/internal/winproc"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLoadDefaults(t *testing.T) {
	root := writeConfig(t, "")
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Root != winproc.LongPath(root) || cfg.WWW != "www" || cfg.TLD != "test" || cfg.Ports.HTTP != 80 || cfg.PHP.Workers != 4 {
		t.Errorf("defaults not applied: %+v", cfg)
	}
	if len(cfg.PHP.Extensions) != len(DefaultExtensions) {
		t.Errorf("default extensions missing: %v", cfg.PHP.Extensions)
	}
}

func TestLoadOverrides(t *testing.T) {
	root := writeConfig(t, `
www = "D:/projects"
[php]
extensions = ["curl"]
[php.ini]
memory_limit = "1G"
"date.timezone" = "Asia/Kolkata"
[ports]
http = 8080
`)
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WWW != "D:/projects" || cfg.Ports.HTTP != 8080 || cfg.Ports.MySQL != 3306 {
		t.Errorf("overrides wrong: %+v", cfg)
	}
	if len(cfg.PHP.Extensions) != 1 {
		t.Errorf("extensions should replace the default list, got %v", cfg.PHP.Extensions)
	}
	if cfg.PHP.INI["date.timezone"] != "Asia/Kolkata" || cfg.PHP.INI["memory_limit"] != "1G" {
		t.Errorf("php.ini map wrong: %v", cfg.PHP.INI)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := map[string]string{
		"unknown key":    "wat = 1\n",
		"bad tld":        "tld = \"Te st\"\n",
		"localhost tld":  "tld = \"localhost\"\n",
		"apache":         "[services]\nweb = \"apache\"\n",
		"mysql version":  "[services]\nmysql = \"latest\"\n",
		"workers":        "[php]\nworkers = 0\n",
		"dll extension":  "[php]\nextensions = [\"php_curl.dll\"]\n",
		"port range":     "[ports]\nhttp = 70000\n",
		"dotted ini key": "[php.ini]\ndate.timezone = \"UTC\"\n", // a nested table, not a string
	}
	for name, body := range cases {
		if _, err := Load(writeConfig(t, body)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestValidateReportsAll(t *testing.T) {
	c := Default()
	c.TLD = ""
	c.PHP.Workers = 0
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "tld") || !strings.Contains(err.Error(), "php.workers") {
		t.Errorf("want both problems reported, got %v", err)
	}
}

func TestLoadSite(t *testing.T) {
	dir := t.TempDir()
	if _, found, err := LoadSite(dir); found || err != nil {
		t.Errorf("missing file: found=%v err=%v", found, err)
	}
	os.WriteFile(filepath.Join(dir, SiteFileName), []byte(`docroot = "../etc"`), 0o644)
	if _, _, err := LoadSite(dir); err == nil {
		t.Error("docroot escaping the site should fail")
	}
}

func TestFindRoot(t *testing.T) {
	// t.TempDir can be an 8.3 short path (C:\Users\ABCDEF~1\...). FindRoot must
	// expand it, because nginx refuses to open short-name paths.
	root := winproc.LongPath(writeConfig(t, ""))
	sub := filepath.Join(root, "www", "site")
	os.MkdirAll(sub, 0o755)
	t.Chdir(sub)
	t.Setenv("DEVSTACK_ROOT", "")
	got, err := FindRoot("")
	if err != nil || got != root {
		t.Errorf("FindRoot from subfolder = %q, %v; want %q", got, err, root)
	}
	t.Setenv("DEVSTACK_ROOT", sub)
	if got, _ := FindRoot(""); got != sub {
		t.Errorf("DEVSTACK_ROOT ignored: %q", got)
	}
	if got, _ := FindRoot(root); got != root {
		t.Errorf("flag ignored: %q", got)
	}
}
