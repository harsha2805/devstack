package sites

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"IKEA_BIX_CDP":        "ikea-bix-cdp",
		"Flipkart_Retail_API": "flipkart-retail-api",
		"kbs":                 "kbs",
		"my app (old)":        "my-app-old",
		"--x--":               "x",
		"___":                 "",
		"café":                "caf",
		"a__b":                "a-b",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
	long := ""
	for i := 0; i < 70; i++ {
		long += "a"
	}
	if got := Slugify(long); len(got) != 63 {
		t.Errorf("long name not capped at 63: %d", len(got))
	}
}

func TestScan(t *testing.T) {
	www := t.TempDir()
	mk := func(parts ...string) {
		if err := os.MkdirAll(filepath.Join(append([]string{www}, parts...)...), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(content string, parts ...string) {
		if err := os.WriteFile(filepath.Join(append([]string{www}, parts...)...), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("Laravel_App", "public")
	write("<?php", "Laravel_App", "public", "index.php")
	mk("plain")
	mk("custom", "dist")
	write("docroot = \"dist\"\nphp = \"8.1\"\naliases = [\"API\"]\n", "custom", ".devstack.toml")
	mk("laravel-app") // collides with Laravel_App
	mk(".hidden")
	mk("_skip")
	mk("broken")
	write("nope = 1\n", "broken", ".devstack.toml")

	got, warns, err := Scan(www)
	if err != nil {
		t.Fatal(err)
	}
	bySlug := map[string]Site{}
	for _, s := range got {
		bySlug[s.Slug] = s
	}
	if len(got) != 5 {
		t.Fatalf("got %d sites, want 5: %+v", len(got), got)
	}
	if s := bySlug["laravel-app-2"]; s.Name == "" {
		t.Errorf("collision not suffixed: %+v", bySlug)
	}
	app := bySlug["laravel-app"]
	if app.Name == "Laravel_App" && (filepath.Base(app.Docroot) != "public" || !app.HasIndex) {
		t.Errorf("Laravel_App docroot/index wrong: %+v", app)
	}
	if s := bySlug["plain"]; s.Docroot != s.Dir || s.HasIndex {
		t.Errorf("plain: %+v", s)
	}
	c := bySlug["custom"]
	if filepath.Base(c.Docroot) != "dist" || c.PHP != "8.1" || len(c.Aliases) != 1 || c.Aliases[0] != "api" {
		t.Errorf("custom overrides not applied: %+v", c)
	}
	if _, ok := bySlug["broken"]; !ok {
		t.Error("a site with a bad .devstack.toml should still be served")
	}
	if len(warns) != 2 { // collision + unknown key
		t.Errorf("want 2 warnings, got %q", warns)
	}
}

// A project linked into www with a junction (mklink /J) is a site; a link to a
// file is not.
func TestScanFollowsJunctions(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junctions are Windows-only")
	}
	www, elsewhere := t.TempDir(), t.TempDir()
	if err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(www, "linked-app"), elsewhere).Run(); err != nil {
		t.Skipf("mklink /J: %v", err)
	}
	os.WriteFile(filepath.Join(www, "notes.txt"), []byte("x"), 0o644)
	list, _, err := Scan(www)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Slug != "linked-app" {
		t.Fatalf("got %+v", list)
	}
}
