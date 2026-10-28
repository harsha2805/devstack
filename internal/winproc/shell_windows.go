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

var procAllowSetForeground = windows.NewLazySystemDLL("user32.dll").NewProc("AllowSetForegroundWindow")

// AllowForeground lets process pid bring its window to the front. Windows only
// lets the program the user just started take focus, so a second launch hands
// that right to the running app before asking it to show its window.
// pid 0 allows any process.
func AllowForeground(pid int) {
	if pid == 0 {
		pid = -1 // ASFW_ANY
	}
	procAllowSetForeground.Call(uintptr(uint32(int32(pid))))
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

// ODBCSQLServerDrivers lists the installed "ODBC Driver N for SQL Server"
// names, newest first.
func ODBCSQLServerDrivers() []string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\ODBC\ODBCINST.INI\ODBC Drivers`, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	names, err := k.ReadValueNames(0)
	if err != nil {
		return nil
	}
	var out []string
	for _, n := range names {
		if v, _, _ := k.GetStringValue(n); v == "Installed" && sqlServerODBC.MatchString(n) {
			out = append(out, n)
		}
	}
	sortODBC(out)
	return out
}
