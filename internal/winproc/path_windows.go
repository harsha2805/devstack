//go:build windows

package winproc

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	userEnvKey   = `Environment`
	systemEnvKey = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
)

// EnvPaths returns the system and user PATH as new programs get them, with
// %VARIABLES% expanded.
func EnvPaths() (system, user string) {
	read := func(root registry.Key, key string) string {
		k, err := registry.OpenKey(root, key, registry.QUERY_VALUE)
		if err != nil {
			return ""
		}
		defer k.Close()
		v, _, err := k.GetStringValue("Path")
		if err != nil {
			return ""
		}
		if x, err := registry.ExpandString(v); err == nil {
			return x
		}
		return v
	}
	return read(registry.LOCAL_MACHINE, systemEnvKey), read(registry.CURRENT_USER, userEnvKey)
}

// SetUserPath rewrites the user PATH with change and tells running programs
// (Explorer, so new terminals) that the environment changed.
func SetUserPath(change func(string) string) error {
	return setPath(registry.CURRENT_USER, userEnvKey, change)
}

// SetSystemPath is SetUserPath for the system PATH; it needs administrator rights.
func SetSystemPath(change func(string) string) error {
	return setPath(registry.LOCAL_MACHINE, systemEnvKey, change)
}

func setPath(root registry.Key, key string, change func(string) string) error {
	k, err := registry.OpenKey(root, key, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	old, _, err := k.GetStringValue("Path") // unexpanded, so %VARIABLES% survive
	if err != nil && err != registry.ErrNotExist {
		return err
	}
	if err := k.SetExpandStringValue("Path", change(old)); err != nil {
		return err
	}
	broadcastEnvironment()
	return nil
}

var procSendMessageTimeout = windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")

// broadcastEnvironment sends WM_SETTINGCHANGE "Environment", as the System
// Properties dialog does, so Explorer passes the new PATH to what it starts.
func broadcastEnvironment() {
	const hwndBroadcast, wmSettingChange, smtoAbortIfHung = 0xffff, 0x001A, 0x0002
	env, _ := windows.UTF16PtrFromString("Environment")
	var result uintptr
	procSendMessageTimeout.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(env)), smtoAbortIfHung, 2000, uintptr(unsafe.Pointer(&result)))
}

// OpenTerminal opens PowerShell (7 when installed) in a new console window in
// dir with env, and runs startup there first. Windows 11 shows it in Windows
// Terminal when that is the default terminal app.
//
// It goes through `cmd /c start`: Go always hands a child standard handles
// (NUL when unset), and PowerShell reading end-of-input from NUL quits even
// with -NoExit. start gives it the new console's own handles.
func OpenTerminal(dir string, env []string, title, startup string) error {
	shell := "powershell.exe"
	if p, err := exec.LookPath("pwsh.exe"); err == nil {
		shell = p
	}
	title = strings.NewReplacer(`"`, "", "%", "", "^", "", "&", "", "|", "", "<", "", ">", "").Replace(title)
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       fmt.Sprintf(`cmd.exe /d /c start "%s" "%s" -NoLogo -NoExit -EncodedCommand %s`, title, shell, encodePS(startup)),
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	cmd.Dir, cmd.Env = dir, env
	return cmd.Run()
}

// encodePS is a script as -EncodedCommand takes it: base64 of UTF-16LE.
func encodePS(script string) string {
	u := windows.StringToUTF16(script)
	u = u[:len(u)-1] // drop the NUL
	b := make([]byte, 0, len(u)*2)
	for _, c := range u {
		b = append(b, byte(c), byte(c>>8))
	}
	return base64.StdEncoding.EncodeToString(b)
}

// Environ is os.Environ with PATH replaced by path and the given variables
// removed: what a program the user starts fresh would see.
func Environ(path string, drop ...string) []string {
	var out []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if strings.EqualFold(k, "Path") || containsFold(drop, k) {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "Path="+path)
}

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}
