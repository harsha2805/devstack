package render

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/sites"
)

var update = flag.Bool("update", false, "rewrite testdata/golden")

// fixture builds a stack under a temp root with fake php/nginx/mysql folders.
func fixture(t *testing.T) *model.Stack {
	t.Helper()
	root := t.TempDir()
	phpDir := filepath.Join(root, "bin", "php", "8.3")
	os.MkdirAll(filepath.Join(phpDir, "ext"), 0o755)
	for _, e := range []string{"curl", "mbstring", "pdo_mysql", "opcache"} {
		os.WriteFile(filepath.Join(phpDir, "ext", "php_"+e+".dll"), nil, 0o644)
	}
	found := discover.Result{Builds: map[discover.Kind][]discover.Build{
		discover.Nginx: {{Kind: discover.Nginx, Version: discover.Version{1, 30, 4}, Dir: filepath.Join(root, "bin", "nginx", "1.30")}},
		discover.PHP:   {{Kind: discover.PHP, Version: discover.Version{8, 3, 36}, Dir: phpDir}},
		discover.MySQL: {{Kind: discover.MySQL, Version: discover.Version{8, 4, 3}, Dir: filepath.Join(root, "bin", "mysql", "8.4"), Flavor: "mysql"}},
	}}
	cfg := config.Default()
	cfg.Root = root
	cfg.Ports.HTTP = 8080
	cfg.PHP.Workers = 2
	cfg.PHP.Extensions = []string{"curl", "mbstring", "pdo_mysql", "gd"}
	cfg.PHP.INI = map[string]string{"memory_limit": "1G", "date.timezone": "Asia/Kolkata"}
	www := filepath.Join(root, "www")
	list := []sites.Site{
		{Name: "My_App", Slug: "my-app", Dir: filepath.Join(www, "My_App"), Docroot: filepath.Join(www, "My_App", "public"), HasIndex: true},
		{Name: "static", Slug: "static", Dir: filepath.Join(www, "static"), Docroot: filepath.Join(www, "static")},
	}
	st, err := model.Resolve(cfg, found, list, nil)
	if err != nil {
		t.Fatal(err)
	}
	st.IPv6 = true
	return st
}

func TestGolden(t *testing.T) {
	st := fixture(t)
	files, err := Files(st)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 7 { // nginx.conf, fastcgi.conf, index.html, 2 sites, php.ini, my.ini
		t.Fatalf("got %d files", len(files))
	}
	for path, content := range files {
		rel, _ := filepath.Rel(st.Etc, path)
		golden := filepath.Join("testdata", "golden", strings.ReplaceAll(filepath.ToSlash(rel), "/", "__"))
		// Golden files use a fixed root so they don't depend on the temp dir.
		got := bytes.ReplaceAll(content, []byte(filepath.ToSlash(st.Root)), []byte("ROOT"))
		got = bytes.ReplaceAll(got, []byte(st.Root), []byte("ROOT"))
		if *update {
			os.MkdirAll(filepath.Dir(golden), 0o755)
			if err := os.WriteFile(golden, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%v (run go test ./internal/render -update)", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from %s (run with -update if intended)\n--- got ---\n%s", rel, golden, got)
		}
	}
}

func TestWritePrunesAndIsIdempotent(t *testing.T) {
	st := fixture(t)
	stale := filepath.Join(st.Etc, "nginx", "sites", "gone.conf")
	os.MkdirAll(filepath.Dir(stale), 0o755)
	os.WriteFile(stale, []byte("old"), 0o644)

	res, err := Write(st)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 7 || len(res.Removed) != 1 || res.Removed[0] != "nginx/sites/gone.conf" {
		t.Errorf("first write: %+v", res)
	}
	res, err = Write(st)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed() {
		t.Errorf("second write should change nothing: %+v", res)
	}
	for _, d := range []string{"tmp/nginx/logs", "tmp/php/sessions", "etc-custom/nginx", "logs/sites"} {
		if _, err := os.Stat(filepath.Join(st.Root, d)); err != nil {
			t.Errorf("runtime dir %s missing", d)
		}
	}
}

func TestModelDetails(t *testing.T) {
	st := fixture(t)
	p := st.DefaultPHP
	if p.Upstream != "php83" || p.Ports[0] != 9830 || p.Ports[1] != 9831 {
		t.Errorf("pool naming/ports: %+v", p)
	}
	if strings.Join(p.Extensions, ",") != "curl,mbstring,pdo_mysql" || !p.Opcache {
		t.Errorf("extension detection: %v opcache=%v", p.Extensions, p.Opcache)
	}
	if len(st.Warnings) != 1 || !strings.Contains(st.Warnings[0], "gd") {
		t.Errorf("missing gd should warn: %q", st.Warnings)
	}
	if got := st.URL(st.Sites[0]); got != "http://my-app.localhost:8080" {
		t.Errorf("URL = %s", got)
	}
	if !strings.HasSuffix(filepath.ToSlash(st.MySQL.DataDir), "data/mysql/8.4") {
		t.Errorf("datadir = %s", st.MySQL.DataDir)
	}
}
