package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeStack(t *testing.T, text string) string {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "www"), 0o755)
	if err := os.WriteFile(filepath.Join(root, FileName), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func readStack(t *testing.T, root string) string {
	b, err := os.ReadFile(filepath.Join(root, FileName))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSetService(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			"replace keeps comment",
			"[services]\nweb = \"nginx\"\ndns   = \"off\"   # local names\n\n[ports]\nhttp = 8080\n",
			"[services]\nweb = \"nginx\"\ndns   = \"on\"   # local names\n\n[ports]\nhttp = 8080\n",
		},
		{
			"insert into section",
			"[services]\nweb   = \"nginx\"\n# mysql comment\nmysql = \"off\"\n\n[ports]\nhttp = 8080\n",
			"[services]\nweb   = \"nginx\"\n# mysql comment\nmysql = \"off\"\ndns   = \"on\"\n\n[ports]\nhttp = 8080\n",
		},
		{
			"no section",
			"www = \"www\"\n",
			"www = \"www\"\n\n[services]\ndns   = \"on\"\n",
		},
		{
			"bom and crlf survive",
			"\ufeffwww = \"www\"\r\n\r\n[services]\r\ndns = \"off\"\r\n",
			"\ufeffwww = \"www\"\r\n\r\n[services]\r\ndns = \"on\"\r\n",
		},
	}
	for _, c := range cases {
		root := writeStack(t, c.in)
		if err := SetService(root, "dns", "on"); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := readStack(t, root); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
		cfg, err := Load(root)
		if err != nil || cfg.Services.DNS != "on" {
			t.Errorf("%s: loads as %q, %v", c.name, cfg.Services.DNS, err)
		}
	}
}

func TestSetServiceRollsBack(t *testing.T) {
	in := "[services]\ndns = \"off\"\n"
	root := writeStack(t, in)
	err := SetService(root, "dns", "maybe")
	if err == nil || !strings.Contains(err.Error(), "services.dns") {
		t.Fatalf("err = %v", err)
	}
	if got := readStack(t, root); got != in {
		t.Fatalf("file changed: %q", got)
	}
}

func TestSetKeyNumber(t *testing.T) {
	root := writeStack(t, "[ports]\nhttp  = 80\nhttps = 443   # TLS\n")
	if err := SetKey(root, "ports", "https", "8443"); err != nil {
		t.Fatal(err)
	}
	if got := readStack(t, root); got != "[ports]\nhttp  = 80\nhttps = 8443   # TLS\n" {
		t.Fatalf("got %q", got)
	}
}

func TestSetExtensions(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			"one line keeps comment",
			"[php]\nworkers    = 4\nextensions = [\"curl\", \"gd\"]  # enabled\n\n[ports]\nhttp = 8080\n",
			"[php]\nworkers    = 4\nextensions = [\"curl\", \"sqlsrv\"]  # enabled\n\n[ports]\nhttp = 8080\n",
		},
		{
			"several lines",
			"[php]\nextensions = [\n  \"curl\", # web\n  \"gd\",\n]\nworkers = 4\n",
			"[php]\nextensions = [\"curl\", \"sqlsrv\"]\nworkers = 4\n",
		},
		{
			"missing key, php.ini table after",
			"[php]\nworkers = 4\n\n[php.ini]\nmemory_limit = \"512M\"\n",
			"[php]\nworkers = 4\nextensions = [\"curl\", \"sqlsrv\"]\n\n[php.ini]\nmemory_limit = \"512M\"\n",
		},
		{
			"no php section",
			"www = \"www\"\n",
			"www = \"www\"\n\n[php]\nextensions = [\"curl\", \"sqlsrv\"]\n",
		},
	}
	for _, c := range cases {
		root := writeStack(t, c.in)
		if err := SetExtensions(root, []string{"curl", "sqlsrv"}); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := readStack(t, root); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestProjects(t *testing.T) {
	root := writeStack(t, "www = \"www\"\n\n[ports]\nhttp = 8080 # web\n")
	if err := AddProject(root, "acme", `E:\clients\Acme Site`); err != nil {
		t.Fatal(err)
	}
	if err := AddProject(root, "blog", "D:/blog"); err != nil {
		t.Fatal(err)
	}
	want := "www = \"www\"\n\n[ports]\nhttp = 8080 # web\n\n[projects]\nacme  = \"E:/clients/Acme Site\"\nblog  = \"D:/blog\"\n"
	if got := readStack(t, root); got != want {
		t.Fatalf("after add:\n%s\nwant:\n%s", got, want)
	}
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Projects["acme"] != "E:/clients/Acme Site" || len(cfg.Projects) != 2 {
		t.Errorf("loaded %v", cfg.Projects)
	}
	if err := AddProject(root, "Bad Name", "D:/x"); err == nil {
		t.Error("a name that isn't a DNS label was accepted")
	}
	if err := RemoveProject(root, "acme"); err != nil {
		t.Fatal(err)
	}
	if got := readStack(t, root); strings.Contains(got, "acme") || !strings.Contains(got, "blog") {
		t.Errorf("after remove:\n%s", got)
	}
	if err := RemoveProject(root, "acme"); err == nil {
		t.Error("removing a project that isn't listed should fail")
	}
}

func TestProjectNameValidated(t *testing.T) {
	root := writeStack(t, "[projects]\n\"My App\" = \"D:/x\"\n")
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "projects.My App") {
		t.Errorf("want a projects error, got %v", err)
	}
}
