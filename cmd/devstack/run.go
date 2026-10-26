package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/harsha2805/devstack/internal/api"
	"github.com/harsha2805/devstack/internal/control"
	"github.com/harsha2805/devstack/internal/daemon"
	"github.com/harsha2805/devstack/internal/stack"
	"github.com/harsha2805/devstack/internal/supervisor"
)

func cmdUp(ctx context.Context, e *env, args []string) error {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	foreground := fs.Bool("foreground", false, "run in this terminal; Ctrl+C stops everything")
	if pos, err := parse(fs, args); err != nil {
		return err
	} else if len(pos) > 0 {
		return usageError{"unexpected argument " + pos[0]}
	}
	root, err := e.root()
	if err != nil {
		return err
	}

	if c, err := api.Connect(root); err == nil {
		fmt.Fprintln(e.stdout, "devstack is already running; applying changes.")
		res, err := c.Reload(ctx)
		printActions(e, res.Actions)
		if err != nil {
			return err
		}
		return printUpSummary(ctx, e, root)
	}

	st, res, err := control.Prepare(ctx, root)
	if st != nil {
		printWarnings(e, st.Warnings)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(e.stdout, "Rendered etc\\ (%d changed, %d removed)\n", len(res.Written), len(res.Removed))

	if *foreground {
		printSites(e, st)
		fmt.Fprintln(e.stdout, "Running in the foreground. Ctrl+C stops everything.")
		return daemon.Run(ctx, daemon.Options{Root: root, Version: version, Console: e.stderr})
	}

	c, err := control.Spawn(ctx, root, "")
	if err != nil {
		return err
	}
	err = control.Follow(ctx, root, c, func(g control.Group) {
		if g.State == supervisor.Running {
			fmt.Fprintf(e.stdout, "  ok    %-9s %-8s :%s\n", g.Name, g.Version, g.Ports)
		} else {
			fmt.Fprintf(e.stdout, "  FAIL  %-9s %s\n", g.Name, g.LastError)
		}
	})
	if err != nil {
		return fmt.Errorf("%w (logs: devstack logs NAME)", err)
	}
	return printUpSummary(ctx, e, root)
}

func printUpSummary(ctx context.Context, e *env, root string) error {
	l, err := stack.Load(ctx, root)
	if err != nil {
		return err
	}
	fmt.Fprintln(e.stdout)
	printSites(e, l.Stack)
	return nil
}

func cmdDown(ctx context.Context, e *env, args []string) error {
	if len(args) > 0 {
		return usageError{"unexpected argument " + args[0]}
	}
	root, err := e.root()
	if err != nil {
		return err
	}
	if _, err := api.Connect(root); errors.Is(err, api.ErrNotRunning) {
		fmt.Fprintln(e.stdout, "devstack is not running.")
		return nil
	}
	fmt.Fprint(e.stdout, "Stopping")
	err = control.Down(ctx, root, func() { fmt.Fprint(e.stdout, ".") })
	if err != nil {
		fmt.Fprintln(e.stdout)
		return err
	}
	fmt.Fprintln(e.stdout, " stopped.")
	return nil
}

func cmdStatus(ctx context.Context, e *env, args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print JSON")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	root, err := e.root()
	if err != nil {
		return err
	}
	c, err := api.Connect(root)
	if errors.Is(err, api.ErrNotRunning) {
		if *asJSON {
			return json.NewEncoder(e.stdout).Encode(map[string]string{"phase": "stopped"})
		}
		fmt.Fprintln(e.stdout, "devstack is not running. Start it with: devstack up")
		return nil
	}
	if err != nil {
		return err
	}
	s, err := c.Status(ctx)
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(e.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(s)
	}
	fmt.Fprintf(e.stdout, "devstack %s  pid %d  up %s  api %s\n\n", s.Phase, s.PID, since(s.Started), c.State.Addr)
	groups := control.Groups(s.Units)
	tw := tabwriter.NewWriter(e.stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "SERVICE\tVERSION\tSTATE\tPORT\tPID\tRESTARTS")
	for _, g := range groups {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\n", g.Name, g.Version, g.State, g.Ports, g.PIDText(), g.Restarts)
	}
	tw.Flush()
	for _, g := range groups {
		if g.LastError != "" && g.State != supervisor.Running {
			fmt.Fprintf(e.stdout, "\n%s: %s", g.Name, g.LastError)
		}
	}
	printWarnings(e, s.Warnings)
	return nil
}

func cmdReload(ctx context.Context, e *env, args []string) error {
	if len(args) > 0 {
		return usageError{"unexpected argument " + args[0]}
	}
	c, err := connect(e)
	if err != nil {
		return err
	}
	res, err := c.Reload(ctx)
	printActions(e, res.Actions)
	if err == nil && len(res.Actions) == 0 {
		fmt.Fprintln(e.stdout, "Nothing changed.")
	}
	return err
}

func cmdRestart(ctx context.Context, e *env, args []string) error {
	if len(args) > 1 {
		return usageError{"takes at most one service name"}
	}
	name := ""
	if len(args) == 1 {
		name = args[0]
	}
	c, err := connect(e)
	if err != nil {
		return err
	}
	res, err := c.Restart(ctx, name)
	printActions(e, res.Actions)
	return err
}

func cmdDaemon(ctx context.Context, e *env, _ []string) error {
	root, err := e.root()
	if err != nil {
		return err
	}
	return daemon.Run(ctx, daemon.Options{Root: root, Version: version})
}

func connect(e *env) (*api.Client, error) {
	root, err := e.root()
	if err != nil {
		return nil, err
	}
	c, err := api.Connect(root)
	if errors.Is(err, api.ErrNotRunning) {
		return nil, errors.New("devstack is not running; start it with `devstack up`")
	}
	return c, err
}

func since(t time.Time) string {
	d := time.Since(t).Round(time.Second)
	if d >= time.Hour {
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return d.String()
}

func printActions(e *env, actions []string) {
	for _, a := range actions {
		fmt.Fprintln(e.stdout, "  "+a)
	}
}

func printWarnings(e *env, warns []string) {
	for _, w := range warns {
		fmt.Fprintln(e.stderr, "warning:", w)
	}
}
