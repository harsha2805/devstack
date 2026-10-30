package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/control"
	"github.com/harsha2805/devstack/internal/stack"
)

// cmdProject serves folders outside www: add, remove and list [projects].
func cmdProject(ctx context.Context, e *env, args []string) error {
	if len(args) == 0 {
		return usageError{"expected add, remove or list"}
	}
	root, err := e.root()
	if err != nil {
		return err
	}
	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("project add", flag.ContinueOnError)
		name := fs.String("name", "", "name to serve it under (default: the folder's name)")
		pos, err := parse(fs, args[1:])
		if err != nil {
			return err
		}
		if len(pos) != 1 {
			return usageError{"expected one folder"}
		}
		res, err := control.AddProject(ctx, root, pos[0], *name)
		var nameErr *control.NameError
		if errors.As(err, &nameErr) {
			return fmt.Errorf("%s (try --name %s)", nameErr.Msg, nameErr.Suggestion)
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(e.stdout, "Added %s.\n", res.Name)
		for _, w := range res.Warnings {
			fmt.Fprintln(e.stdout, "warning: "+w)
		}
		if l, err := stack.Load(ctx, root); err == nil && l.Stack != nil {
			printSites(e, l.Stack)
		}
	case "remove":
		if len(args) != 2 {
			return usageError{"expected the project's name"}
		}
		if err := control.RemoveProject(ctx, root, args[1]); err != nil {
			return err
		}
		fmt.Fprintf(e.stdout, "%s is no longer served. Its folder is untouched.\n", args[1])
	case "list":
		if len(args) != 1 {
			return usageError{"unexpected argument " + args[1]}
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}
		projects := stack.Projects(cfg)
		if len(projects) == 0 {
			fmt.Fprintln(e.stdout, "No projects outside www. Add one with `devstack project add <folder>`.")
			return nil
		}
		tw := tabwriter.NewWriter(e.stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "NAME\tFOLDER\t")
		for _, p := range projects {
			note := ""
			if st, err := os.Stat(p.Dir); err != nil || !st.IsDir() {
				note = "(missing)"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\n", p.Name, p.Dir, note)
		}
		tw.Flush()
	default:
		return usageError{"expected add, remove or list"}
	}
	return nil
}
