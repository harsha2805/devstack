//go:build windows

package api

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/harsha2805/devstack/internal/winproc"
)

// A daemon killed at Windows shutdown leaves daemon.json behind, and after the
// next boot its pid can belong to an unrelated process. That must read as
// "not running", not as a daemon to send requests to.
func TestReadStateReusedPID(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data", "devstack"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func() {
		// Our own pid: alive, but not a daemon.
		b, _ := json.Marshal(State{PID: os.Getpid(), Addr: "127.0.0.1:1"})
		if err := os.WriteFile(StatePath(root), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write()
	if _, err := ReadState(root); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("stale state with a reused pid: got %v, want ErrNotRunning", err)
	}
	if _, err := os.Stat(StatePath(root)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale state file not removed: %v", err)
	}

	lock, err := winproc.LockFile(LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	write()
	if _, err := ReadState(root); err != nil {
		t.Fatalf("live daemon (lock held): got %v, want state", err)
	}
}
