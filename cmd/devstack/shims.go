package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/harsha2805/devstack/internal/control"
	"github.com/harsha2805/devstack/internal/shim"
)

// cmdShims reports or fixes whether `php` and `composer` in terminals run
// devstack's shims, which pick each site's PHP.
func cmdShims(ctx context.Context, e *env, args []string) error {
	if len(args) == 2 && args[0] == "fix-system" {
		// Run as administrator by FixShimPath. The shims beside this
		// devstack.exe are copied; the argument only has to agree.
		dir := shim.Dir()
		if dir == "" || !strings.EqualFold(filepath.Clean(args[1]), dir) {
			return fmt.Errorf("fix-system: %s isn't this devstack's shims folder", args[1])
		}
		return control.InstallSystemShims(dir)
	}
	if len(args) == 1 && args[0] == "remove-system" {
		// Run as administrator by the uninstaller.
		return control.RemoveSystemShims()
	}
	if len(args) > 1 || (len(args) == 1 && args[0] != "status" && args[0] != "fix") {
		return usageError{"expected status or fix"}
	}
	s, ok := control.ShimStatus()
	if !ok {
		return fmt.Errorf("this devstack.exe has no shims folder next to it; install devstack with its installer")
	}
	if len(args) == 1 && args[0] == "fix" && !s.OK() {
		if s.NeedsAdmin() {
			fmt.Fprintln(e.stdout, "This changes the system PATH, so Windows will ask for administrator approval once.")
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
