package control

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSystemShimCopy(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	old := time.Now().Add(-time.Hour)
	for _, n := range shimNames {
		os.WriteFile(filepath.Join(src, n), []byte("shim v2"), 0o755)
		os.WriteFile(filepath.Join(dst, n), []byte("shim v1"), 0o755)
		os.Chtimes(filepath.Join(src, n), old, old)
	}
	if sameShims(src, dst) {
		t.Fatal("different shims compare equal")
	}
	for _, n := range shimNames {
		if err := replaceFile(filepath.Join(src, n), filepath.Join(dst, n)); err != nil {
			t.Fatal(err)
		}
	}
	if !sameShims(src, dst) {
		t.Error("copied shims compare different, so the app would keep asking to fix them")
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "php.exe")); string(b) != "shim v2" {
		t.Errorf("php.exe = %q", b)
	}
	if _, err := os.Stat(filepath.Join(dst, "php.exe.old")); err == nil {
		t.Error("php.exe.old left behind")
	}
}
