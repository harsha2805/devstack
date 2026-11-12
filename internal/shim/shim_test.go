package shim

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPick(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "www", "app")
	api := filepath.Join(root, "www", "api")
	for _, d := range []string{filepath.Join(app, "src"), api} {
		os.MkdirAll(d, 0o755)
	}
	m := Map{
		Default: "8.3",
		PHP: map[string]Build{
			"8.3": {Version: "8.3.36", Exe: `C:\php\8.3\php.exe`},
			"8.5": {Version: "8.5.12", Exe: `C:\php\8.5\php.exe`},
		},
		Sites: []Site{{Name: "app", Dir: app, PHP: "8.3"}, {Name: "api", Dir: api, PHP: "8.5"}},
	}
	for _, c := range []struct {
		dir, force, want, site string
	}{
		{api, "", "8.5.12", "api"},
		{filepath.Join(app, "src"), "", "8.3.36", "app"},
		{root, "", "8.3.36", ""},                                 // outside every site: the default
		{filepath.Join(root, "www", "apix"), "", "8.3.36", ""},   // a prefix that isn't the folder
		{api, "8.3", "8.3.36", ""},                               // DEVSTACK_PHP wins
		{filepath.Join(root, "WWW", "API"), "", "8.5.12", "api"}, // case doesn't matter on Windows
	} {
		b, s, err := m.Pick(c.dir, c.force)
		if err != nil || b.Version != c.want || (s == nil) != (c.site == "") || (s != nil && s.Name != c.site) {
			t.Errorf("%s (force %q): got %s, %v, %v; want %s in %q", c.dir, c.force, b.Version, s, err, c.want, c.site)
		}
	}
	if _, _, err := m.Pick(api, "7.4"); err == nil {
		t.Error("forcing a PHP devstack doesn't run should fail")
	}
}

func TestPickThroughLink(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "code", "shop")
	os.MkdirAll(filepath.Join(real, "app"), 0o755)
	www := filepath.Join(root, "www")
	os.MkdirAll(www, 0o755)
	link := filepath.Join(www, "shop")
	// A junction is how people link a project in (mklink /J); it needs no
	// special rights, unlike a symlink.
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, real).CombinedOutput(); err != nil {
		if err := os.Symlink(real, link); err != nil {
			t.Skipf("can't make a link here: %v %s", err, out)
		}
	}
	m := Map{Default: "8.3", PHP: map[string]Build{"8.3": {Version: "8.3.36"}, "8.5": {Version: "8.5.12"}},
		Sites: []Site{{Name: "shop", Dir: link, PHP: "8.5"}}}
	if b, s, _ := m.Pick(filepath.Join(real, "app"), ""); b.Version != "8.5.12" || s == nil {
		t.Errorf("from the linked folder's real path: got %s, %v", b.Version, s)
	}
}

func TestFindRootAndMap(t *testing.T) {
	root := t.TempDir()
	site := filepath.Join(root, "www", "app")
	os.MkdirAll(site, 0o755)
	m := Map{Default: "8.3", PHP: map[string]Build{"8.3": {Version: "8.3.36"}}}
	if err := Write(root, m); err != nil {
		t.Fatal(err)
	}
	trusted := func(d string) bool { return d == root }
	if got := FindRoot(site, "", "elsewhere", trusted); got != root {
		t.Errorf("FindRoot from a site: %s", got)
	}
	if got := FindRoot(t.TempDir(), "", "elsewhere", trusted); got != "elsewhere" {
		t.Errorf("FindRoot outside a stack: %s", got)
	}
	if got := FindRoot(site, `D:\set`, "elsewhere", trusted); got != `D:\set` {
		t.Errorf("DEVSTACK_ROOT ignored: %s", got)
	}
	// A cloned project inside www shipping its own map, naming its own php.exe:
	// passed over for the trusted stack above it.
	if err := Write(site, Map{Default: "x", PHP: map[string]Build{"x": {Exe: `C:\evil.exe`}}}); err != nil {
		t.Fatal(err)
	}
	if got := FindRoot(filepath.Join(site, "src"), "", "elsewhere", trusted); got != root {
		t.Errorf("FindRoot used an untrusted map: %s", got)
	}
	if got := FindRoot(site, "", "elsewhere", func(string) bool { return false }); got != "elsewhere" {
		t.Errorf("FindRoot with nothing trusted: %s", got)
	}
	got, err := Load(root)
	if err != nil || got.PHP["8.3"].Version != "8.3.36" {
		t.Errorf("Load: %v, %v", got, err)
	}
	if _, err := Load(t.TempDir()); err == nil {
		t.Error("Load without a map should explain")
	}
}

func TestCheckPath(t *testing.T) {
	base := t.TempDir()
	shims := filepath.Join(base, "devstack", "shims")
	other := filepath.Join(base, "php")
	plain := filepath.Join(base, "tools")
	pf := filepath.Join(base, "Program Files", "devstack", "shims")
	for _, d := range []string{shims, other, plain, pf} {
		os.MkdirAll(d, 0o755)
	}
	os.WriteFile(filepath.Join(other, "php.exe"), nil, 0o644)

	for _, c := range []struct {
		name, system, user string
		ok                 bool
		blocker            string
		system_            bool
	}{
		{"first in user PATH", plain, shims + ";" + other, true, "", false},
		{"behind a user PHP", plain, other + ";" + shims, false, other, false},
		{"behind a system PHP", other, shims, false, other, true},
		{"not on PATH", other, plain, false, "", false},
		// 1.4.1 and earlier put the user-writable folder on the system PATH.
		{"user-writable folder on the system PATH", shims, "", false, "", false},
		{"admin copy first on the system PATH", pf + ";" + other, shims, true, "", false},
		{"admin copy behind a system PHP", other + ";" + pf, "", false, other, true},
		{"trailing slash and quotes", "", `"` + shims + `\"`, true, "", false},
	} {
		s := CheckPath(shims, pf, c.system, c.user)
		if s.OK() != c.ok || s.Blocker != c.blocker || s.BlockerSystem != c.system_ {
			t.Errorf("%s: %+v", c.name, s)
		}
		if (s.Problem() == "") != c.ok {
			t.Errorf("%s: problem %q", c.name, s.Problem())
		}
	}

	if s := CheckPath(shims, pf, shims, ""); !s.Unsafe || !s.NeedsAdmin() {
		t.Errorf("shims on the system PATH: %+v", s)
	}
	if s := CheckPath(shims, pf, pf, ""); s.System != pf {
		t.Errorf("admin copy: %+v", s)
	}
	if got := Remove(shims+";"+other, shims); got != other {
		t.Errorf("Remove: %s", got)
	}
	if got := Prepend(other+";"+shims+";;"+plain, shims); got != shims+";"+other+";"+plain {
		t.Errorf("Prepend: %s", got)
	}
}

func TestFindComposer(t *testing.T) {
	bin := t.TempDir()
	for _, v := range []string{"2.8.9", "2.8.12", "2.10.0", "junk"} {
		os.MkdirAll(filepath.Join(bin, "composer", v), 0o755)
		os.WriteFile(filepath.Join(bin, "composer", v, "composer.phar"), nil, 0o644)
	}
	os.MkdirAll(filepath.Join(bin, "composer", "3.0.0"), 0o755) // no phar
	got, ok := FindComposer(bin)
	if want := filepath.Join(bin, "composer", "2.10.0", "composer.phar"); !ok || got != want {
		t.Errorf("FindComposer = %s, %v; want %s", got, ok, want)
	}
	if _, ok := FindComposer(t.TempDir()); ok {
		t.Error("found a composer in an empty bin")
	}
}
