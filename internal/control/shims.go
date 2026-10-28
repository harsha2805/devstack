package control

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/harsha2805/devstack/internal/shim"
	"github.com/harsha2805/devstack/internal/winproc"
)

// ShimStatus says whether `php` and `composer` in a new terminal run
// devstack's shims. ok is false when this devstack has no shims folder (a
// development build).
func ShimStatus() (s shim.PathStatus, ok bool) {
	dir := shim.Dir()
	if dir == "" {
		return s, false
	}
	system, user := winproc.EnvPaths()
	return shim.CheckPath(dir, system, user), true
}

// FixShimPath puts the shims first on PATH: on the user PATH when nothing on
// the system PATH is in the way, else on the system PATH, which runs
// devstack.exe (beside the shims folder) as administrator, one UAC prompt.
func FixShimPath() error {
	s, ok := ShimStatus()
	if !ok {
		return errors.New("this devstack has no shims folder")
	}
	if s.OK() {
		return nil
	}
	if !s.BlockerSystem {
		return winproc.SetUserPath(func(p string) string { return shim.Prepend(p, s.Dir) })
	}
	exe := filepath.Join(filepath.Dir(s.Dir), "devstack.exe")
	if err := winproc.RunElevated(exe, fmt.Sprintf(`shims fix-system "%s"`, s.Dir)); err != nil {
		return err
	}
	if s, _ = ShimStatus(); !s.OK() {
		return errors.New(s.Problem())
	}
	return nil
}

// OpenTerminal opens PowerShell in site's folder, where php and composer run
// the site's PHP, and says which.
func OpenTerminal(root, site string) error {
	m, err := shim.Load(root)
	if err != nil {
		return err
	}
	var s *shim.Site
	for i := range m.Sites {
		if m.Sites[i].Name == site {
			s = &m.Sites[i]
		}
	}
	if s == nil {
		return fmt.Errorf("no site %q", site)
	}
	php := m.PHP[s.PHP]
	system, user := winproc.EnvPaths()
	path := strings.Join([]string{system, user}, ";")
	if dir := shim.Dir(); dir != "" {
		path = shim.Prepend(path, dir)
	} else {
		// A build without shims: put the site's PHP itself first.
		path = shim.Prepend(path, strings.TrimSuffix(php.Exe, `\php.exe`))
	}
	env := append(winproc.Environ(path, "PHPRC", "DEVSTACK_PHP"), "PHPRC="+strings.TrimSuffix(php.INI, `\php.ini`))
	startup := fmt.Sprintf(`Write-Host "%s: php and composer here run PHP %s, with devstack's php.ini." -ForegroundColor Cyan`, s.Name, php.Version)
	return winproc.OpenTerminal(s.Dir, env, fmt.Sprintf("%s - PHP %s", s.Name, php.Version), startup)
}
