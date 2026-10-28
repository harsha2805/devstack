package aiguide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/sites"
)

func testStack(t *testing.T) (*model.Stack, model.Site) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "www", "my-app")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pool := &model.PHPPool{
		Build:   discover.Build{Kind: "php", Version: discover.Version{8, 3, 36}, Dir: filepath.Join(root, "bin", "php", "8.3.36")},
		Key:     "8.3",
		Name:    "php-8.3",
		INIPath: filepath.Join(root, "etc", "php", "8.3", "php.ini"),
	}
	site := model.Site{
		Site:      sites.Site{Name: "My_App", Slug: "my-app", Dir: dir, Docroot: filepath.Join(dir, "public"), HasIndex: true},
		PHP:       pool,
		AccessLog: filepath.Join(root, "logs", "sites", "my-app.access.log"),
	}
	st := &model.Stack{
		Root: root, WWW: filepath.Join(root, "www"), Etc: filepath.Join(root, "etc"), EtcCustom: filepath.Join(root, "etc-custom"),
		Logs: filepath.Join(root, "logs"), HTTPPort: 8080,
		Nginx: discover.Build{Version: discover.Version{1, 30, 6}},
		PHP:   []*model.PHPPool{pool}, DefaultPHP: pool,
		MySQL: &model.MySQL{Build: discover.Build{Version: discover.Version{8, 4, 13}}, Port: 3307},
		Sites: []model.Site{site},
	}
	return st, site
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInstallRemoveFreshProject(t *testing.T) {
	st, site := testStack(t)
	if err := Install(st, site, "devstack"); err != nil {
		t.Fatal(err)
	}
	if !Installed(site.Dir) {
		t.Fatal("not installed")
	}
	guide := read(t, filepath.Join(site.Dir, Dir, File))
	for _, want := range []string{"http://my-app.localhost:8080", "PHP 8.3.36", "port `3307`", "devstack db creds", "php-8.3-error.log", "PHPRC"} {
		if !strings.Contains(guide, want) {
			t.Errorf("guide lacks %q", want)
		}
	}
	if read(t, filepath.Join(site.Dir, Dir, ".gitignore")) != gitignore {
		t.Error("gitignore")
	}
	if !strings.Contains(read(t, filepath.Join(site.Dir, "AGENTS.md")), ".devstack/ai-guide.md") {
		t.Error("AGENTS.md pointer")
	}
	if !strings.Contains(read(t, filepath.Join(site.Dir, "CLAUDE.md")), "@AGENTS.md") {
		t.Error("CLAUDE.md import")
	}

	if err := Remove(site.Dir); err != nil {
		t.Fatal(err)
	}
	if left, _ := os.ReadDir(site.Dir); len(left) != 0 {
		t.Fatalf("left behind: %v", left)
	}
}

func TestKeepsProjectInstructions(t *testing.T) {
	st, site := testStack(t)
	agents := "# Project rules\r\n\r\nUse tabs.\r\n"
	claude := "See @AGENTS.md\n"
	os.WriteFile(filepath.Join(site.Dir, "AGENTS.md"), []byte(agents), 0o644)
	os.WriteFile(filepath.Join(site.Dir, "CLAUDE.md"), []byte(claude), 0o644)

	for i := 0; i < 2; i++ { // installing twice must not duplicate the block
		if err := Install(st, site, "devstack"); err != nil {
			t.Fatal(err)
		}
	}
	got := read(t, filepath.Join(site.Dir, "AGENTS.md"))
	if !strings.HasPrefix(got, agents) || strings.Count(got, begin) != 1 {
		t.Fatalf("AGENTS.md:\n%q", got)
	}
	if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Error("mixed line endings in a CRLF file")
	}
	if read(t, filepath.Join(site.Dir, "CLAUDE.md")) != claude {
		t.Error("CLAUDE.md already imports AGENTS.md and should be untouched")
	}

	if err := Remove(site.Dir); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(site.Dir, "AGENTS.md")); got != agents {
		t.Fatalf("after remove: %q", got)
	}
	if _, err := os.Stat(filepath.Join(site.Dir, Dir)); !os.IsNotExist(err) {
		t.Error(".devstack folder left behind")
	}
}

func TestSync(t *testing.T) {
	st, site := testStack(t)
	if changed, _ := Sync(st, "devstack"); len(changed) != 0 {
		t.Fatalf("synced a site without a guide: %v", changed)
	}
	Install(st, site, "devstack")
	if changed, _ := Sync(st, "devstack"); len(changed) != 0 {
		t.Fatalf("rewrote an unchanged guide: %v", changed)
	}
	st.HTTPPort = 9090
	changed, err := Sync(st, "devstack")
	if err != nil || len(changed) != 1 {
		t.Fatalf("changed %v, err %v", changed, err)
	}
	if !strings.Contains(read(t, filepath.Join(site.Dir, Dir, File)), "my-app.localhost:9090") {
		t.Error("port not updated")
	}
}

func TestMissingExtensions(t *testing.T) {
	st, site := testStack(t)
	site.PHP.ExtDir = filepath.Join(site.PHP.Build.Dir, "ext")
	site.PHP.Extensions = []string{"curl", "mbstring"}
	site.PHP.Missing = []string{"sqlsrv_83_nts_x64", "pdo_sqlsrv_83_nts_x64"}
	guide := Render(st, site, "devstack")
	for _, want := range []string{
		"Build: NTS x64",
		"but not loaded, because `" + site.PHP.ExtDir + "` has no DLL for them: sqlsrv_83_nts_x64, pdo_sqlsrv_83_nts_x64",
		"`devstack pkg install ext sqlsrv pdo_sqlsrv`",
	} {
		if !strings.Contains(guide, want) {
			t.Errorf("guide lacks %q", want)
		}
	}
}

func TestDatabaseFromEnv(t *testing.T) {
	odbc, orig := "", odbcDriver
	odbcDriver = func() (string, bool) { return odbc, odbc != "" }
	t.Cleanup(func() { odbcDriver = orig })

	st, site := testStack(t)
	st.MySQL = nil
	site.PHP.Extensions = []string{"pdo_mysql"}
	env := "APP_NAME=x\n# DB_CONNECTION=mysql\nDB_CONNECTION=sqlsrv\nDB_HOST=\"sql.example.com\"\nDB_PORT=1433 # default\n"
	os.WriteFile(filepath.Join(site.Dir, ".env"), []byte(env), 0o644)

	guide := Render(st, site, "devstack")
	for _, want := range []string{
		"`DB_CONNECTION=sqlsrv` on `sql.example.com:1433`",
		"doesn't load `pdo_sqlsrv`, so database calls fail",
		"`devstack pkg install ext pdo_sqlsrv`",
		"ODBC Driver for SQL Server isn't installed",
		"This project's database is elsewhere",
	} {
		if !strings.Contains(guide, want) {
			t.Errorf("guide lacks %q", want)
		}
	}
	if strings.Contains(guide, "Ask the user before turning it on") {
		t.Error("asks for MySQL that the project doesn't use")
	}

	odbc = "ODBC Driver 18 for SQL Server"
	site.PHP.Extensions = []string{"pdo_sqlsrv_83_nts_x64"}
	guide = Render(st, site, "devstack")
	for _, want := range []string{"loads `pdo_sqlsrv`, the driver it needs", "ODBC Driver 18 for SQL Server is installed"} {
		if !strings.Contains(guide, want) {
			t.Errorf("guide lacks %q", want)
		}
	}

	os.WriteFile(filepath.Join(site.Dir, ".env"), []byte("DB_CONNECTION=mysql\nDB_HOST=127.0.0.1\n"), 0o644)
	if guide := Render(st, site, "devstack"); !strings.Contains(guide, "Ask the user before turning it on") {
		t.Error("MySQL on this machine is off, but the guide doesn't say to ask")
	}
}

func TestMsysPath(t *testing.T) {
	if got := msysPath(`C:\devstack\bin\php\8.3.36`); got != "/c/devstack/bin/php/8.3.36" {
		t.Fatal(got)
	}
}
