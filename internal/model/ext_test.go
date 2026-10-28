package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/discover"
)

func phpBuild(t *testing.T, version discover.Version, files ...string) discover.Build {
	t.Helper()
	b := discover.Build{Kind: discover.PHP, Version: version, Dir: t.TempDir()}
	os.MkdirAll(filepath.Join(b.Dir, "ext"), 0o755)
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(b.Dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return b
}

func TestExtensionFile(t *testing.T) {
	php85 := phpBuild(t, discover.Version{8, 5, 12}, "ext/php_curl.dll", "ext/php_sqlsrv_85_nts_x64.dll", "ext/php_pdo_sqlsrv.dll")
	for name, want := range map[string]string{
		"curl":                  "curl",
		"sqlsrv":                "sqlsrv_85_nts_x64", // Microsoft's build for this version
		"sqlsrv_83_nts_x64":     "sqlsrv_85_nts_x64", // listed for 8.3, found for 8.5
		"pdo_sqlsrv_83_nts_x64": "pdo_sqlsrv",        // devstack's download
		"pdo_sqlsrv":            "pdo_sqlsrv",
	} {
		if got, ok := ExtensionFile(php85, name); !ok || got != want {
			t.Errorf("%s: got %q, %v; want %q", name, got, ok, want)
		}
	}
	if got, ok := ExtensionFile(php85, "gd"); ok {
		t.Errorf("gd: found %q without a DLL", got)
	}

	ts := phpBuild(t, discover.Version{8, 4, 1}, "php8ts.dll", "ext/php_sqlsrv_84_ts_x64.dll")
	if got, ok := ExtensionFile(ts, "sqlsrv"); !ok || got != "sqlsrv_84_ts_x64" {
		t.Errorf("thread-safe build: got %q, %v", got, ok)
	}
	if _, ok := ExtensionFile(php85, "sqlsrv_84_ts_x64"); !ok {
		t.Error("a TS name should still find the NTS DLL for an NTS build")
	}
}

func TestNewPoolExtensions(t *testing.T) {
	b := phpBuild(t, discover.Version{8, 5, 12}, "ext/php_curl.dll", "ext/php_sqlsrv.dll")
	cfg := config.Default()
	cfg.PHP.Extensions = []string{"curl", "sqlsrv", "sqlsrv_83_nts_x64", "pdo_sqlsrv_83_nts_x64"}
	st := &Stack{Etc: t.TempDir()}
	p := newPool(cfg, st, b)
	if got := p.Extensions; len(got) != 2 || got[0] != "curl" || got[1] != "sqlsrv" {
		t.Errorf("loaded %v; want curl, sqlsrv once", got)
	}
	if len(p.Missing) != 1 || p.Missing[0] != "pdo_sqlsrv_83_nts_x64" || len(st.Warnings) != 1 {
		t.Errorf("missing %v, warnings %v", p.Missing, st.Warnings)
	}
}
