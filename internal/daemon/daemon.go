// Package daemon is the long-running process behind `devstack up`. It renders
// config, owns the supervisor, and serves the local API the CLI talks to.
package daemon

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/harsha2805/devstack/internal/aiguide"
	"github.com/harsha2805/devstack/internal/api"
	"github.com/harsha2805/devstack/internal/dns"
	"github.com/harsha2805/devstack/internal/logfile"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/render"
	"github.com/harsha2805/devstack/internal/services"
	"github.com/harsha2805/devstack/internal/sites"
	"github.com/harsha2805/devstack/internal/stack"
	"github.com/harsha2805/devstack/internal/supervisor"
	"github.com/harsha2805/devstack/internal/winproc"
)

type Options struct {
	Root    string
	Version string
	// Console, if set, also receives the daemon log (foreground mode).
	Console io.Writer
}

type Daemon struct {
	opts    Options
	log     *log.Logger
	sup     *supervisor.Supervisor
	logs    *services.Logs
	token   string
	started time.Time

	mu       sync.Mutex
	phase    string
	errs     []string
	warnings []string
	st       *model.Stack
	www      string // watched folder, the listed projects and the fingerprint of their scan in st (see watch.go)
	projects []sites.Project
	scan     string

	dnsMu  sync.Mutex
	dnsSrv *dns.Server // nil unless services.dns is on (see dns.go)

	opMu     sync.Mutex // one reload/restart at a time
	shutdown chan struct{}
	once     sync.Once
}

// Run blocks until ctx is cancelled or a client asks for shutdown, then stops
// every service.
func Run(ctx context.Context, opts Options) error {
	dataDir := filepath.Join(opts.Root, "data", "devstack")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	lock, err := winproc.LockFile(filepath.Join(dataDir, "daemon.lock"))
	if err != nil {
		return fmt.Errorf("devstack is already running for %s", opts.Root)
	}
	defer lock.Close()

	lw, err := logfile.Open(filepath.Join(opts.Root, "logs", "devstack.log"))
	if err != nil {
		return err
	}
	defer lw.Close()
	var out io.Writer = lw
	if opts.Console != nil {
		out = io.MultiWriter(lw, opts.Console)
	}
	logger := log.New(out, "", log.LstdFlags)

	d := &Daemon{
		opts:     opts,
		log:      logger,
		sup:      supervisor.New(logger),
		logs:     services.NewLogs(filepath.Join(opts.Root, "logs")),
		token:    randomHex(32),
		started:  time.Now(),
		phase:    api.PhaseStarting,
		shutdown: make(chan struct{}),
	}
	defer d.logs.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: d.routes(), ReadHeaderTimeout: 5 * time.Second}
	go srv.Serve(ln)
	defer srv.Close()

	state := api.State{PID: os.Getpid(), Addr: ln.Addr().String(), Token: d.token, Started: d.started, Version: opts.Version}
	if err := writeJSON(api.StatePath(opts.Root), state); err != nil {
		return err
	}
	defer os.Remove(api.StatePath(opts.Root))
	logger.Printf("daemon %s pid %d, api %s, root %s", opts.Version, state.PID, state.Addr, opts.Root)

	if err := d.startAll(ctx); err != nil {
		logger.Printf("startup failed: %v", err)
		d.stopAll()
		return err
	}
	go d.watchSites(ctx)

	select {
	case <-ctx.Done():
		logger.Printf("interrupted")
	case <-d.shutdown:
		logger.Printf("shutdown requested")
	}
	d.stopAll()
	logger.Printf("daemon stopped")
	return nil
}

func (d *Daemon) startAll(ctx context.Context) error {
	l, err := stack.Load(ctx, d.opts.Root)
	if err != nil {
		return err
	}
	st := l.Stack
	res, err := render.Write(st)
	if err != nil {
		return err
	}
	d.log.Printf("rendered etc/: %d written, %d removed", len(res.Written), len(res.Removed))
	d.syncGuides(st)
	d.applyDNS(st)
	specs, err := services.Specs(st, d.logs)
	if err != nil {
		return err
	}
	d.logWarnings(nil, st.Warnings)
	d.mu.Lock()
	d.st, d.warnings = st, st.Warnings
	d.www, d.projects, d.scan = l.WWW, l.Projects, fingerprint(l.Sites, st)
	d.mu.Unlock()

	var errs []string
	for _, sp := range specs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := d.sup.Add(ctx, sp); err != nil {
			errs = append(errs, err.Error())
		}
	}
	d.mu.Lock()
	d.phase, d.errs = api.PhaseRunning, errs
	d.mu.Unlock()
	d.log.Printf("startup complete, %d problem(s)", len(errs))
	return nil
}

func (d *Daemon) stopAll() {
	d.mu.Lock()
	d.phase = api.PhaseStopping
	d.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	d.sup.StopAll(ctx)
	d.stopDNS()
}

// reload re-reads everything and changes only what differs: new units start,
// removed ones stop, changed command lines restart, changed config reloads
// in place where the service supports it, and failed units get another try.
func (d *Daemon) reload(ctx context.Context) ([]string, error) {
	d.opMu.Lock()
	defer d.opMu.Unlock()

	l, err := stack.Load(ctx, d.opts.Root)
	if err != nil {
		return nil, err
	}
	st := l.Stack
	res, err := render.Write(st)
	if err != nil {
		return nil, err
	}
	specs, err := services.Specs(st, d.logs)
	if err != nil {
		return nil, err
	}

	d.syncGuides(st)
	d.applyDNS(st)

	var actions, errs []string
	for _, f := range res.Written {
		actions = append(actions, "wrote etc/"+f)
	}
	for _, f := range res.Removed {
		actions = append(actions, "removed etc/"+f)
	}
	existing := map[string]*supervisor.Unit{}
	for _, u := range d.sup.Units() {
		existing[u.Spec().Name] = u
	}
	note := func(err error, what string) {
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			actions = append(actions, what)
		}
	}
	for _, sp := range specs {
		u, ok := existing[sp.Name]
		delete(existing, sp.Name)
		switch {
		case !ok:
			_, err := d.sup.Add(ctx, sp)
			note(err, "started "+sp.Name)
		case u.Status().State == supervisor.Failed || u.Spec().CmdKey != sp.CmdKey:
			_, err := d.sup.Replace(ctx, u, sp)
			note(err, "restarted "+sp.Name)
		case u.Spec().ConfKey != sp.ConfKey && sp.Reload != nil:
			err := sp.Reload(ctx)
			if err == nil {
				u.SetConfKey(sp.ConfKey)
			}
			note(err, "reloaded "+sp.Name)
		case u.Spec().ConfKey != sp.ConfKey:
			_, err := d.sup.Replace(ctx, u, sp)
			note(err, "restarted "+sp.Name)
		}
	}
	for name, u := range existing {
		d.sup.Remove(ctx, u)
		actions = append(actions, "stopped "+name)
	}

	d.mu.Lock()
	before := d.warnings
	d.st, d.warnings, d.errs = st, st.Warnings, errs
	d.www, d.projects, d.scan = l.WWW, l.Projects, fingerprint(l.Sites, st)
	d.mu.Unlock()
	d.logWarnings(before, st.Warnings)
	if len(errs) > 0 {
		return actions, errors.New(strings.Join(errs, "\n"))
	}
	return actions, nil
}

func (d *Daemon) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		s := api.Status{
			PID: os.Getpid(), Started: d.started, Version: d.opts.Version, Root: d.opts.Root,
			Phase: d.phase, Errors: d.errs, Warnings: d.warnings,
		}
		d.mu.Unlock()
		s.Units = d.sup.Statuses()
		reply(w, http.StatusOK, s)
	})
	mux.HandleFunc("POST /v1/reload", func(w http.ResponseWriter, r *http.Request) {
		actions, err := d.reload(r.Context())
		result(w, actions, err)
	})
	mux.HandleFunc("POST /v1/restart", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("unit")
		units := d.sup.Find(name)
		if len(units) == 0 {
			reply(w, http.StatusNotFound, api.Result{Error: fmt.Sprintf("no service named %q", name)})
			return
		}
		d.opMu.Lock()
		err := d.sup.Restart(r.Context(), units)
		d.opMu.Unlock()
		var actions []string
		for _, u := range units {
			actions = append(actions, "restarted "+u.Spec().Name)
		}
		result(w, actions, err)
	})
	mux.HandleFunc("POST /v1/shutdown", func(w http.ResponseWriter, r *http.Request) {
		reply(w, http.StatusAccepted, api.Result{})
		d.once.Do(func() { close(d.shutdown) })
	})
	return d.auth(mux)
}

// auth requires the bearer token from daemon.json, so a web page open in a
// browser cannot drive the API.
func (d *Daemon) auth(next http.Handler) http.Handler {
	want := []byte("Bearer " + d.token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			reply(w, http.StatusUnauthorized, api.Result{Error: "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func result(w http.ResponseWriter, actions []string, err error) {
	if err != nil {
		reply(w, http.StatusInternalServerError, api.Result{Actions: actions, Error: err.Error()})
		return
	}
	reply(w, http.StatusOK, api.Result{Actions: actions})
}

func reply(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// logWarnings writes the warnings in now that weren't in before to
// devstack.log, so a reload doesn't repeat ones already logged.
func (d *Daemon) logWarnings(before, now []string) {
	for _, w := range now {
		if !slices.Contains(before, w) {
			d.log.Printf("warning: %s", w)
		}
	}
}

// syncGuides keeps projects' AI guides in step with the stack (PHP version,
// ports, paths). Failures are logged, never fatal.
func (d *Daemon) syncGuides(st *model.Stack) {
	changed, err := aiguide.Sync(st, aiguide.CLI())
	for _, slug := range changed {
		d.log.Printf("updated AI guide for %s", slug)
	}
	if err != nil {
		d.log.Printf("AI guide: %v", err)
	}
}
