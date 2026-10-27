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
