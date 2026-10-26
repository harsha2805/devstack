// Package stack runs the full resolve pipeline: stack.toml, bin discovery,
// site scan, model. The CLI and the daemon both start here.
package stack

import (
	"context"
	"path/filepath"

	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/sites"
	"github.com/harsha2805/devstack/internal/winproc"
)

// Loaded is everything the pipeline produced, kept for diagnostics.
type Loaded struct {
	Config config.Stack
	Found  discover.Result
	Stack  *model.Stack
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
	list, warns, err := sites.Scan(www)
	if err != nil {
		return l, err
	}
	l.Stack, err = model.Resolve(cfg, l.Found, list, append(l.Found.Problems, warns...))
	return l, err
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
