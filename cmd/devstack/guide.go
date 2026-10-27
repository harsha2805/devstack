package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/harsha2805/devstack/internal/aiguide"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/stack"
)

// cmdGuide adds, removes or prints a site's AI guide. With no SITE it uses the
// site whose folder holds the current directory.
func cmdGuide(ctx context.Context, e *env, args []string) error {
	if len(args) == 0 || len(args) > 2 {
		return usageError{"expected add, remove or show"}
	}
	action := args[0]
	if action != "add" && action != "remove" && action != "show" {
		return usageError{"unknown action " + action}
	}
	root, err := e.root()
	if err != nil {
		return err
	}
	l, err := stack.Load(ctx, root)
	if err != nil {
		return err
	}
	slug := ""
	if len(args) == 2 {
		slug = args[1]
	}
	site, err := findSite(l.Stack, slug)
	if err != nil {
		return err
	}
	cli := aiguide.CLI()
	switch action {
	case "add":
		if err := aiguide.Install(l.Stack, site, cli); err != nil {
			return err
		}
		fmt.Fprintf(e.stdout, "Added the AI guide to %s:\n  %s\n  pointer blocks in AGENTS.md and CLAUDE.md\n",
			site.Slug, filepath.Join(site.Dir, aiguide.Dir, aiguide.File))
	case "remove":
		if err := aiguide.Remove(site.Dir); err != nil {
			return err
		}
		fmt.Fprintf(e.stdout, "Removed the AI guide from %s.\n", site.Slug)
	case "show":
		fmt.Fprint(e.stdout, aiguide.Render(l.Stack, site, cli))
	}
	return nil
}

func findSite(st *model.Stack, slug string) (model.Site, error) {
	if slug != "" {
		for _, s := range st.Sites {
			if s.Slug == slug || strings.EqualFold(s.Name, slug) {
				return s, nil
			}
		}
		return model.Site{}, fmt.Errorf("no site %q; see `devstack sites`", slug)
	}
	wd, err := os.Getwd()
	if err != nil {
		return model.Site{}, err
	}
	wd = strings.ToLower(filepath.Clean(wd))
	for _, s := range st.Sites {
		dir := strings.ToLower(filepath.Clean(s.Dir))
		if wd == dir || strings.HasPrefix(wd, dir+string(filepath.Separator)) {
			return s, nil
		}
	}
	return model.Site{}, fmt.Errorf("run this inside a site's folder, or name the site (see `devstack sites`)")
}
