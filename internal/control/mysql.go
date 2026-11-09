package control

import (
	"context"
	"fmt"

	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/stack"
)

// SetMySQL sets services.mysql and reloads a running stack. version is
// config.Off, or a major.minor ("9.7") that must already be installed: a
// version that isn't would stop the whole stack from loading. MySQL starts
// (or stops) with the reload; a version used for the first time gets a fresh
// datadir under data/mysql/<major.minor>.
func SetMySQL(ctx context.Context, root, version string) error {
	if version != config.Off {
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}
		if _, err := discover.ParseVersion(version); err != nil {
			return fmt.Errorf("MySQL version: %w", err)
		}
		if _, err := discover.Scan(ctx, stack.BinRoots(cfg)).Pick(discover.MySQL, version); err != nil {
			return fmt.Errorf("MySQL %s isn't installed; install it first", version)
		}
	}
	if err := config.SetService(root, "mysql", version); err != nil {
		return err
	}
	return reloadIfRunning(ctx, root)
}
