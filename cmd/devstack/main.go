// Command devstack runs a local web dev stack (nginx, PHP, MySQL) from one config file.
package main

//go:generate go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --out rsrc

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"

	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/packages"
)

// version is set at build time: -ldflags "-X main.version=0.1.0".
var version = "dev"

type command struct {
	name    string
	args    string
	summary string
	hidden  bool
	run     func(ctx context.Context, env *env, args []string) error
}

// env carries global options to every command.
type env struct {
	rootFlag string
	stdout   io.Writer
	stderr   io.Writer
}

// root finds the stack root, or explains how to create one.
func (e *env) root() (string, error) { return config.FindRoot(e.rootFlag) }

type usageError struct{ msg string }

func (u usageError) Error() string { return u.msg }

var commands []command

func init() {
	commands = []command{
		{"init", "[--www DIR] [--bin DIR]... [--force]", "create stack.toml from the servers found on this machine", false, cmdInit},
		{"up", "[--foreground]", "render config and start every service", false, cmdUp},
		{"down", "", "stop every service", false, cmdDown},
		{"status", "[--json]", "show services and their state", false, cmdStatus},
		{"reload", "", "rescan www and stack.toml, apply changes without downtime", false, cmdReload},
		{"restart", "[SERVICE]", "restart one service (nginx, php-8.3, mysql) or all", false, cmdRestart},
		{"sites", "", "list sites and their URLs", false, cmdSites},
		{"logs", "[NAME] [-f] [-n N] [--list]", "show a log: nginx, php-8.3, mysql, a site, or the daemon", false, cmdLogs},
		{"doctor", "", "check config, servers, ports and folders", false, cmdDoctor},
		{"render", "", "write etc/ without starting anything", false, cmdRender},
		{"db", "creds", "show MySQL connection details", false, cmdDB},
		{"dns", "on | off | status", "<site>.test addresses for every program (one admin prompt)", false, cmdDNS},
		{"https", "on | off | status | untrust", "trusted HTTPS for every site (Windows asks once)", false, cmdHTTPS},
		{"guide", "add | remove | show [SITE]", "AI guide for coding agents in a site's folder", false, cmdGuide},
		{"pkg", "list | install SERVER [VERSION] | remove SERVER VERSION | install/remove ext NAME...", "download nginx, PHP and MySQL builds, and extra PHP extensions (sqlsrv)", false, cmdPkg},
		{"version", "", "print the version", false, cmdVersion},
		{"daemon", "", "run the supervisor (started by `up`)", true, cmdDaemon},
	}
}

func main() {
	packages.UserAgent = "devstack/" + version
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	e := &env{stdout: stdout, stderr: stderr}
	args, rootFlag, err := extractRoot(args)
	if err != nil {
		fmt.Fprintln(stderr, "devstack:", err)
		return 2
	}
	e.rootFlag = rootFlag

	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		usage(stdout)
		return 0
	}
	for _, c := range commands {
		if c.name != args[0] {
			continue
		}
		err := c.run(ctx, e, args[1:])
		var ue usageError
		switch {
		case err == nil:
			return 0
		case errors.Is(err, flag.ErrHelp):
			return 0
		case errors.As(err, &ue):
			fmt.Fprintf(stderr, "devstack %s: %s\nusage: devstack %s %s\n", c.name, ue.msg, c.name, c.args)
			return 2
		default:
			fmt.Fprintf(stderr, "devstack %s: %v\n", c.name, err)
			return 1
		}
	}
	fmt.Fprintf(stderr, "devstack: unknown command %q\n\n", args[0])
	usage(stderr)
	return 2
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "devstack: local nginx + PHP + MySQL for *.test sites")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "usage: devstack [--root DIR] <command> [options]")
	fmt.Fprintln(w)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, c := range commands {
		if !c.hidden {
			fmt.Fprintf(tw, "  %s %s\t%s\n", c.name, c.args, c.summary)
		}
	}
	tw.Flush()
	fmt.Fprintln(w)
	fmt.Fprintln(w, "The root is the folder holding stack.toml: --root, else DEVSTACK_ROOT, else the")
	fmt.Fprintln(w, "nearest parent of the current folder, else the folder of devstack.exe.")
}

// extractRoot pulls --root DIR / --root=DIR from anywhere in args.
func extractRoot(args []string) ([]string, string, error) {
	var out []string
	root := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--root" || a == "-root":
			if i+1 >= len(args) {
				return nil, "", errors.New("--root needs a folder")
			}
			root = args[i+1]
			i++
		case strings.HasPrefix(a, "--root="), strings.HasPrefix(a, "-root="):
			root = a[strings.IndexByte(a, '=')+1:]
		default:
			out = append(out, a)
		}
	}
	return out, root, nil
}

// parse handles flags placed before or after positional arguments.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	fs.SetOutput(io.Discard)
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil, err
			}
			return nil, usageError{err.Error()}
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func cmdVersion(_ context.Context, e *env, _ []string) error {
	fmt.Fprintln(e.stdout, "devstack", version)
	return nil
}
