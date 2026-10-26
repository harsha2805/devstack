// Package control starts and stops a stack the same way for every front end:
// the CLI and the desktop app both go through here.
package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/harsha2805/devstack/internal/api"
	"github.com/harsha2805/devstack/internal/logfile"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/render"
	"github.com/harsha2805/devstack/internal/stack"
	"github.com/harsha2805/devstack/internal/supervisor"
	"github.com/harsha2805/devstack/internal/winproc"
)

// Prepare loads the stack, checks its ports, and renders etc/. It runs in the
// caller so config errors show up where the user is looking.
func Prepare(ctx context.Context, root string) (*model.Stack, render.Result, error) {
	l, err := stack.Load(ctx, root)
	if err != nil {
		return nil, render.Result{}, err
	}
	st := l.Stack
	if conflicts := PortConflicts(st); len(conflicts) > 0 {
		return st, render.Result{}, fmt.Errorf("ports already in use:\n  %s\nchange [ports] in stack.toml, or stop the other program",
			strings.Join(conflicts, "\n  "))
	}
	res, err := render.Write(st)
	return st, res, err
}

// Spawn starts `<exe> --root <root> daemon` in the background and waits until
// its API answers. exe must be a devstack binary that understands "daemon".
func Spawn(ctx context.Context, root, exe string) (*api.Client, error) {
	if exe == "" {
		var err error
		if exe, err = os.Executable(); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "logs"), 0o755); err != nil {
		return nil, err
	}
	// Anything the daemon prints before its own log is open lands here.
	out, err := os.OpenFile(filepath.Join(root, "logs", "devstack-daemon.out"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	defer out.Close()
	cmd := exec.Command(exe, "--root", root, "daemon")
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = out, out
	pid, err := winproc.StartDetached(cmd)
	if err != nil {
		return nil, fmt.Errorf("start daemon: %w", err)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := api.Connect(root); err == nil && c.State.PID == pid {
			return c, nil
		}
		if !winproc.Alive(pid) {
			return nil, daemonDied(root)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil, errors.New("daemon did not come up in 15s; see logs\\devstack.log")
}

// Follow polls the daemon until startup finishes, calling onSettled once per
// service as it becomes running or failed. It returns an error if any failed.
func Follow(ctx context.Context, root string, c *api.Client, onSettled func(Group)) error {
	seen := map[string]bool{}
	for {
		s, err := c.Status(ctx)
		if err != nil {
			if !winproc.Alive(c.State.PID) {
				return daemonDied(root)
			}
			return err
		}
		for _, g := range Groups(s.Units) {
			if seen[g.Name] || (g.State != supervisor.Running && g.State != supervisor.Failed) {
				continue
			}
			seen[g.Name] = true
			if onSettled != nil {
				onSettled(g)
			}
		}
		if s.Phase != api.PhaseStarting {
			if len(s.Errors) > 0 {
				return fmt.Errorf("%d service(s) failed to start; fix the cause, then reload", len(s.Errors))
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// Up starts the stack, or applies changes if it is already running.
func Up(ctx context.Context, root, exe string, onSettled func(Group)) error {
	if c, err := api.Connect(root); err == nil {
		_, err := c.Reload(ctx)
		return err
	}
	if _, _, err := Prepare(ctx, root); err != nil {
		return err
	}
	c, err := Spawn(ctx, root, exe)
	if err != nil {
		return err
	}
	return Follow(ctx, root, c, onSettled)
}

// Down stops the stack and waits for the daemon to exit. tick, if set, is
// called while waiting. Stopping a stack that isn't running is not an error.
func Down(ctx context.Context, root string, tick func()) error {
	c, err := api.Connect(root)
	if errors.Is(err, api.ErrNotRunning) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := c.Shutdown(ctx); err != nil {
		return err
	}
	deadline := time.Now().Add(3 * time.Minute)
	for winproc.Alive(c.State.PID) {
		if time.Now().After(deadline) {
			return fmt.Errorf("daemon pid %d still running after 3m; see logs\\devstack.log", c.State.PID)
		}
		if tick != nil {
			tick()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return nil
}

func daemonDied(root string) error {
	tail := logfile.Tail(filepath.Join(root, "logs", "devstack.log"), 8)
	if tail == "" {
		tail = logfile.Tail(filepath.Join(root, "logs", "devstack-daemon.out"), 8)
	}
	return fmt.Errorf("daemon exited during startup:\n  %s", strings.ReplaceAll(strings.TrimRight(tail, "\r\n"), "\n", "\n  "))
}

// PortConflicts lists ports the stack needs that another process holds.
func PortConflicts(st *model.Stack) []string {
	var out []string
	ports := st.Ports()
	keys := make([]int, 0, len(ports))
	for p := range ports {
		keys = append(keys, p)
	}
	sort.Ints(keys)
	for _, p := range keys {
		pid, err := winproc.PortOwner(p)
		if err != nil || pid == 0 {
			continue
		}
		who := winproc.ProcessName(pid)
		if who == "" {
			who = "unknown process"
		}
		out = append(out, fmt.Sprintf("%d (%s) is used by %s pid %d", p, ports[p], who, pid))
	}
	return out
}

// Group is one service as people think of it, folding php-cgi workers together.
type Group struct {
	Name      string           `json:"name"`
	Version   string           `json:"version"`
	State     supervisor.State `json:"state"`
	Ports     string           `json:"ports"` // "8080" or "9830-9833"
	PIDs      []int            `json:"pids"`
	Restarts  int              `json:"restarts"`
	LastError string           `json:"last_error,omitempty"`
}

// PIDText is "1234" for one process, "4 procs" for a pool.
func (g Group) PIDText() string {
	switch {
	case len(g.PIDs) > 1:
		return fmt.Sprintf("%d procs", len(g.PIDs))
	case len(g.PIDs) == 1 && g.PIDs[0] != 0:
		return fmt.Sprint(g.PIDs[0])
	}
	return "-"
}

func Groups(units []supervisor.Status) []Group {
	var out []Group
	var ports [][]int
	idx := map[string]int{}
	for _, u := range units {
		i, ok := idx[u.Group]
		if !ok {
			i = len(out)
			idx[u.Group] = i
			out = append(out, Group{Name: u.Group, Version: u.Version, State: u.State})
			ports = append(ports, nil)
		}
		g := &out[i]
		g.Restarts += u.Restarts
		g.State = worse(g.State, u.State)
		if u.LastError != "" {
			g.LastError = u.LastError
		}
		ports[i] = append(ports[i], u.Ports...)
		g.PIDs = append(g.PIDs, u.PID)
	}
	for i := range out {
		out[i].Ports = PortRange(ports[i])
	}
	return out
}

// PortRange prints 9830,9831,9832 as 9830-9832.
func PortRange(ports []int) string {
	if len(ports) == 0 {
		return "-"
	}
	ports = append([]int(nil), ports...)
	sort.Ints(ports)
	contiguous := true
	for i := 1; i < len(ports); i++ {
		if ports[i] != ports[i-1]+1 {
			contiguous = false
		}
	}
	if contiguous && len(ports) > 1 {
		return fmt.Sprintf("%d-%d", ports[0], ports[len(ports)-1])
	}
	s := make([]string, len(ports))
	for i, p := range ports {
		s[i] = fmt.Sprint(p)
	}
	return strings.Join(s, ",")
}

// worse picks the state to show for a group: any failure wins, then any transition.
func worse(a, b supervisor.State) supervisor.State {
	rank := map[supervisor.State]int{
		supervisor.Running: 0, supervisor.Stopped: 1, supervisor.Starting: 2,
		supervisor.Stopping: 3, supervisor.Backoff: 4, supervisor.Failed: 5,
	}
	if rank[b] > rank[a] {
		return b
	}
	return a
}
