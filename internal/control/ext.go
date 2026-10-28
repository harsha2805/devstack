package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/packages"
	"github.com/harsha2805/devstack/internal/winproc"
)

// ExtProgress reports one extension download: bytes done out of total.
type ExtProgress func(x packages.Extension, done, total int64)

// EnableExtensions turns on PHP extensions the catalog has (sqlsrv,
// pdo_sqlsrv): it downloads each into every PHP version in devstack's bin/
// that lacks it, adds the names to php.extensions in stack.toml, and reloads
// the running stack so the PHP pools restart with them.
func EnableExtensions(ctx context.Context, root string, names []string, progress ExtProgress) error {
	cat, err := packages.Default()
	if err != nil {
		return err
	}
	for _, n := range names {
		if !slices.Contains(cat.ExtensionNames(), n) {
			return fmt.Errorf("devstack can't download the %s extension (it can: %s); php.net's PHP already includes the common ones, so check php.extensions in stack.toml",
				n, strings.Join(cat.ExtensionNames(), ", "))
		}
	}
	if err := FetchExtensions(ctx, root, names, progress); err != nil {
		return err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	list := slices.Clone(cfg.PHP.Extensions)
	for _, n := range names {
		// Microsoft's own zip names the DLL php_sqlsrv_83_nts_x64.dll; a stack
		// set up by hand lists "sqlsrv_83_nts_x64", and loading both fails.
		list = slices.DeleteFunc(list, func(e string) bool { return vendorName(e, n) })
		if !slices.Contains(list, n) {
			list = append(list, n)
		}
	}
	if !slices.Equal(list, cfg.PHP.Extensions) {
		if err := config.SetExtensions(root, list); err != nil {
			return err
		}
	}
	return reloadIfRunning(ctx, root)
}

// DisableExtensions removes names from php.extensions and reloads. The DLLs
// stay in place, so turning one back on needs no download.
func DisableExtensions(ctx context.Context, root string, names []string) error {
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	list := slices.DeleteFunc(slices.Clone(cfg.PHP.Extensions), func(e string) bool { return slices.Contains(names, e) })
	if len(list) == len(cfg.PHP.Extensions) {
		return nil
	}
	if err := config.SetExtensions(root, list); err != nil {
		return err
	}
	return reloadIfRunning(ctx, root)
}

// FetchExtensions downloads the named catalog extensions into every PHP
// version under devstack's bin/ that lacks them; names the catalog doesn't
// have are skipped. PHP builds in other paths.bin folders belong to other
// tools and are left alone.
func FetchExtensions(ctx context.Context, root string, names []string, progress ExtProgress) error {
	cat, err := packages.Default()
	if err != nil {
		return err
	}
	var errs []error
	for _, b := range discover.Scan(ctx, []string{filepath.Join(root, "bin")}).Builds[discover.PHP] {
		for _, n := range names {
			if !slices.Contains(cat.ExtensionNames(), n) {
				continue
			}
			x, err := cat.FindExtension(n, b.Version.MajorMinor())
			if err != nil {
				errs = append(errs, err)
				continue
			}
			_, err = packages.InstallExtension(ctx, filepath.Join(b.Dir, "ext"), x, func(done, total int64) {
				if progress != nil {
					progress(x, done, total)
				}
			})
			if err != nil && !errors.Is(err, packages.ErrInstalled) {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// ExtensionStatus is one installable extension as the CLI and app show it.
type ExtensionStatus struct {
	Name    string   `json:"name"`
	Enabled bool     `json:"enabled"`           // listed in php.extensions, by name or Microsoft's ("sqlsrv_83_nts_x64")
	Missing []string `json:"missing,omitempty"` // enabled, but PHP minors in bin/ that lack the DLL
	ODBC    bool     `json:"odbc"`              // needs Microsoft's ODBC Driver for SQL Server
}

// ODBCStatus says whether enabled extensions need the ODBC driver and which
// one is installed.
type ODBCStatus struct {
	Needed bool   `json:"needed"`
	Driver string `json:"driver,omitempty"` // "ODBC Driver 18 for SQL Server"; empty when none
	URL    string `json:"url"`
}

// CheckExtensions lists the catalog's extensions with their state in root's
// stack, and whether the ODBC driver is missing for the ones turned on.
func CheckExtensions(ctx context.Context, root string) ([]ExtensionStatus, ODBCStatus, error) {
	odbc := ODBCStatus{URL: packages.ODBCDriverURL}
	cfg, err := config.Load(root)
	if err != nil {
		return nil, odbc, err
	}
	cat, err := packages.Default()
	if err != nil {
		return nil, odbc, err
	}
	builds := discover.Scan(ctx, []string{filepath.Join(root, "bin")}).Builds[discover.PHP]
	var out []ExtensionStatus
	for _, n := range cat.ExtensionNames() {
		enabled := slices.ContainsFunc(cfg.PHP.Extensions, func(e string) bool { return model.ExtensionBase(e) == n })
		s := ExtensionStatus{Name: n, Enabled: enabled, ODBC: packages.NeedsODBC(n)}
		if s.Enabled {
			for _, b := range builds {
				if _, ok := model.ExtensionFile(b, n); !ok && !slices.Contains(s.Missing, b.Version.MajorMinor()) {
					s.Missing = append(s.Missing, b.Version.MajorMinor())
				}
			}
			odbc.Needed = odbc.Needed || s.ODBC
		}
		out = append(out, s)
	}
	if odbc.Needed {
		odbc.Driver, _ = winproc.ODBCSQLServer()
	}
	return out, odbc, nil
}

var vendorSuffix = regexp.MustCompile(`^_\d+_n?ts_x(64|86)$`)

// vendorName reports whether e is name with Microsoft's build suffix
// ("sqlsrv_83_nts_x64" for "sqlsrv").
func vendorName(e, name string) bool {
	return strings.HasPrefix(e, name) && vendorSuffix.MatchString(strings.TrimPrefix(e, name))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Hint is the line shown when sqlsrv is on but the driver is missing.
func (o ODBCStatus) Hint() string {
	if !o.Needed || o.Driver != "" {
		return ""
	}
	return fmt.Sprintf("sqlsrv and pdo_sqlsrv need Microsoft's ODBC Driver for SQL Server (17 or 18), which isn't installed. "+
		"PHP loads them, but connecting fails until it is. Download it from %s (its installer asks for administrator rights).", o.URL)
}
