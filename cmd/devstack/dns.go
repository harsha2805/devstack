package main

import (
	"context"
	"fmt"

	"github.com/harsha2805/devstack/internal/control"
	"github.com/harsha2805/devstack/internal/stack"
)

// cmdDNS turns <site>.<tld> addresses on or off, or reports their state.
func cmdDNS(ctx context.Context, e *env, args []string) error {
	if len(args) != 1 || (args[0] != "on" && args[0] != "off" && args[0] != "status") {
		return usageError{"expected on, off or status"}
	}
	root, err := e.root()
	if err != nil {
		return err
	}
	switch args[0] {
	case "on":
		fmt.Fprintln(e.stdout, "Windows will ask for administrator approval once, to send only *.test lookups to devstack.")
		if err := control.SetDNS(ctx, root, true); err != nil {
			return err
		}
		s, _ := control.CheckDNS(ctx, root, true)
		fmt.Fprintf(e.stdout, "On. Sites now answer at <name>.%s in every program", s.TLD)
		if !s.Answers {
			fmt.Fprint(e.stdout, " (while the servers run: start them with `devstack up`)")
		}
		fmt.Fprintln(e.stdout, ".")
		if l, err := stack.Load(ctx, root); err == nil && l.Stack != nil {
			printSites(e, l.Stack)
		}
	case "off":
		if err := control.SetDNS(ctx, root, false); err != nil {
			return err
		}
		fmt.Fprintln(e.stdout, "Off. Sites answer at <name>.localhost again (browsers and curl).")
	case "status":
		s, err := control.CheckDNS(ctx, root, true)
		if err != nil {
			return err
		}
		yes := map[bool]string{true: "yes", false: "no"}
		fmt.Fprintf(e.stdout, "services.dns      %s\n", map[bool]string{true: "on", false: "off"}[s.On])
		fmt.Fprintf(e.stdout, "Windows rule      %s (*.%s -> 127.0.0.1)\n", yes[s.Rule], s.TLD)
		if s.On && s.Rule {
			fmt.Fprintf(e.stdout, "answering now     %s\n", yes[s.Answers])
		}
		switch {
		case s.On && !s.Rule:
			fmt.Fprintln(e.stdout, "\nThe Windows rule is missing. Run `devstack dns on` to add it again.")
		case !s.On && s.Rule:
			fmt.Fprintln(e.stdout, "\nThe Windows rule is left over. Run `devstack dns off` to remove it.")
		}
	}
	return nil
}
