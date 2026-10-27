package update

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// uninstallKey is Inno Setup's "<AppId>_is1"; AppId is pinned in installer/devstack.iss.
const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\devstack_is1`

// Installed reports whether exe lives in the folder the installer registered.
// Portable copies are updated by downloading a new zip instead.
func Installed(exe string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, uninstallKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	loc, _, err := k.GetStringValue("InstallLocation")
	if err != nil || loc == "" {
		return false
	}
	clean := func(p string) string { return strings.TrimRight(filepath.Clean(p), `\`) }
	return strings.EqualFold(clean(loc), clean(filepath.Dir(exe)))
}

// Launch re-checks the installer's sha256 and starts it. The file is held open
// with writes and deletes denied from the check until Windows has started it,
// so nothing can swap it in between.
func Launch(i Info, path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(h), path)
	defer f.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(sum.Sum(nil)), i.SHA256) {
		return errIntegrity
	}
	cmd := exec.Command(path, InstallerArgs...)
	cmd.Dir = filepath.Dir(path)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
