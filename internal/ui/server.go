// Package ui serves the desktop app's dashboard: embedded HTML/CSS/JS plus a
// small JSON API on 127.0.0.1. The tray and window in cmd/devstack-app are a
// thin shell around it; it also works in an ordinary browser.
package ui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/harsha2805/devstack/internal/aiguide"
	"github.com/harsha2805/devstack/internal/api"
	"github.com/harsha2805/devstack/internal/appconf"
	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/control"
	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/logfile"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/packages"
	"github.com/harsha2805/devstack/internal/services"
	"github.com/harsha2805/devstack/internal/setup"
	"github.com/harsha2805/devstack/internal/stack"
	"github.com/harsha2805/devstack/internal/supervisor"
	"github.com/harsha2805/devstack/internal/winproc"
)

//go:embed web
var webFS embed.FS

// AutostartName is the HKCU Run value the app uses.
const AutostartName = "devstack"

type Server struct {
	Version string
	Exe     string // this executable; the autostart target
	Daemon  string // runs the background service (`<Daemon> --root X daemon`); Exe when empty
	OnShow  func() // bring the window forward (second launch)
	OnQuit  func()

	token string
	addr  string

	mu       sync.Mutex
	job      *Job
	settings appconf.Settings
	upd      updateState
}

// Job is a long action (setup, start, stop, install) the UI shows progress for.
type Job struct {
	Title    string  `json:"title"`
	Steps    []*Step `json:"steps"`
	Finished bool    `json:"finished"`
	Error    string  `json:"error,omitempty"`
}

type Step struct {
	Label string `json:"label"`
	State string `json:"state"` // pending, running, done, failed
	Done  int64  `json:"done,omitempty"`
	Total int64  `json:"total,omitempty"`
}

// Listen binds 127.0.0.1 on a random port and starts serving.
func (s *Server) Listen() (string, error) {
	st, err := appconf.Load()
	if err != nil {
		return "", err
	}
	s.settings = st
	b := make([]byte, 24)
	rand.Read(b)
	s.token = hex.EncodeToString(b)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	s.addr = ln.Addr().String()
	srv := &http.Server{Handler: s.routes(), ReadHeaderTimeout: 5 * time.Second}
	go srv.Serve(ln)
	return s.addr, nil
}

// URL opens the dashboard and authenticates the viewer.
func (s *Server) URL() string { return "http://" + s.addr + "/?t=" + s.token }

// Token lets a second app instance call /api/show.
func (s *Server) Token() string { return s.token }

func (s *Server) Settings() appconf.Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(webFS, "web")
	files := http.FileServerFS(static)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if t := r.URL.Query().Get("t"); t != "" {
			if !s.tokenOK(t) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "ds", Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		if !s.authed(r) {
			http.Error(w, "Open devstack from its tray icon.", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	})

	api := http.NewServeMux()
	api.HandleFunc("GET /api/state", s.handleState)
	api.HandleFunc("GET /api/logs", s.handleLogs)
	api.HandleFunc("GET /api/packages", s.handlePackages)
	api.HandleFunc("GET /api/db", s.handleDB)
	api.HandleFunc("POST /api/setup", s.handleSetup)
	api.HandleFunc("POST /api/start", func(w http.ResponseWriter, r *http.Request) { s.startJob(w, "Starting servers", s.runStart) })
	api.HandleFunc("POST /api/stop", func(w http.ResponseWriter, r *http.Request) { s.startJob(w, "Stopping servers", s.runStop) })
	api.HandleFunc("POST /api/reload", s.handleReload)
	api.HandleFunc("POST /api/restart", s.handleRestart)
	api.HandleFunc("POST /api/site/php", s.handleSitePHP)
	api.HandleFunc("POST /api/site/guide", s.handleSiteGuide)
	api.HandleFunc("POST /api/project/pick", s.handlePickFolder)
	api.HandleFunc("POST /api/project/add", s.handleProjectAdd)
	api.HandleFunc("POST /api/project/remove", s.handleProjectRemove)
	api.HandleFunc("POST /api/dns", s.handleDNS)
	api.HandleFunc("POST /api/https", s.handleHTTPS)
	api.HandleFunc("POST /api/dns/dismiss", s.handleDNSDismiss)
	api.HandleFunc("POST /api/packages/install", s.handleInstall)
	api.HandleFunc("POST /api/packages/remove", s.handleRemove)
	api.HandleFunc("POST /api/ext", s.handleExt)
	api.HandleFunc("POST /api/open", s.handleOpen)
	api.HandleFunc("POST /api/terminal", s.handleTerminal)
	api.HandleFunc("POST /api/shims/fix", s.handleShimsFix)
	api.HandleFunc("POST /api/settings", s.handleSettings)
	api.HandleFunc("POST /api/update/check", s.handleUpdateCheck)
	api.HandleFunc("POST /api/update/install", s.handleUpdateInstall)
	api.HandleFunc("POST /api/job/dismiss", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		if s.job != nil && s.job.Finished {
			s.job = nil
		}
		s.mu.Unlock()
		reply(w, nil)
	})
	api.HandleFunc("POST /api/show", func(w http.ResponseWriter, r *http.Request) {
		if s.OnShow != nil {
			go s.OnShow()
		}
		reply(w, nil)
	})
	api.HandleFunc("POST /api/quit", func(w http.ResponseWriter, r *http.Request) {
		reply(w, nil)
		if s.OnQuit != nil {
			go s.OnQuit()
		}
	})
	mux.Handle("/api/", s.guard(api))
	return mux
}

// guard admits only this app's own pages: the right Host (no DNS rebinding),
// the session cookie or token header, and a custom header on writes (no
// cross-site form posts).
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != s.addr {
			http.Error(w, "bad host", http.StatusForbidden)
			return
		}
		if !s.authed(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet && r.Header.Get("X-Devstack") != "1" {
			http.Error(w, "missing X-Devstack header", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authed(r *http.Request) bool {
	if t := r.Header.Get("X-Token"); t != "" {
		return s.tokenOK(t)
	}
	c, err := r.Cookie("ds")
	return err == nil && s.tokenOK(c.Value)
}

func (s *Server) tokenOK(t string) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}

// root returns the managed stack root, or "" before setup.
func (s *Server) root() string {
	s.mu.Lock()
	r := s.settings.Root
	s.mu.Unlock()
	if r == "" {
		return ""
	}
	if _, err := os.Stat(filepath.Join(r, config.FileName)); err != nil {
		return ""
	}
	return r
}

// ---- state -------------------------------------------------------------

type siteView struct {
	Name    string   `json:"name"`
	Slug    string   `json:"slug"`
	URL     string   `json:"url"`
	PHP     string   `json:"php"`
	Pinned  bool     `json:"pinned"` // php set in .devstack.toml
	Guide   bool     `json:"guide"`  // has an AI guide (package aiguide)
	Dir     string   `json:"dir"`
	Docroot string   `json:"docroot"`
	Aliases []string `json:"aliases,omitempty"`
	Project bool     `json:"project"` // listed in [projects], outside www
}

type stateView struct {
	Version    string                    `json:"version"`
	NeedsSetup bool                      `json:"needs_setup"`
	Setup      *setupView                `json:"setup,omitempty"`
	Root       string                    `json:"root,omitempty"`
	WWW        string                    `json:"www,omitempty"`
	RootURL    string                    `json:"root_url,omitempty"`
	Running    bool                      `json:"running"`
	Phase      string                    `json:"phase"`
	Groups     []control.Group           `json:"groups"`
	Planned    []plannedService          `json:"planned"`
	Sites      []siteView                `json:"sites"`
	Unserved   []projectView             `json:"unserved"`
	PHPs       []string                  `json:"phps"`
	DefaultPHP string                    `json:"default_php"`
	Errors     []string                  `json:"errors"`
	Warnings   []string                  `json:"warnings"`
	Job        *Job                      `json:"job"`
	Settings   map[string]bool           `json:"settings"`
	Links      map[string]string         `json:"links"`
	Update     *updateView               `json:"update"`
	DNS        *dnsView                  `json:"dns,omitempty"`
	HTTPS      *control.HTTPSStatus      `json:"https,omitempty"`
	Extensions []control.ExtensionStatus `json:"extensions"`
	ODBC       *control.ODBCStatus       `json:"odbc,omitempty"`
	Shims      *shimsView                `json:"shims,omitempty"`
}

// plannedService describes what will run, for display while stopped.
type plannedService struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Ports   string `json:"ports"`
}

type setupView struct {
	Root      string       `json:"root"`
	PHP       []optionView `json:"php"`
	MySQL     []optionView `json:"mysql"`
	Nginx     optionView   `json:"nginx"`
	VCRuntime bool         `json:"vc_runtime"`
}

type optionView struct {
	Value   string `json:"value"` // constraint to send back, e.g. "8.4"
	Version string `json:"version"`
	Label   string `json:"label,omitempty"`
	Size    int64  `json:"size"`
	Default bool   `json:"default,omitempty"`
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	v := stateView{Version: s.Version, Phase: "stopped", Groups: []control.Group{}, Sites: []siteView{}, Unserved: []projectView{},
		Errors: []string{}, Warnings: []string{}, Planned: []plannedService{}, Extensions: []control.ExtensionStatus{},
		Links: map[string]string{"vc_redist": setup.VCRedistURL, "odbc": packages.ODBCDriverURL}}
	s.mu.Lock()
	if s.job != nil {
		j := *s.job
		v.Job = &j
	}
	set := s.settings
	v.Update = s.updateViewLocked()
	s.mu.Unlock()
	v.Settings = map[string]bool{"start_servers": set.StartServers, "check_updates": set.CheckUpdates,
		"autostart": winproc.Autostart(AutostartName)}

	root := s.root()
	if root == "" {
		v.NeedsSetup = true
		v.Setup = s.setupOptions()
		reply(w, v)
		return
	}
	v.Root = root
	if ds, err := control.CheckDNS(r.Context(), root, false); err == nil {
		v.DNS = &dnsView{DNSStatus: ds, Asked: set.DNSAsked}
	}
	if hs, err := control.CheckHTTPS(root); err == nil {
		v.HTTPS = &hs
	}
	if xs, odbc, err := control.CheckExtensions(r.Context(), root); err == nil {
		v.Extensions, v.ODBC = xs, &odbc
	}
	if ss, ok := control.ShimStatus(); ok {
		v.Shims = &shimsView{PathStatus: ss, OK: ss.OK(), Problem: ss.Problem()}
	}
	if l, err := stack.Load(r.Context(), root); err != nil {
		v.Errors = append(v.Errors, err.Error())
	} else if st := l.Stack; st != nil {
		s.fillStack(&v, st)
	}
	if c, err := api.Connect(root); err == nil {
		if ds, err := c.Status(r.Context()); err == nil {
			v.Running = true
			v.Phase = ds.Phase
			v.Groups = control.Groups(ds.Units)
			v.Errors = append(v.Errors, ds.Errors...)
		}
	}
	for _, g := range v.Groups {
		if g.State != supervisor.Running && g.LastError != "" {
			v.Errors = append(v.Errors, g.Name+": "+g.LastError)
		}
	}
	reply(w, v)
}

func (s *Server) fillStack(v *stateView, st *model.Stack) {
	v.WWW = st.WWW
	v.RootURL = st.RootURL()
	// Missing extensions devstack can download show on the extensions card
	// with an Install button instead.
	cat, _ := packages.Default()
	handled := map[string]bool{}
	bin := strings.ToLower(filepath.Join(st.Root, "bin")) + string(filepath.Separator)
	for _, p := range st.PHP {
		if len(p.Missing) > 0 && strings.HasPrefix(strings.ToLower(p.Build.Dir), bin) &&
			!slices.ContainsFunc(p.Missing, func(e string) bool { return !slices.Contains(cat.ExtensionNames(), e) }) {
			handled[model.MissingExtensionsWarning(p)] = true
		}
	}
	for _, w := range st.Warnings {
		if !handled[w] {
			v.Warnings = append(v.Warnings, w)
		}
	}
	if st.DefaultPHP != nil {
		v.DefaultPHP = st.DefaultPHP.Key
	}
	v.Planned = append(v.Planned, plannedService{"nginx", st.Nginx.Version.String(), fmt.Sprint(st.HTTPPort)})
	for _, p := range st.PHP {
		v.Planned = append(v.Planned, plannedService{p.Name, p.Build.Version.String(), control.PortRange(p.Ports)})
	}
	if st.MySQL != nil {
		v.Planned = append(v.Planned, plannedService{"mysql", st.MySQL.Build.Version.String(), fmt.Sprint(st.MySQL.Port)})
	}
	seen := map[string]bool{}
	for _, b := range s.installedPHP(st) {
		if !seen[b] {
			seen[b] = true
			v.PHPs = append(v.PHPs, b)
		}
	}
	for _, site := range st.Sites {
		sv := siteView{Name: site.Name, Slug: site.Slug, URL: st.URL(site), Dir: site.Dir, Docroot: site.Docroot,
			Aliases: site.Aliases, Pinned: site.Site.PHP != "", Guide: aiguide.Installed(site.Dir), Project: site.Project}
		if site.PHP != nil {
			sv.PHP = site.PHP.Key
		}
		v.Sites = append(v.Sites, sv)
	}
	v.Unserved = unserved(st.Projects, v.Sites)
}

// installedPHP lists installed PHP minors, newest first.
func (s *Server) installedPHP(st *model.Stack) []string {
	cfg, err := config.Load(st.Root)
	if err != nil {
		return nil
	}
	found := discover.Scan(context.Background(), stack.BinRoots(cfg))
	var out []string
	for _, b := range found.Builds[discover.PHP] {
		out = append(out, b.Version.MajorMinor())
	}
	return out
}

func (s *Server) setupOptions() *setupView {
	cat, err := packages.Default()
	if err != nil {
		return nil
	}
	v := &setupView{Root: setup.DefaultRoot()}
	_, v.VCRuntime = winproc.VCRuntime()
	for _, e := range cat.Entries {
		ver, _ := discover.ParseVersion(e.Version)
		o := optionView{Value: ver.MajorMinor(), Version: e.Version, Label: e.Label, Size: e.Size}
		switch e.Kind {
		case discover.PHP:
			o.Default = o.Value == setup.DefaultPHP
			v.PHP = append(v.PHP, o)
		case discover.MySQL:
			o.Default = o.Value == setup.DefaultMySQL
			v.MySQL = append(v.MySQL, o)
		case discover.Nginx:
			if e.Label == "stable" {
				v.Nginx = o
			}
		}
	}
	return v
}

// ---- jobs ----------------------------------------------------------------

// startJob runs fn in the background unless another job is running.
func (s *Server) startJob(w http.ResponseWriter, title string, fn func(ctx context.Context, j *Job) error) {
	s.mu.Lock()
	if s.job != nil && !s.job.Finished {
		s.mu.Unlock()
		replyErr(w, http.StatusConflict, fmt.Errorf("%s is still in progress", strings.ToLower(s.job.Title)))
		return
	}
	j := &Job{Title: title}
	s.job = j
	s.mu.Unlock()
	go func() {
		err := fn(context.Background(), j)
		s.mu.Lock()
		j.Finished = true
		if err != nil {
			j.Error = err.Error()
			for _, st := range j.Steps {
				if st.State == "running" {
					st.State = "failed"
				}
			}
		} else if s.job == j && j.Title != "Setting up devstack" {
			s.job = nil // routine jobs vanish when done; the dashboard shows the result
		}
		s.mu.Unlock()
	}()
	reply(w, nil)
}

// step adds a step to j, or updates one, under the lock.
func (s *Server) step(j *Job, label, state string, done, total int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range j.Steps {
		if st.Label == label {
			st.State, st.Done, st.Total = state, done, total
			return
		}
	}
	j.Steps = append(j.Steps, &Step{Label: label, State: state, Done: done, Total: total})
}

func (s *Server) runStart(ctx context.Context, j *Job) error {
	root := s.root()
	if root == "" {
		return errors.New("devstack is not set up yet")
	}
	s.step(j, "Checking configuration", "running", 0, 0)
	if c, err := api.Connect(root); err == nil {
		_, err := c.Reload(ctx)
		s.step(j, "Checking configuration", "done", 0, 0)
		return err
	}
	if _, _, err := control.Prepare(ctx, root); err != nil {
		return err
	}
	s.step(j, "Checking configuration", "done", 0, 0)
	s.step(j, "Starting servers", "running", 0, 0)
	daemon := s.Daemon
	if daemon == "" {
		daemon = s.Exe
	}
	c, err := control.Spawn(ctx, root, daemon)
	if err != nil {
		return err
	}
	err = control.Follow(ctx, root, c, func(g control.Group) {
		state := "done"
		if g.State == supervisor.Failed {
			state = "failed"
		}
		s.step(j, unitTitle(g.Name)+" "+g.Version+" started", state, 0, 0)
	})
	if err == nil {
		s.step(j, "Starting servers", "done", 0, 0)
	}
	return err
}

func (s *Server) runStop(ctx context.Context, j *Job) error {
	root := s.root()
	if root == "" {
		return nil
	}
	s.step(j, "Stopping servers", "running", 0, 0)
	if err := control.Down(ctx, root, nil); err != nil {
		return err
	}
	s.step(j, "Stopping servers", "done", 0, 0)
	return nil
}

// StopServers stops the stack; the app calls it on Quit.
func (s *Server) StopServers(ctx context.Context) error {
	if root := s.root(); root != "" {
		return control.Down(ctx, root, nil)
	}
	return nil
}

// StartServers starts the stack in the background, as if Start was clicked.
func (s *Server) StartServers() {
	if s.root() == "" {
		return
	}
	s.startJob(discardWriter{}, "Starting servers", s.runStart)
}

// StopServersAsync stops the stack in the background, as if Stop was clicked.
func (s *Server) StopServersAsync() {
	s.startJob(discardWriter{}, "Stopping servers", s.runStop)
}

// Root is the managed stack root, or "" before setup.
func (s *Server) Root() string { return s.root() }

// Busy reports the title of a job in progress, or "".
func (s *Server) Busy() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job != nil && !s.job.Finished {
		return s.job.Title
	}
	return ""
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Root  string `json:"root"`
		PHP   string `json:"php"`
		MySQL string `json:"mysql"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	root := winproc.LongPath(filepath.Clean(strings.TrimSpace(req.Root)))
	if err := setup.CheckRoot(root); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	s.startJob(w, "Setting up devstack", func(ctx context.Context, j *Job) error {
		o := setup.Options{Root: root, Download: true, PHP: req.PHP, MySQL: req.MySQL}
		// An existing stack in this folder is adopted, not overwritten.
		if _, err := os.Stat(filepath.Join(root, config.FileName)); err != nil {
			todo, err := setup.Plan(ctx, o)
			if err != nil {
				return err
			}
			for _, e := range todo {
				s.step(j, "Download "+e.Title(), "pending", 0, e.Size)
			}
			s.step(j, "Create "+root, "pending", 0, 0)
			_, err = setup.Init(ctx, o, setup.Events{
				Download: func(e packages.Entry, done, total int64) {
					s.step(j, "Download "+e.Title(), "running", done, total)
				},
				Installed: func(e packages.Entry, dir string) {
					s.step(j, "Download "+e.Title(), "done", e.Size, e.Size)
				},
			})
			if err != nil {
				return err
			}
			s.step(j, "Create "+root, "done", 0, 0)
		}
		s.mu.Lock()
		s.settings.Root = root
		set := s.settings
		s.mu.Unlock()
		if err := appconf.Save(set); err != nil {
			return err
		}
		return s.runStart(ctx, j)
	})
}

// ---- actions ---------------------------------------------------------------

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	c, err := api.Connect(s.root())
	if err != nil {
		replyErr(w, http.StatusConflict, errors.New("servers are not running"))
		return
	}
	res, err := c.Reload(r.Context())
	if err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	reply(w, res)
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Unit string `json:"unit"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	c, err := api.Connect(s.root())
	if err != nil {
		replyErr(w, http.StatusConflict, errors.New("servers are not running"))
		return
	}
	res, err := c.Restart(r.Context(), req.Unit)
	if err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	reply(w, res)
}

// handleSitePHP pins a site to a PHP version in its .devstack.toml ("" = default).
func (s *Server) handleSitePHP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug string `json:"slug"`
		PHP  string `json:"php"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	root := s.root()
	l, err := stack.Load(r.Context(), root)
	if err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	var dir string
	for _, site := range l.Stack.Sites {
		if site.Slug == req.Slug {
			dir = site.Dir
		}
	}
	if dir == "" {
		replyErr(w, http.StatusNotFound, fmt.Errorf("no site %q", req.Slug))
		return
	}
	over, _, err := config.LoadSite(dir)
	if err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	over.PHP = req.PHP
	if err := writeSiteFile(dir, over); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	if c, err := api.Connect(root); err == nil {
		if _, err := c.Reload(r.Context()); err != nil {
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		s.syncGuides(r)
	}
	reply(w, nil)
}

func writeSiteFile(dir string, s config.Site) error {
	path := filepath.Join(dir, config.SiteFileName)
	if s.PHP == "" && s.Docroot == "" && len(s.Aliases) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	var b strings.Builder
	b.WriteString("# devstack settings for this site.\n")
	if err := toml.NewEncoder(&b).Encode(s); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

type packageView struct {
	packages.Entry
	Installed bool `json:"installed"`
	InUse     bool `json:"in_use"`
}

func (s *Server) handlePackages(w http.ResponseWriter, r *http.Request) {
	root := s.root()
	cat, err := packages.Default()
	if err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	installed := map[string]bool{}
	inUse := map[string]bool{}
	if root != "" {
		for _, builds := range discover.Scan(r.Context(), []string{filepath.Join(root, "bin")}).Builds {
			for _, b := range builds {
				installed[string(b.Kind)+" "+b.Version.String()] = true
			}
		}
		if l, err := stack.Load(r.Context(), root); err == nil && l.Stack != nil {
			// Only builds in devstack's own bin/ count; a stack may also use
			// builds from other folders listed in paths.bin.
			st := l.Stack
			bin := strings.ToLower(filepath.Join(root, "bin")) + string(filepath.Separator)
			mark := func(b discover.Build) {
				if strings.HasPrefix(strings.ToLower(b.Dir), bin) {
					inUse[string(b.Kind)+" "+b.Version.String()] = true
				}
			}
			mark(st.Nginx)
			for _, p := range st.PHP {
				mark(p.Build)
			}
			if st.MySQL != nil {
				mark(st.MySQL.Build)
			}
		}
	}
	out := []packageView{}
	for _, e := range cat.Entries {
		k := string(e.Kind) + " " + e.Version
		out = append(out, packageView{Entry: e, Installed: installed[k], InUse: inUse[k]})
	}
	sort.SliceStable(out, func(i, j int) bool { return kindOrder(out[i].Kind) < kindOrder(out[j].Kind) })
	reply(w, out)
}

func kindOrder(k discover.Kind) int {
	switch k {
	case discover.PHP:
		return 0
	case discover.MySQL:
		return 1
	}
	return 2
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind    string `json:"kind"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	cat, _ := packages.Default()
	e, err := cat.Find(discover.Kind(req.Kind), req.Version)
	if err != nil || e.Version != req.Version {
		replyErr(w, http.StatusBadRequest, fmt.Errorf("unknown package %s %s", req.Kind, req.Version))
		return
	}
	root := s.root()
	s.startJob(w, "Installing "+e.String(), func(ctx context.Context, j *Job) error {
		label := "Download " + e.Title()
		s.step(j, label, "running", 0, e.Size)
		_, err := packages.Install(ctx, filepath.Join(root, "bin"), e, func(done, total int64) {
			s.step(j, label, "running", done, total)
		})
		if err != nil && !errors.Is(err, packages.ErrInstalled) {
			return err
		}
		s.step(j, label, "done", e.Size, e.Size)
		if e.Kind != discover.PHP {
			return nil
		}
		// Extensions devstack downloads (sqlsrv) aren't in php.net's zip.
		cfg, err := config.Load(root)
		if err != nil {
			return nil
		}
		return control.FetchExtensions(ctx, root, cfg.PHP.Extensions, s.extStep(j))
	})
}

// extStep shows each extension download as a job step.
func (s *Server) extStep(j *Job) control.ExtProgress {
	return func(x packages.Extension, done, total int64) {
		state := "running"
		if total > 0 && done >= total {
			state = "done"
		}
		s.step(j, "Download "+x.Name+" for PHP "+x.PHP, state, done, total)
	}
}

// handleExt turns a catalog PHP extension (sqlsrv) on, downloading it for
// every PHP version, or off.
func (s *Server) handleExt(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		On   bool   `json:"on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	cat, _ := packages.Default()
	if !slices.Contains(cat.ExtensionNames(), req.Name) {
		replyErr(w, http.StatusBadRequest, fmt.Errorf("unknown extension %q", req.Name))
		return
	}
	root := s.root()
	if !req.On {
		if err := control.DisableExtensions(r.Context(), root, []string{req.Name}); err != nil {
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
		reply(w, nil)
		return
	}
	s.startJob(w, "Installing "+req.Name, func(ctx context.Context, j *Job) error {
		return control.EnableExtensions(ctx, root, []string{req.Name}, s.extStep(j))
	})
}

func (s *Server) handleRemove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind    string `json:"kind"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	root := s.root()
	if l, err := stack.Load(r.Context(), root); err == nil && l.Stack != nil {
		st := l.Stack
		used := st.Nginx.Version.String() == req.Version && req.Kind == "nginx" ||
			st.MySQL != nil && req.Kind == "mysql" && st.MySQL.Build.Version.String() == req.Version
		for _, p := range st.PHP {
			used = used || req.Kind == "php" && p.Build.Version.String() == req.Version
		}
		if used {
			replyErr(w, http.StatusConflict, fmt.Errorf("%s %s is in use by this stack; switch away from it first", req.Kind, req.Version))
			return
		}
	}
	if err := packages.Remove(filepath.Join(root, "bin"), discover.Kind(req.Kind), req.Version); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	reply(w, nil)
}

func (s *Server) handleDB(w http.ResponseWriter, r *http.Request) {
	root := s.root()
	l, err := stack.Load(r.Context(), root)
	if err != nil || l.Stack == nil || l.Stack.MySQL == nil {
		reply(w, map[string]any{"enabled": false})
		return
	}
	sec, _ := services.LoadSecrets(l.Stack)
	reply(w, map[string]any{
		"enabled": true, "host": "127.0.0.1", "port": l.Stack.MySQL.Port, "user": "root",
		"password": sec.MySQLRootPassword, "ready": sec.MySQLRootApplied,
	})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	root := s.root()
	dir := filepath.Join(root, "logs")
	var names []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".log") {
			rel, _ := filepath.Rel(dir, p)
			names = append(names, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(names)
	name := r.URL.Query().Get("name")
	text := ""
	if name != "" {
		for _, n := range names { // only files we listed, never arbitrary paths
			if n == name {
				text = logfile.Tail(filepath.Join(dir, filepath.FromSlash(n)), 400)
			}
		}
	}
	reply(w, map[string]any{"files": names, "text": text})
}

// handleOpen opens a site URL, a folder inside the stack or a listed project,
// stack.toml, a PHP version's own php.ini, or a known external link. Anything
// else is refused.
func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind   string `json:"kind"` // url, folder, config, phpini
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	root := s.root()
	var target string
	switch req.Kind {
	case "url":
		if localURL(req.Target) || knownLink(req.Target) {
			target = req.Target
		}
	case "folder":
		p := filepath.Clean(req.Target)
		if root != "" && (inside(p, root) || inProject(root, p)) {
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				target = p
			}
		}
	case "config":
		if root != "" {
			target = filepath.Join(root, config.FileName)
		}
	case "phpini": // target is a PHP version, "8.3"
		if root != "" {
			p, err := control.CustomPHPINI(r.Context(), root, req.Target)
			if err != nil {
				replyErr(w, http.StatusBadRequest, err)
				return
			}
			target = p
		}
	}
	if target == "" {
		replyErr(w, http.StatusForbidden, errors.New("not allowed"))
		return
	}
	if err := winproc.ShellOpen(target); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	reply(w, nil)
}

func inside(p, base string) bool {
	return p == base || strings.HasPrefix(strings.ToLower(p), strings.ToLower(base)+string(filepath.Separator))
}

// inProject reports whether p is a folder listed in root's [projects], or inside one.
func inProject(root, p string) bool {
	cfg, err := config.Load(root)
	if err != nil {
		return false
	}
	p = winproc.LongPath(p) // projects are stored as long paths; C:\USERS~1\... is the same folder
	for _, pr := range stack.Projects(cfg) {
		if inside(p, pr.Dir) {
			return true
		}
	}
	return false
}

func localURL(u string) bool {
	var host string
	switch {
	case strings.HasPrefix(u, "http://"):
		host = strings.TrimPrefix(u, "http://")
	case strings.HasPrefix(u, "https://"):
		host = strings.TrimPrefix(u, "https://")
	default:
		return false
	}
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".test")
}

var knownLinks = map[string]bool{
	setup.VCRedistURL:                        true,
	packages.ODBCDriverURL:                   true,
	"https://www.php.net/license/":           true,
	"https://nginx.org/LICENSE":              true,
	"https://www.mysql.com/about/legal/":     true,
	"https://github.com/harsha2805/devstack": true,
	"https://github.com/sponsors/harsha2805": true,
}

func knownLink(u string) bool { return knownLinks[u] }

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StartServers *bool `json:"start_servers"`
		CheckUpdates *bool `json:"check_updates"`
		Autostart    *bool `json:"autostart"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	if req.Autostart != nil {
		cmd := `"` + s.Exe + `" --tray`
		if err := winproc.SetAutostart(AutostartName, cmd, *req.Autostart); err != nil {
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
	}
	if req.StartServers != nil || req.CheckUpdates != nil {
		s.mu.Lock()
		if req.StartServers != nil {
			s.settings.StartServers = *req.StartServers
		}
		if req.CheckUpdates != nil {
			s.settings.CheckUpdates = *req.CheckUpdates
		}
		set := s.settings
		s.mu.Unlock()
		if err := appconf.Save(set); err != nil {
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
	}
	reply(w, nil)
}

// ---- helpers ----------------------------------------------------------------

// unitTitle turns a service name into its product name: php-8.3 -> PHP.
func unitTitle(name string) string {
	switch {
	case strings.HasPrefix(name, "php-"):
		return "PHP"
	case name == "mysql":
		return "MySQL"
	}
	return name
}

func reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if v == nil {
		v = map[string]bool{"ok": true}
	}
	json.NewEncoder(w).Encode(v)
}

func replyErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// discardWriter lets StartServers reuse startJob without an HTTP request.
type discardWriter struct{}

func (discardWriter) Header() http.Header         { return http.Header{} }
func (discardWriter) Write(b []byte) (int, error) { return len(b), nil }
func (discardWriter) WriteHeader(int)             {}
