// Package supervisor runs long-lived processes: start, wait until ready,
// respawn with backoff after a crash, and stop gracefully with a hard kill as
// the fallback. On Windows each unit lives in its own Job Object, so killing a
// unit kills its whole tree, and every unit dies if the supervisor dies.
package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/harsha2805/devstack/internal/winproc"
)

// Spec describes one process. All funcs are optional.
type Spec struct {
	Name    string // unique unit name, e.g. "php-8.3#0"
	Group   string // display and command group, e.g. "php-8.3"
	Version string
	Ports   []int

	Path string
	Args []string
	Dir  string
	Env  []string // full environment; nil = inherit
	Log  io.Writer

	// Before runs before each start (config test, first-run init).
	Before func(ctx context.Context) error
	// Ready is polled until it returns nil or ReadyTimeout passes.
	Ready        func(ctx context.Context) error
	ReadyTimeout time.Duration
	// After runs once after each start becomes ready.
	After func(ctx context.Context) error
	// Stop asks the process to exit. The job is killed after StopTimeout either way.
	Stop        func(ctx context.Context) error
	StopTimeout time.Duration
	// Reload applies config changes without a restart (nginx -s reload).
	Reload func(ctx context.Context) error
	// Expected reports whether an exit is routine (php-cgi recycling after
	// max_requests). Expected exits respawn at once and don't count as crashes.
	Expected func(exitCode int) bool

	// CmdKey and ConfKey let a reload decide between restart, reload, or nothing.
	CmdKey  string
	ConfKey string
}

type State string

const (
	Stopped  State = "stopped"
	Starting State = "starting"
	Running  State = "running"
	Backoff  State = "backoff"
	Stopping State = "stopping"
	Failed   State = "failed"
)

// Status is a snapshot of one unit.
type Status struct {
	Name      string    `json:"name"`
	Group     string    `json:"group"`
	Version   string    `json:"version"`
	Ports     []int     `json:"ports"`
	State     State     `json:"state"`
	PID       int       `json:"pid,omitempty"`
	Since     time.Time `json:"since"`
	Restarts  int       `json:"restarts"`
	LastError string    `json:"last_error,omitempty"`
}

// Crash policy: give up after MaxCrashes within CrashWindow.
var (
	MaxCrashes  = 5
	CrashWindow = time.Minute
	MaxBackoff  = 30 * time.Second
)

type Unit struct {
	spec Spec
	logf func(format string, args ...any)

	mu       sync.Mutex
	state    State
	pid      int
	since    time.Time
	restarts int
	lastErr  string
	crashes  []time.Time
	confKey  string

	stopCh chan struct{} // closed to ask the run loop to stop
	doneCh chan struct{} // closed when the run loop has exited
	exitCh chan error    // current process exit, while running
	job    *winproc.Job
}

// Spec returns the unit's spec, with ConfKey as last set by SetConfKey.
func (u *Unit) Spec() Spec {
	u.mu.Lock()
	defer u.mu.Unlock()
	sp := u.spec
	sp.ConfKey = u.confKey
	return sp
}

// SetConfKey records that the running process has picked up new config
// (after a successful Reload), without restarting it.
func (u *Unit) SetConfKey(k string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.confKey = k
}

func (u *Unit) Status() Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	return Status{
		Name: u.spec.Name, Group: u.spec.Group, Version: u.spec.Version, Ports: u.spec.Ports,
		State: u.state, PID: u.pid, Since: u.since, Restarts: u.restarts, LastError: u.lastErr,
	}
}

func (u *Unit) set(state State, pid int, errMsg string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.state != state {
		u.since = time.Now()
	}
	u.state, u.pid = state, pid
	if errMsg != "" {
		u.lastErr = errMsg
	}
}

// Supervisor owns a set of units.
type Supervisor struct {
	log   *log.Logger
	mu    sync.Mutex
	units []*Unit
}

func New(logger *log.Logger) *Supervisor {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Supervisor{log: logger}
}

// Units returns the current units in start order.
func (s *Supervisor) Units() []*Unit {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*Unit(nil), s.units...)
}

func (s *Supervisor) Statuses() []Status {
	var out []Status
	for _, u := range s.Units() {
		out = append(out, u.Status())
	}
	return out
}

// Find returns the units whose name or group equals name ("" = all).
func (s *Supervisor) Find(name string) []*Unit {
	var out []*Unit
	for _, u := range s.Units() {
		if name == "" || u.spec.Name == name || u.spec.Group == name {
			out = append(out, u)
		}
	}
	return out
}

// Add registers a unit and starts it, returning once it is ready or has
// failed its first start. A first-start failure is not retried: it is
// almost always a config problem, and retrying only hides the message.
func (s *Supervisor) Add(ctx context.Context, spec Spec) (*Unit, error) {
	u := &Unit{spec: spec, state: Stopped, since: time.Now(), confKey: spec.ConfKey}
	u.logf = func(format string, args ...any) { s.log.Printf(spec.Name+": "+format, args...) }
	s.mu.Lock()
	s.units = append(s.units, u)
	s.mu.Unlock()
	return u, u.start(ctx)
}

// Remove stops a unit and forgets it.
func (s *Supervisor) Remove(ctx context.Context, u *Unit) {
	u.stop(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, x := range s.units {
		if x == u {
			s.units = append(s.units[:i], s.units[i+1:]...)
			break
		}
	}
}

// Replace stops u and starts a unit from spec in the same position.
func (s *Supervisor) Replace(ctx context.Context, u *Unit, spec Spec) (*Unit, error) {
	u.stop(ctx)
	n := &Unit{spec: spec, state: Stopped, since: time.Now(), restarts: u.Status().Restarts, confKey: spec.ConfKey}
	n.logf = func(format string, args ...any) { s.log.Printf(spec.Name+": "+format, args...) }
	s.mu.Lock()
	for i, x := range s.units {
		if x == u {
			s.units[i] = n
		}
	}
	s.mu.Unlock()
	return n, n.start(ctx)
}

// Restart stops and starts the given units, waiting for each to be ready.
func (s *Supervisor) Restart(ctx context.Context, units []*Unit) error {
	var errs []error
	for _, u := range units {
		u.stop(ctx)
		if err := u.start(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// StopAll stops units in reverse start order.
func (s *Supervisor) StopAll(ctx context.Context) {
	units := s.Units()
	for i := len(units) - 1; i >= 0; i-- {
		units[i].stop(ctx)
	}
}

// start launches the run loop and waits for the first start to settle.
func (u *Unit) start(ctx context.Context) error {
	u.mu.Lock()
	u.stopCh = make(chan struct{})
	u.doneCh = make(chan struct{})
	u.crashes = nil
	u.lastErr = ""
	u.mu.Unlock()

	first := make(chan error, 1)
	go u.run(first)
	select {
	case err := <-first:
		if err != nil {
			return fmt.Errorf("%s: %w", u.spec.Name, err)
		}
		return nil
	case <-ctx.Done():
		u.stop(context.Background())
		return ctx.Err()
	}
}

func (u *Unit) run(first chan<- error) {
	defer close(u.doneCh)
	backoff := time.Second
	reported := false
	report := func(err error) {
		if !reported {
			reported = true
			first <- err
		}
	}
	for {
		started := time.Now()
		err := u.launch()
		if err != nil {
			if !reported || u.stopping() {
				u.set(Failed, 0, err.Error())
				u.logf("start failed: %v", err)
				report(err)
				return
			}
			u.logf("restart failed: %v", err)
		} else {
			u.set(Running, u.pidNow(), "")
			report(nil)
			select {
			case <-u.stopCh:
				return // stop() handles the process
			case exitErr := <-u.exitCh:
				code := exitCode(exitErr)
				u.cleanupJob()
				if u.spec.Expected != nil && u.spec.Expected(code) {
					u.logf("exited (code %d), respawning", code)
					u.mu.Lock()
					u.restarts++
					u.mu.Unlock()
					continue
				}
				err = fmt.Errorf("exited unexpectedly (code %d) after %s", code, time.Since(started).Round(time.Second))
				u.logf("%v", err)
			}
		}

		// Crash: count it, back off, try again.
		now := time.Now()
		u.mu.Lock()
		u.restarts++
		u.lastErr = err.Error()
		var recent []time.Time
		for _, t := range u.crashes {
			if now.Sub(t) < CrashWindow {
				recent = append(recent, t)
			}
		}
		u.crashes = append(recent, now)
		tooMany := len(u.crashes) > MaxCrashes
		u.mu.Unlock()
		if tooMany {
			u.set(Failed, 0, fmt.Sprintf("gave up after %d crashes in %s; last: %v", MaxCrashes, CrashWindow, err))
			u.logf("giving up after %d crashes", MaxCrashes)
			return
		}
		if time.Since(started) > CrashWindow {
			backoff = time.Second
		}
		u.set(Backoff, 0, "")
		select {
		case <-u.stopCh:
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, MaxBackoff)
	}
}

// launch runs Before, spawns the process, waits for Ready, then runs After.
func (u *Unit) launch() error {
	sp := u.spec
	u.set(Starting, 0, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-u.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	if sp.Before != nil {
		if err := sp.Before(ctx); err != nil {
			return err
		}
	}

	cmd := exec.Command(sp.Path, sp.Args...)
	cmd.Dir = sp.Dir
	cmd.Env = sp.Env
	cmd.Stdout = sp.Log
	cmd.Stderr = sp.Log
	winproc.HideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	if job, err := winproc.NewJob(); err == nil {
		if err := job.Assign(cmd.Process.Pid); err != nil {
			u.logf("warning: %v", err)
		}
		u.mu.Lock()
		u.job = job
		u.mu.Unlock()
	}
	exitCh := make(chan error, 1)
	go func() { exitCh <- cmd.Wait() }()
	u.mu.Lock()
	u.pid = cmd.Process.Pid
	u.exitCh = exitCh
	u.mu.Unlock()
	u.logf("started pid %d", cmd.Process.Pid)

	if sp.Ready != nil {
		timeout := sp.ReadyTimeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		deadline := time.Now().Add(timeout)
		var last error
		for {
			pctx, pcancel := context.WithTimeout(ctx, 3*time.Second)
			last = sp.Ready(pctx)
			pcancel()
			if last == nil {
				break
			}
			select {
			case err := <-exitCh:
				u.cleanupJob()
				return fmt.Errorf("exited during startup (code %d); see its log", exitCode(err))
			case <-ctx.Done():
				u.kill(exitCh)
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
			if time.Now().After(deadline) {
				u.kill(exitCh)
				return fmt.Errorf("not ready after %s: %v", timeout, last)
			}
		}
	}
	if sp.After != nil {
		if err := sp.After(ctx); err != nil {
			u.logf("after-start step failed: %v", err)
			u.mu.Lock()
			u.lastErr = err.Error()
			u.mu.Unlock()
		}
	}
	return nil
}

func (u *Unit) pidNow() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.pid
}

func (u *Unit) stopping() bool {
	select {
	case <-u.stopCh:
		return true
	default:
		return false
	}
}

// stop asks the run loop to end, stops the process gracefully, and kills it
// after StopTimeout. Safe to call on a unit that is not running.
func (u *Unit) stop(ctx context.Context) {
	u.mu.Lock()
	stopCh, doneCh := u.stopCh, u.doneCh
	u.mu.Unlock()
	if stopCh == nil {
		return
	}
	select {
	case <-stopCh:
	default:
		close(stopCh)
	}
	<-doneCh

	// The run loop has exited. Only a Running unit still owns a live process;
	// every other path has already reaped it.
	u.mu.Lock()
	exitCh, state := u.exitCh, u.state
	u.mu.Unlock()

	if state == Running {
		u.set(Stopping, u.pidNow(), "")
		timeout := u.spec.StopTimeout
		if timeout == 0 {
			timeout = 10 * time.Second
		}
		if u.spec.Stop == nil {
			// Nothing to ask nicely; the process is stateless.
			u.kill(exitCh)
		} else {
			sctx, cancel := context.WithTimeout(ctx, timeout)
			if err := u.spec.Stop(sctx); err != nil {
				u.logf("graceful stop failed: %v", err)
			}
			cancel()
		}
		if exitCh != nil && u.spec.Stop != nil {
			select {
			case <-exitCh:
			case <-time.After(timeout):
				u.logf("still running after %s, killing", timeout)
				u.kill(exitCh)
			case <-ctx.Done():
				u.kill(exitCh)
			}
		}
	}
	u.cleanupJob()
	u.set(Stopped, 0, "")
	u.logf("stopped")
}

// kill terminates the whole process tree and waits for the main process.
func (u *Unit) kill(exitCh <-chan error) {
	u.mu.Lock()
	job, pid := u.job, u.pid
	u.mu.Unlock()
	if job == nil || job.Terminate() != nil {
		if p, err := os.FindProcess(pid); err == nil && pid > 0 {
			p.Kill()
		}
	}
	if exitCh != nil {
		select {
		case <-exitCh:
		case <-time.After(5 * time.Second):
		}
	}
	u.cleanupJob()
}

// cleanupJob kills anything left in the job (e.g. orphaned workers) and closes it.
func (u *Unit) cleanupJob() {
	u.mu.Lock()
	job := u.job
	u.job = nil
	u.mu.Unlock()
	if job != nil {
		job.Terminate()
		job.Close()
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}
