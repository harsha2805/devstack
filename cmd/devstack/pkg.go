package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/control"
	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/packages"
)

func cmdPkg(ctx context.Context, e *env, args []string) error {
	if len(args) == 0 {
		return usageError{"needs a subcommand: list, install, remove"}
	}
	root, err := e.root()
	if err != nil {
		return err
	}
	bin := filepath.Join(root, "bin")
	cat, err := packages.Default()
	if err != nil {
		return err
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list", "ls":
		return pkgList(ctx, e, cat, bin)
	case "install", "add":
		if len(rest) > 0 && rest[0] == "ext" {
			return pkgExt(ctx, e, root, rest[1:], true)
		}
		if len(rest) < 1 || len(rest) > 2 {
			return usageError{"usage: pkg install <nginx|php|mysql> [version], or pkg install ext <name>..."}
		}
		kind, err := parseKind(rest[0])
		if err != nil {
			return err
		}
		constraint := ""
		if len(rest) == 2 {
			constraint = rest[1]
		}
		entry, err := cat.Find(kind, constraint)
		if err != nil {
			return err
		}
		_, err = installWithProgress(ctx, e, bin, entry)
		if errors.Is(err, packages.ErrInstalled) {
			fmt.Fprintf(e.stdout, "%s is already installed.\n", entry)
			return nil
		}
		if err == nil && kind == discover.PHP {
			// Extensions devstack downloads (sqlsrv) aren't in php.net's zip.
			if cfg, cerr := config.Load(root); cerr == nil {
				err = control.FetchExtensions(ctx, root, cfg.PHP.Extensions, extProgress(e))
			}
		}
		if err == nil {
			fmt.Fprintln(e.stdout, "Run `devstack reload` (or `up`) to use it.")
		}
		return err
	case "remove", "rm":
		if len(rest) > 0 && rest[0] == "ext" {
			return pkgExt(ctx, e, root, rest[1:], false)
		}
		if len(rest) != 2 {
			return usageError{"usage: pkg remove <nginx|php|mysql> <exact version>"}
		}
		kind, err := parseKind(rest[0])
		if err != nil {
			return err
		}
		if err := packages.Remove(bin, kind, rest[1]); err != nil {
			return err
		}
		fmt.Fprintf(e.stdout, "Removed %s %s.\n", kind, rest[1])
		return nil
	}
	return usageError{"unknown pkg subcommand " + sub}
}

func pkgList(ctx context.Context, e *env, cat packages.Catalog, bin string) error {
	found := discover.Scan(ctx, []string{bin})
	installed := map[string]bool{}
	for _, builds := range found.Builds {
		for _, b := range builds {
			installed[string(b.Kind)+" "+b.Version.String()] = true
		}
	}
	tw := tabwriter.NewWriter(e.stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "SERVER\tVERSION\tSIZE\tNOTE\tSTATUS")
	for _, en := range cat.Entries {
		status := ""
		if installed[string(en.Kind)+" "+en.Version] {
			status = "installed"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", en.Kind, en.Version, humanSize(en.Size), en.Label, status)
	}
	tw.Flush()
	fmt.Fprintf(e.stdout, "\nCatalog from %s. Installs go to %s.\n", cat.Generated.Format("2006-01-02"), bin)

	exts, odbc, err := control.CheckExtensions(ctx, filepath.Dir(bin))
	if err != nil || len(exts) == 0 {
		return nil
	}
	fmt.Fprintln(e.stdout, "\nPHP extensions devstack can download (pkg install ext <name>):")
	tw = tabwriter.NewWriter(e.stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "EXTENSION\tSTATUS")
	for _, x := range exts {
		status := "off"
		switch {
		case x.Enabled && len(x.Missing) > 0:
			status = "on, not downloaded for PHP " + strings.Join(x.Missing, ", ")
		case x.Enabled:
			status = "on"
		}
		fmt.Fprintf(tw, "%s\t%s\n", x.Name, status)
	}
	tw.Flush()
	if h := odbc.Hint(); h != "" {
		fmt.Fprintln(e.stdout, "\n"+h)
	}
	return nil
}

// pkgExt turns catalog PHP extensions on (download + php.extensions) or off.
func pkgExt(ctx context.Context, e *env, root string, names []string, on bool) error {
	if len(names) == 0 {
		return usageError{"usage: pkg install ext <name>... (or pkg remove ext <name>...), e.g. pkg install ext sqlsrv pdo_sqlsrv"}
	}
	for i, n := range names {
		names[i] = strings.TrimPrefix(strings.TrimSuffix(strings.ToLower(n), ".dll"), "php_")
	}
	if !on {
		if err := control.DisableExtensions(ctx, root, names); err != nil {
			return err
		}
		fmt.Fprintf(e.stdout, "Turned off %s in stack.toml. The files stay, so turning it back on needs no download.\n", strings.Join(names, ", "))
		return nil
	}
	if err := control.EnableExtensions(ctx, root, names, extProgress(e)); err != nil {
		return err
	}
	fmt.Fprintf(e.stdout, "Turned on %s in stack.toml; running PHP versions restart with it.\n", strings.Join(names, ", "))
	if _, odbc, err := control.CheckExtensions(ctx, root); err == nil {
		if h := odbc.Hint(); h != "" {
			fmt.Fprintln(e.stdout, "\n"+h)
		} else if odbc.Needed {
			fmt.Fprintf(e.stdout, "It connects through %s.\n", odbc.Driver)
		}
	}
	return nil
}

// extProgress prints a progress line per extension download.
func extProgress(e *env) control.ExtProgress {
	var start time.Time
	current := ""
	return func(x packages.Extension, done, total int64) {
		label := x.Name + " (PHP " + x.PHP + ")"
		if label != current {
			current, start = label, time.Now()
		}
		fmt.Fprintf(e.stdout, "\r  %-22s %s", label, progressText(done, total, time.Since(start)))
		if total > 0 && done >= total {
			fmt.Fprintf(e.stdout, "\r  %-22s verified and installed%s\n", label, strings.Repeat(" ", 30))
		}
	}
}

func installWithProgress(ctx context.Context, e *env, bin string, en packages.Entry) (string, error) {
	start := time.Now()
	label := fmt.Sprintf("%s %s", en.Kind, en.Version)
	dir, err := packages.Install(ctx, bin, en, func(done, total int64) {
		fmt.Fprintf(e.stdout, "\r  %-14s %s", label, progressText(done, total, time.Since(start)))
	})
	if errors.Is(err, packages.ErrInstalled) {
		return dir, err
	}
	fmt.Fprintln(e.stdout)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(e.stdout, "  %-14s verified and installed to %s\n", label, dir)
	return dir, nil
}

func progressText(done, total int64, elapsed time.Duration) string {
	speed := ""
	if s := elapsed.Seconds(); s > 0.5 {
		speed = fmt.Sprintf("  %s/s", humanSize(int64(float64(done)/s)))
	}
	if total <= 0 {
		return humanSize(done) + speed + "   "
	}
	pct := float64(done) * 100 / float64(total)
	return fmt.Sprintf("%5.1f%%  %s / %s%s   ", pct, humanSize(done), humanSize(total), speed)
}

func parseKind(s string) (discover.Kind, error) {
	switch k := discover.Kind(strings.ToLower(s)); k {
	case discover.Nginx, discover.PHP, discover.MySQL:
		return k, nil
	}
	return "", usageError{fmt.Sprintf("unknown server %q (nginx, php, mysql)", s)}
}
