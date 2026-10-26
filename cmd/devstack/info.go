package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/harsha2805/devstack/internal/api"
	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/control"
	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/logfile"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/render"
	"github.com/harsha2805/devstack/internal/services"
	"github.com/harsha2805/devstack/internal/setup"
	"github.com/harsha2805/devstack/internal/stack"
	"github.com/harsha2805/devstack/internal/winproc"
)

func cmdSites(ctx context.Context, e *env, args []string) error {
	if len(args) > 0 {
		return usageError{"unexpected argument " + args[0]}
	}
	root, err := e.root()
	if err != nil {
		return err
	}
	l, err := stack.Load(ctx, root)
	if err != nil {
		return err
	}
	printWarnings(e, l.Stack.Warnings)
	printSites(e, l.Stack)
	return nil
}

func printSites(e *env, st *model.Stack) {
	if len(st.Sites) == 0 {
		fmt.Fprintf(e.stdout, "No sites yet. Create a folder in %s.\n", st.WWW)
		return
	}
	tw := tabwriter.NewWriter(e.stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "SITE\tURL\tPHP\tDOCROOT")
	for _, s := range st.Sites {
		php := "-"
		if s.PHP != nil {
			php = s.PHP.Key
		}
		docroot, err := filepath.Rel(st.Root, s.Docroot)
		if err != nil || strings.HasPrefix(docroot, "..") {
			docroot = s.Docroot
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.Slug, st.URL(s), php, docroot)
	}
	tw.Flush()
	fmt.Fprintf(e.stdout, "\nAll sites: %s\n", st.RootURL())
}

func cmdRender(ctx context.Context, e *env, args []string) error {
	if len(args) > 0 {
		return usageError{"unexpected argument " + args[0]}
	}
	root, err := e.root()
	if err != nil {
		return err
	}
	l, err := stack.Load(ctx, root)
	if err != nil {
		return err
	}
	printWarnings(e, l.Stack.Warnings)
	res, err := render.Write(l.Stack)
	if err != nil {
		return err
	}
	for _, f := range res.Written {
		fmt.Fprintln(e.stdout, "  wrote   etc/"+f)
	}
	for _, f := range res.Removed {
		fmt.Fprintln(e.stdout, "  removed etc/"+f)
	}
	if !res.Changed() {
		fmt.Fprintln(e.stdout, "etc/ is up to date.")
	} else if _, err := api.Connect(root); err == nil {
		fmt.Fprintln(e.stdout, "devstack is running; run `devstack reload` to apply.")
	}
	return nil
}

func cmdDB(ctx context.Context, e *env, args []string) error {
	if len(args) != 1 || args[0] != "creds" {
		return usageError{"the only db subcommand so far is `creds`"}
	}
	root, err := e.root()
	if err != nil {
		return err
	}
	l, err := stack.Load(ctx, root)
	if err != nil {
		return err
	}
	st := l.Stack
	if st.MySQL == nil {
		return fmt.Errorf("mysql is not enabled (services.mysql in stack.toml)")
	}
	sec, err := services.LoadSecrets(st)
	if err != nil {
		return err
	}
	if sec.MySQLRootPassword == "" || !sec.MySQLRootApplied {
		return fmt.Errorf("no root password yet; it is set the first time `devstack up` starts MySQL")
	}
	tw := tabwriter.NewWriter(e.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "host\t127.0.0.1\n")
	fmt.Fprintf(tw, "port\t%d\n", st.MySQL.Port)
	fmt.Fprintf(tw, "user\troot\n")
	fmt.Fprintf(tw, "password\t%s\n", sec.MySQLRootPassword)
	tw.Flush()
	fmt.Fprintf(e.stdout, "\n.env:  DB_HOST=127.0.0.1  DB_PORT=%d  DB_USERNAME=root  DB_PASSWORD=%s\n", st.MySQL.Port, sec.MySQLRootPassword)
	return nil
}

func cmdLogs(ctx context.Context, e *env, args []string) error {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	follow := fs.Bool("f", false, "keep printing new lines")
	n := fs.Int("n", 40, "number of lines")
	list := fs.Bool("list", false, "list log files")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 1 {
		return usageError{"takes at most one log name"}
	}
	root, err := e.root()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "logs")
	if *list {
		return listLogs(e, dir)
	}
	name := "devstack"
	if len(pos) == 1 {
		name = pos[0]
	}
	path, err := findLog(dir, name)
	if err != nil {
		return err
	}
	fmt.Fprintf(e.stderr, "==> %s <==\n", path)
	fmt.Fprint(e.stdout, logfile.Tail(path, *n))
	if !*follow {
		return nil
	}
	return followFile(ctx, e.stdout, path)
}

// findLog maps a short name to a file under logs/.
func findLog(dir, name string) (string, error) {
	special := map[string]string{"nginx": "nginx-error.log", "mysql": "mysql.log", "devstack": "devstack.log"}
	cands := []string{special[name], name + "-error.log", name + ".log", filepath.Join("sites", name+".access.log"), name}
	for _, c := range cands {
		if c == "" {
			continue
		}
		p := filepath.Join(dir, c)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("no log named %q; see `devstack logs --list`", name)
}

func listLogs(e *env, dir string) error {
	var files []string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	tw := tabwriter.NewWriter(e.stdout, 0, 0, 3, ' ', 0)
	for _, f := range files {
		st, err := os.Stat(f)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(dir, f)
		fmt.Fprintf(tw, "%s\t%s\t%s\n", filepath.ToSlash(rel), humanSize(st.Size()), st.ModTime().Format("2006-01-02 15:04"))
	}
	return tw.Flush()
}

func followFile(ctx context.Context, w io.Writer, path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	off := st.Size()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(300 * time.Millisecond):
		}
		st, err := os.Stat(path)
		if err != nil {
			continue // rotating
		}
		if st.Size() < off {
			off = 0
		}
		if st.Size() == off {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		buf := make([]byte, st.Size()-off)
		k, _ := f.ReadAt(buf, off)
		f.Close()
		w.Write(buf[:k])
		off += int64(k)
	}
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

func cmdDoctor(ctx context.Context, e *env, args []string) error {
	if len(args) > 0 {
		return usageError{"unexpected argument " + args[0]}
	}
	fails := 0
	ok := func(f string, a ...any) { fmt.Fprintf(e.stdout, "  ok    "+f+"\n", a...) }
	warn := func(f string, a ...any) { fmt.Fprintf(e.stdout, "  warn  "+f+"\n", a...) }
	fail := func(f string, a ...any) { fails++; fmt.Fprintf(e.stdout, "  FAIL  "+f+"\n", a...) }

	root, err := e.root()
	if err != nil {
		fail("%v", err)
		return fmt.Errorf("%d problem(s)", fails)
	}
	ok("root %s", root)

	l, err := stack.Load(ctx, root)
	if l == nil {
		fail("%v", err)
		return fmt.Errorf("%d problem(s)", fails)
	}
	ok("%s parsed", config.FileName)
	for _, b := range stack.BinRoots(l.Config) {
		// <root>/bin stays empty until devstack can download builds (M8).
		if b == filepath.Join(root, "bin") {
			continue
		}
		if _, statErr := os.Stat(b); statErr != nil {
			warn("paths.bin: %s does not exist", b)
		}
	}
	for _, k := range []discover.Kind{discover.Nginx, discover.PHP, discover.MySQL} {
		builds := l.Found.Builds[k]
		if len(builds) == 0 {
			continue
		}
		var vs []string
		for _, b := range builds {
			vs = append(vs, b.Version.String())
		}
		ok("%s installed: %s", k, strings.Join(vs, ", "))
	}
	if err != nil {
		for _, line := range strings.Split(err.Error(), "\n") {
			fail("%s", line)
		}
	}
	st := l.Stack
	if st == nil {
		return fmt.Errorf("%d problem(s)", fails)
	}
	for _, w := range st.Warnings {
		warn("%s", w)
	}
	if st.DefaultPHP != nil {
		ok("php default %s, %d worker(s) on %s", st.DefaultPHP.Build.Version, len(st.DefaultPHP.Ports), control.PortRange(st.DefaultPHP.Ports))
	}
	if st.MySQL != nil {
		ok("mysql %s, datadir %s", st.MySQL.Build.Version, st.MySQL.DataDir)
	}
	ok("%d site(s) in %s", len(st.Sites), st.WWW)
	if v, has := winproc.VCRuntime(); has {
		ok("Visual C++ runtime %s", v)
	} else {
		fail("Visual C++ 2015-2022 x64 runtime missing; PHP and MySQL need it: %s", setup.VCRedistURL)
	}

	if c, cerr := api.Connect(root); cerr == nil {
		ok("daemon running, pid %d (ports are in use by it)", c.State.PID)
	} else {
		ok("daemon not running")
		conflicts := control.PortConflicts(st)
		for _, c := range conflicts {
			fail("port %s", c)
		}
		if len(conflicts) == 0 {
			ok("all ports free (%d)", len(st.Ports()))
		}
	}

	probe := filepath.Join(root, ".devstack-write-test")
	if werr := os.WriteFile(probe, nil, 0o644); werr != nil {
		fail("cannot write to %s: %v", root, werr)
	} else {
		os.Remove(probe)
		ok("root is writable")
	}

	if fails > 0 {
		return fmt.Errorf("%d problem(s)", fails)
	}
	fmt.Fprintln(e.stdout, "\nAll good.")
	return nil
}
