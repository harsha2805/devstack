//go:build windows

package winproc

import (
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// ShellOpen opens a URL, folder or file with its default handler, as a
// double-click in Explorer would. Callers must validate target first.
func ShellOpen(target string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	file, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}

// MessageBox shows a modal error dialog (for failures before any UI exists).
func MessageBox(title, text string) {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(text)
	windows.MessageBox(0, m, t, windows.MB_OK|windows.MB_ICONERROR)
}

const runKey =`Software\Microsoft\Windows\CurrentVersion\Run`

// Autostart reports whether HKCU\...\Run has a value named name.
func Autostart(name string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(name)
	return err == nil
}

// SetAutostart adds or removes command under HKCU\...\Run, so it starts when
// the user signs in. No admin rights needed.
func SetAutostart(name, command string, on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		if err := k.DeleteValue(name); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	return k.SetStringValue(name, command)
}
