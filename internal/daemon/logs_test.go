package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotateIfLarge(t *testing.T) {
	dir := t.TempDir()
	big, small := filepath.Join(dir, "big.log"), filepath.Join(dir, "small.log")
	os.WriteFile(big, []byte(strings.Repeat("x", 100)), 0o644)
	os.WriteFile(big+".1", []byte("older"), 0o644)
	os.WriteFile(small, []byte("x"), 0o644)
	for _, p := range []string{big, small, filepath.Join(dir, "missing.log")} {
		if err := rotateIfLarge(p, 10); err != nil {
			t.Fatal(err)
		}
	}
	if b, _ := os.ReadFile(big + ".1"); len(b) != 100 {
		t.Errorf("big.log.1 has %d bytes; want the rotated 100", len(b))
	}
	if _, err := os.Stat(big); err == nil {
		t.Error("big.log still there")
	}
	if _, err := os.Stat(small + ".1"); err == nil {
		t.Error("small.log was rotated")
	}
}
