package supervisor

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"testing"
	"time"
)

// The test binary doubles as the supervised process: with DEVSTACK_HELPER set
// it acts as a tiny server instead of running tests.
func TestMain(m *testing.M) {
	switch os.Getenv("DEVSTACK_HELPER") {
	case "listen":
		ln, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("DEVSTACK_PORT"))
		if err != nil {
			os.Exit(3)
		}
		if d := os.Getenv("DEVSTACK_EXIT_AFTER"); d != "" {
			ms, _ := strconv.Atoi(d)
			time.Sleep(time.Duration(ms) * time.Millisecond)
			ln.Close()
			os.Exit(0)
		}
		for {
			c, err := ln.Accept()
			if err == nil {
				c.Close()
			}
		}
	case "fail":
		os.Exit(7)
	}
	os.Exit(m.Run())
}

func freePort(t *testing.T) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func helperSpec(t *testing.T, name, mode string, port int, extraEnv ...string) Spec {
	exe, _ := os.Executable()
	env := append(os.Environ(), "DEVSTACK_HELPER="+mode, "DEVSTACK_PORT="+strconv.Itoa(port))
	return Spec{
		Name: name, Group: name, Path: exe, Env: append(env, extraEnv...), Log: io.Discard,
		Ready: func(ctx context.Context) error {
			var d net.Dialer
			c, err := d.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
			if err == nil {
				c.Close()
			}
			return err
		},
		ReadyTimeout: 5 * time.Second,
		StopTimeout:  2 * time.Second,
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestStartStop(t *testing.T) {
	s := New(nil)
	port := freePort(t)
	u, err := s.Add(context.Background(), helperSpec(t, "web", "listen", port))
	if err != nil {
		t.Fatal(err)
	}
	st := u.Status()
	if st.State != Running || st.PID == 0 {
		t.Fatalf("status after start: %+v", st)
	}
	s.StopAll(context.Background())
	if u.Status().State != Stopped {
		t.Fatalf("state after stop: %s", u.Status().State)
	}
	if c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond); err == nil {
		c.Close()
		t.Fatal("port still open after stop")
	}
}

func TestFirstStartFailureIsNotRetried(t *testing.T) {
	s := New(nil)
	_, err := s.Add(context.Background(), helperSpec(t, "bad", "fail", freePort(t)))
	if err == nil {
		t.Fatal("expected an error")
	}
	u := s.Find("bad")[0]
	time.Sleep(1500 * time.Millisecond) // longer than the first backoff
	if st := u.Status(); st.State != Failed || st.Restarts != 0 {
		t.Fatalf("want failed with no retries, got %+v", st)
	}
}

func TestCrashRespawns(t *testing.T) {
	s := New(nil)
	defer s.StopAll(context.Background())
	u, err := s.Add(context.Background(), helperSpec(t, "web", "listen", freePort(t)))
	if err != nil {
		t.Fatal(err)
	}
	first := u.Status().PID
	p, _ := os.FindProcess(first)
	p.Kill()
	waitFor(t, "respawn", func() bool {
		st := u.Status()
		return st.State == Running && st.PID != first && st.Restarts == 1
	})
	if u.Status().LastError == "" {
		t.Error("crash should be recorded in LastError")
	}
}

func TestExpectedExitRespawnsWithoutCrash(t *testing.T) {
	s := New(nil)
	defer s.StopAll(context.Background())
	sp := helperSpec(t, "php", "listen", freePort(t), "DEVSTACK_EXIT_AFTER=300")
	sp.Expected = func(code int) bool { return code == 0 }
	u, err := s.Add(context.Background(), sp)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "two recycles", func() bool { return u.Status().Restarts >= 2 })
	u.mu.Lock()
	crashes := len(u.crashes)
	u.mu.Unlock()
	if crashes != 0 {
		t.Errorf("expected exits counted as %d crashes", crashes)
	}
}

func TestReplaceKeepsPosition(t *testing.T) {
	s := New(nil)
	defer s.StopAll(context.Background())
	a, _ := s.Add(context.Background(), helperSpec(t, "a", "listen", freePort(t)))
	s.Add(context.Background(), helperSpec(t, "b", "listen", freePort(t)))
	n, err := s.Replace(context.Background(), a, helperSpec(t, "a", "listen", freePort(t)))
	if err != nil {
		t.Fatal(err)
	}
	if units := s.Units(); units[0] != n || units[1].Spec().Name != "b" {
		t.Errorf("order after replace: %v, %v", units[0].Spec().Name, units[1].Spec().Name)
	}
	if a.Status().State != Stopped {
		t.Error("old unit should be stopped")
	}
}
