package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

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
		if len(rest) < 1 || len(rest) > 2 {
			return usageError{"usage: pkg install <nginx|php|mysql> [version]"}
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
		if err == nil {
			fmt.Fprintln(e.stdout, "Run `devstack reload` (or `up`) to use it.")
		}
		return err
	case "remove", "rm":
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
	return nil
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
