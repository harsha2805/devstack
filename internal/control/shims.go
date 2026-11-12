package control

import (
	"errors"
	"fmt"
	"io"
	"os"
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
	s = shim.CheckPath(dir, winproc.SystemShimDir(), system, user)
	if s.System != "" {
		s.Stale = !sameShims(dir, s.System)
	}
	return s, true
}

// FixShimPath puts the shims first on PATH: on the user PATH when nothing on
// the system PATH is in the way, else on the system PATH, which runs
// devstack.exe (beside the shims folder) as administrator, one UAC prompt.
// The system PATH gets an administrators-only copy (InstallSystemShims).
func FixShimPath() error {
	s, ok := ShimStatus()
	if !ok {
		return errors.New("this devstack has no shims folder")
	}
	if s.OK() {
		return nil
	}
	if !s.NeedsAdmin() {
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

// shimNames are the shim executables, the same program under two names.
var shimNames = []string{"php.exe", "composer.exe"}

// InstallSystemShims runs as administrator. It copies the shims in src to
// winproc.SystemShimDir, where only administrators can change them, puts that
// folder first on the system PATH and takes src off it.
func InstallSystemShims(src string) error {
	dst := winproc.SystemShimDir()
	if dst == "" {
		return errors.New("can't find the Program Files folder")
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	if err := winproc.ProtectAdminFolder(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("limiting %s to administrators: %w", filepath.Dir(dst), err)
	}
	// Never put a folder on the system PATH that other accounts can write to.
	if who, err := winproc.WritableByOthers(dst); err != nil {
		return err
	} else if who != "" {
		return fmt.Errorf("%s can be changed by %s, so it can't go on the system PATH", dst, who)
	}
	for _, n := range shimNames {
		if err := replaceFile(filepath.Join(src, n), filepath.Join(dst, n)); err != nil {
			return err
		}
	}
	return winproc.SetSystemPath(func(p string) string { return shim.Prepend(shim.Remove(p, src), dst) })
}

// RemoveSystemShims runs as administrator when devstack is uninstalled: it
// takes the administrators-only copy off the system PATH and deletes it.
func RemoveSystemShims() error {
	dst := winproc.SystemShimDir()
	if dst == "" {
		return nil
	}
	err := winproc.SetSystemPath(func(p string) string { return shim.Remove(p, dst) })
	os.RemoveAll(filepath.Dir(dst))
	return err
}

// replaceFile copies src over dst, keeping src's modification time (which
// sameShims compares). A dst that is running (a php command in a terminal) is
// renamed out of the way first; Windows allows that, not overwriting it.
func replaceFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	tmp := dst + ".new"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chtimes(tmp, fi.ModTime(), fi.ModTime())
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	old := dst + ".old"
	os.Remove(old)
	if _, err := os.Stat(dst); err == nil {
		if err := os.Rename(dst, old); err != nil {
			os.Remove(tmp)
			return err
		}
	}
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}
	os.Remove(old) // fails while it runs; the next copy removes it
	return nil
}

// sameShims reports whether dir and the copy in sys hold the same shims.
func sameShims(dir, sys string) bool {
	for _, n := range shimNames {
		a, err1 := os.Stat(filepath.Join(dir, n))
		b, err2 := os.Stat(filepath.Join(sys, n))
		if err1 != nil || err2 != nil || a.Size() != b.Size() || !a.ModTime().Equal(b.ModTime()) {
			return false
		}
	}
	return true
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
