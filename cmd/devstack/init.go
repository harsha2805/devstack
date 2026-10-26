package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/packages"
	"github.com/harsha2805/devstack/internal/setup"
	"github.com/harsha2805/devstack/internal/winproc"
)

// multiFlag collects a repeatable string flag.
type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func cmdInit(ctx context.Context, e *env, args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	o := setup.Options{}
	fs.StringVar(&o.WWW, "www", "www", "projects folder")
	fs.BoolVar(&o.Force, "force", false, "overwrite an existing stack.toml")
	noDownload := fs.Bool("no-download", false, "don't download missing servers")
	fs.StringVar(&o.PHP, "php", "", "PHP version to use (downloaded if missing)")
	fs.StringVar(&o.MySQL, "mysql", "", `MySQL version to use (downloaded if missing), or "off"`)
	var bins multiFlag
	fs.Var(&bins, "bin", "extra folder with <service>/<version>/ builds (repeatable)")
	if pos, err := parse(fs, args); err != nil {
		return err
	} else if len(pos) > 0 {
		return usageError{"unexpected argument " + pos[0]}
	}
	root := e.rootFlag
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	o.Root = winproc.LongPath(abs)
	o.ExtraBins = bins
	o.Download = !*noDownload

	if o.Download {
		todo, err := setup.Plan(ctx, o)
		if err != nil {
			return err
		}
		if len(todo) > 0 {
			var total int64
			for _, en := range todo {
				total += en.Size
			}
			fmt.Fprintf(e.stdout, "Downloading %d server(s), %s, from their official sites:\n", len(todo), humanSize(total))
		}
	}
	start := time.Now()
	res, err := setup.Init(ctx, o, setup.Events{
		Download: func(en packages.Entry, done, total int64) {
			fmt.Fprintf(e.stdout, "\r  %-14s %s", en.String(), progressText(done, total, time.Since(start)))
		},
		Installed: func(en packages.Entry, dir string) {
			fmt.Fprintf(e.stdout, "\n  %-14s verified and installed\n", en.String())
			start = time.Now()
		},
		Port: func(name string, wanted, picked int, takenBy string) {
			if wanted == picked {
				fmt.Fprintf(e.stdout, "Port %-6s %d\n", name, picked)
			} else {
				fmt.Fprintf(e.stdout, "Port %-6s %d is taken (%s) -> using %d\n", name, wanted, takenBy, picked)
			}
		},
	})
	if err != nil {
		if !o.Download {
			return fmt.Errorf("%w\nrun without --no-download, or pass --bin with a folder of builds", err)
		}
		return err
	}
	fmt.Fprintln(e.stdout, "Servers:")
	for _, k := range []discover.Kind{discover.Nginx, discover.PHP, discover.MySQL} {
		for _, b := range res.Found.Builds[k] {
			fmt.Fprintf(e.stdout, "  %-6s %-8s %s\n", k, b.Version, b.Dir)
		}
	}
	if !res.VCRuntime {
		fmt.Fprintf(e.stderr, "warning: the Visual C++ 2015-2022 x64 runtime is missing; PHP and MySQL won't start.\n  Install it from %s\n", setup.VCRedistURL)
	}
	if res.SampleSite != "" {
		fmt.Fprintf(e.stdout, "Created sample site %s\n", res.SampleSite)
	}
	fmt.Fprintf(e.stdout, "Wrote %s. Edit it if needed, then run: devstack up\n", res.ConfigPath)
	return nil
}
