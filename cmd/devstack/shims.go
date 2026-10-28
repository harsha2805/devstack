package main

import (
	"context"
	"fmt"
	"os"

	"github.com/harsha2805/devstack/internal/control"
	"github.com/harsha2805/devstack/internal/shim"
	"github.com/harsha2805/devstack/internal/winproc"
)

// cmdShims reports or fixes whether `php` and `composer` in terminals run
// devstack's shims, which pick each site's PHP.
func cmdShims(ctx context.Context, e *env, args []string) error {
	if len(args) == 2 && args[0] == "fix-system" {
		// Run as administrator by FixShimPath.
		return winproc.SetSystemPath(func(p string) string { return shim.Prepend(p, args[1]) })
	}
	if len(args) > 1 || (len(args) == 1 && args[0] != "status" && args[0] != "fix") {
		return usageError{"expected status or fix"}
	}
	s, ok := control.ShimStatus()
	if !ok {
		return fmt.Errorf("this devstack.exe has no shims folder next to it; install devstack with its installer")
	}
	if len(args) == 1 && args[0] == "fix" && !s.OK() {
		if s.BlockerSystem {
			fmt.Fprintf(e.stdout, "%s is on the system PATH, so Windows will ask for administrator approval once.\n", s.Blocker)
		}
		if err := control.FixShimPath(); err != nil {
			return err
		}
		s, _ = control.ShimStatus()
	}
	fmt.Fprintf(e.stdout, "shims   %s\n", s.Dir)
	if !s.OK() {
		fmt.Fprintf(e.stdout, "\n%s\nRun `devstack shims fix` to put devstack's php and composer first.\n", s.Problem())
		return nil
	}
	fmt.Fprintln(e.stdout, "first on PATH: php and composer in new terminals run each site's PHP.")
	if root, err := e.root(); err == nil {
		if m, err := shim.Load(root); err == nil {
			cwd, _ := os.Getwd()
			if b, site, err := m.Pick(cwd, os.Getenv("DEVSTACK_PHP")); err == nil {
				where := "outside every site: the default PHP"
				if site != nil {
					where = "site " + site.Name
				}
				fmt.Fprintf(e.stdout, "here    PHP %s (%s)\n", b.Version, where)
			}
		}
	}
	return nil
}

// cmdTerminal opens a terminal in a site's folder, where php and composer run
// the site's PHP.
func cmdTerminal(ctx context.Context, e *env, args []string) error {
	if len(args) != 1 {
		return usageError{"usage: terminal SITE"}
	}
	root, err := e.root()
	if err != nil {
		return err
	}
	return control.OpenTerminal(root, args[0])
}
