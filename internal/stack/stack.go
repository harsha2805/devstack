// Package stack runs the full resolve pipeline: stack.toml, bin discovery,
// site scan, model. The CLI and the daemon both start here.
package stack

import (
	"context"
	"path/filepath"
	"sort"

	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/sites"
	"github.com/harsha2805/devstack/internal/winproc"
)

// Loaded is everything the pipeline produced, kept for diagnostics.
type Loaded struct {
	Config   config.Stack
	Found    discover.Result
	WWW      string          // absolute www folder
	Projects []sites.Project // [projects], with absolute folders
	Sites    []sites.Site    // the raw scan the model was built from (the daemon's watcher compares against it)
	Stack    *model.Stack
}

// Load returns the resolved stack. On a resolve error Stack may be partially
// filled, which is still useful for `doctor`.
func Load(ctx context.Context, root string) (*Loaded, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	l := &Loaded{Config: cfg}
	l.Found = discover.Scan(ctx, BinRoots(cfg))

	www := cfg.WWW
	if filepath.IsAbs(www) {
		www = winproc.LongPath(www)
		cfg.WWW = www
	} else {
		www = filepath.Join(cfg.Root, www)
	}
	l.WWW = www
	l.Projects = Projects(cfg)
	list, warns, err := sites.Scan(www, l.Projects)
	if err != nil {
		return l, err
	}
	l.Sites = list
	l.Stack, err = model.Resolve(cfg, l.Found, list, append(l.Found.Problems, warns...))
	if l.Stack != nil {
		l.Stack.Projects = l.Projects
	}
	return l, err
}

// Projects resolves [projects] against the root, sorted by name.
func Projects(cfg config.Stack) []sites.Project {
	out := make([]sites.Project, 0, len(cfg.Projects))
	for name, p := range cfg.Projects {
		if filepath.IsAbs(p) {
			p = winproc.LongPath(filepath.Clean(p))
		} else {
			p = filepath.Join(cfg.Root, p)
		}
		out = append(out, sites.Project{Name: name, Dir: p})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// BinRoots resolves paths.bin against the root.
func BinRoots(cfg config.Stack) []string {
	out := make([]string, len(cfg.Paths.Bin))
	for i, p := range cfg.Paths.Bin {
		if filepath.IsAbs(p) {
			out[i] = winproc.LongPath(filepath.Clean(p))
		} else {
			out[i] = filepath.Join(cfg.Root, p)
		}
	}
	return out
}
