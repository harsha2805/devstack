package main

import (
	"context"
	"fmt"

	"github.com/harsha2805/devstack/internal/certs"
	"github.com/harsha2805/devstack/internal/control"
	"github.com/harsha2805/devstack/internal/stack"
)

// cmdHTTPS turns trusted HTTPS on or off, reports it, or (untrust) removes
// devstack's CA from Windows; the uninstaller uses that last one.
func cmdHTTPS(ctx context.Context, e *env, args []string) error {
	if len(args) != 1 {
		return usageError{"expected on, off, status or untrust"}
	}
	if args[0] == "untrust" {
		n, err := certs.Untrust()
		if err == nil {
			err = certs.Delete()
		}
		fmt.Fprintf(e.stdout, "Removed %d devstack certificate(s) from Windows.\n", n)
		return err
	}
	root, err := e.root()
	if err != nil {
		return err
	}
	switch args[0] {
	case "on":
		fmt.Fprintln(e.stdout, "Windows will ask once whether to trust devstack's local certificate authority.")
		fmt.Fprintln(e.stdout, "It can only sign local names (*.localhost and your TLD), never real websites.")
		if err := control.SetHTTPS(ctx, root, true); err != nil {
			return err
		}
		fmt.Fprintln(e.stdout, "On.")
		if l, err := stack.Load(ctx, root); err == nil && l.Stack != nil {
			printSites(e, l.Stack)
		}
	case "off":
		if err := control.SetHTTPS(ctx, root, false); err != nil {
			return err
		}
		fmt.Fprintln(e.stdout, "Off. The devstack certificate is no longer trusted and was deleted.")
	case "status":
		s, err := control.CheckHTTPS(root)
		if err != nil {
			return err
		}
		yes := map[bool]string{true: "yes", false: "no"}
		fmt.Fprintf(e.stdout, "services.https    %s (port %d)\n", map[bool]string{true: "on", false: "off"}[s.On], s.Port)
		fmt.Fprintf(e.stdout, "CA trusted        %s\n", yes[s.Trusted])
		if s.CA != "" {
			fmt.Fprintf(e.stdout, "CA certificate    %s\n", s.CA)
		}
		if s.On && !s.Trusted {
			fmt.Fprintln(e.stdout, "\nBrowsers will warn: run `devstack https on` to trust the certificate again.")
		}
	default:
		return usageError{"unknown action " + args[0]}
	}
	return nil
}
