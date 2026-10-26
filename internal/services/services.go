// Package services turns a model.Stack into supervisor specs: the command
// line, readiness probe, graceful stop and first-run steps for each server.
package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/harsha2805/devstack/internal/logfile"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/supervisor"
	"github.com/harsha2805/devstack/internal/winproc"
)

// Logs opens one log file per group and hands out the same writer to every
// unit in the group. Close it when the stack stops.
type Logs struct {
	dir   string
	files map[string]*logfile.Writer
}

func NewLogs(dir string) *Logs { return &Logs{dir: dir, files: map[string]*logfile.Writer{}} }

func (l *Logs) Get(group string) (io.Writer, error) {
	if w, ok := l.files[group]; ok {
		return w, nil
	}
	w, err := logfile.Open(filepath.Join(l.dir, group+".log"))
	if err != nil {
		return nil, err
	}
	l.files[group] = w
	return w, nil
}

func (l *Logs) Close() {
	for _, w := range l.files {
		w.Close()
	}
}

// Specs returns every unit in start order: databases, PHP pools, then the web
// server, so nginx never answers before its upstreams exist.
func Specs(st *model.Stack, logs *Logs) ([]supervisor.Spec, error) {
	var specs []supervisor.Spec
	if st.MySQL != nil {
		sp, err := MySQLSpec(st, logs)
		if err != nil {
			return nil, err
		}
		specs = append(specs, sp)
	}
	for _, p := range st.PHP {
		sps, err := PHPSpecs(st, p, logs)
		if err != nil {
			return nil, err
		}
		specs = append(specs, sps...)
	}
	sp, err := NginxSpec(st, logs)
	if err != nil {
		return nil, err
	}
	return append(specs, sp), nil
}

// NginxSpec runs nginx with a private prefix so it never touches its install folder.
func NginxSpec(st *model.Stack, logs *Logs) (supervisor.Spec, error) {
	w, err := logs.Get("nginx")
	if err != nil {
		return supervisor.Spec{}, err
	}
	conf := filepath.Join(st.Etc, "nginx", "nginx.conf")
	// nginx wants the prefix to end with a separator.
	base := []string{
		"-p", filepath.Join(st.Tmp, "nginx") + string(filepath.Separator),
		"-c", conf,
		"-e", filepath.Join(st.Logs, "nginx-error.log"),
	}
	run := func(ctx context.Context, extra ...string) error {
		cmd := exec.CommandContext(ctx, st.Nginx.Exe, append(append([]string{}, base...), extra...)...)
		cmd.Dir = filepath.Join(st.Tmp, "nginx")
		winproc.HideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("nginx %s: %v\n%s", strings.Join(extra, " "), err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	test := func(ctx context.Context) error { return run(ctx, "-t", "-q") }
	return supervisor.Spec{
		Name: "nginx", Group: "nginx", Version: st.Nginx.Version.String(), Ports: []int{st.HTTPPort},
		Path: st.Nginx.Exe, Args: base, Dir: filepath.Join(st.Tmp, "nginx"), Log: w,
		Before: test,
		Ready:  tcpReady(st.HTTPPort),
		Stop:   func(ctx context.Context) error { return run(ctx, "-s", "quit") },
		Reload: func(ctx context.Context) error {
			if err := test(ctx); err != nil {
				return err
			}
			return run(ctx, "-s", "reload")
		},
		StopTimeout: 10 * time.Second,
		CmdKey:      key(st.Nginx.Exe, base),
		ConfKey:     fileKey(nginxFiles(st)...),
	}, nil
}

func nginxFiles(st *model.Stack) []string {
	files := []string{filepath.Join(st.Etc, "nginx", "nginx.conf"), filepath.Join(st.Etc, "nginx", "fastcgi.conf")}
	for _, s := range st.Sites {
		files = append(files, filepath.Join(st.Etc, "nginx", "sites", s.Slug+".conf"))
	}
	custom, _ := filepath.Glob(filepath.Join(st.EtcCustom, "nginx", "*.conf"))
	return append(files, custom...)
}

// PHPSpecs runs one php-cgi per port. php-cgi exits by itself after
// PHP_FCGI_MAX_REQUESTS requests; that exit is routine and respawns at once.
func PHPSpecs(st *model.Stack, p *model.PHPPool, logs *Logs) ([]supervisor.Spec, error) {
	w, err := logs.Get(p.Name)
	if err != nil {
		return nil, err
	}
	exe := filepath.Join(p.Build.Dir, "php-cgi.exe")
	env := append(os.Environ(),
		"PHP_FCGI_CHILDREN=0",
		"PHP_FCGI_MAX_REQUESTS="+strconv.Itoa(p.MaxRequests),
		"PHPRC="+filepath.Dir(p.INIPath),
	)
	var specs []supervisor.Spec
	for i, port := range p.Ports {
		args := []string{"-b", "127.0.0.1:" + strconv.Itoa(port), "-c", p.INIPath}
		specs = append(specs, supervisor.Spec{
			Name: fmt.Sprintf("%s#%d", p.Name, i), Group: p.Name, Version: p.Build.Version.String(), Ports: []int{port},
			Path: exe, Args: args, Dir: p.Build.Dir, Env: env, Log: w,
			Ready:       tcpReady(port),
			StopTimeout: 2 * time.Second,
			Expected:    func(code int) bool { return code == 0 && p.MaxRequests > 0 },
			CmdKey:      key(exe, args) + strconv.Itoa(p.MaxRequests),
			ConfKey:     fileKey(p.INIPath),
		})
	}
	return specs, nil
}

func tcpReady(port int) func(context.Context) error {
	addr := "127.0.0.1:" + strconv.Itoa(port)
	return func(ctx context.Context) error {
		var d net.Dialer
		c, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return err
		}
		return c.Close()
	}
}

func key(exe string, args []string) string {
	return exe + "\x00" + strings.Join(args, "\x00")
}

// fileKey hashes file contents so a reload can tell whether config changed.
func fileKey(paths ...string) string {
	h := sha256.New()
	for _, p := range paths {
		b, _ := os.ReadFile(p)
		fmt.Fprintf(h, "%s\x00%d\x00", p, len(b))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))
}
