//go:build windows

package winproc

import (
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procCoCreateInstance    = windows.NewLazySystemDLL("ole32.dll").NewProc("CoCreateInstance")
	procGetForegroundWindow = windows.NewLazySystemDLL("user32.dll").NewProc("GetForegroundWindow")

	clsidFileOpenDialog = windows.GUID{Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE, Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidFileOpenDialog   = windows.GUID{Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768, Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
)

const (
	fosPickFolders     = 0x20
	fosForceFileSystem = 0x40
	sigdnFileSysPath   = 0x80058000
	hrCancelled        = 0x800704C7 // HRESULT_FROM_WIN32(ERROR_CANCELLED)
	sFalse             = syscall.Errno(1)
)

// Slots in the IFileOpenDialog and IShellItem vtables.
const (
	vRelease        = 2
	vShow           = 3
	vSetOptions     = 9
	vGetOptions     = 10
	vSetTitle       = 17
	vGetResult      = 20
	vGetDisplayName = 5
)

type comObj struct{ vtbl *[32]uintptr }

func (o *comObj) call(slot int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.vtbl[slot], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return r
}

func failed(hr uintptr) bool { return int32(hr) < 0 }

// PickFolder shows the Windows folder picker over the window in front (the
// app the user just clicked in) and returns the folder chosen, or "" when the
// user cancels.
func PickFolder(title string) (string, error) {
	type result struct {
		dir string
		err error
	}
	ch := make(chan result, 1)
	// COM needs a single-threaded apartment of its own. The goroutine never
	// unlocks its thread, so Go retires the thread and the apartment with it.
	go func() {
		runtime.LockOSThread()
		dir, err := pickFolder(title)
		ch <- result{dir, err}
	}()
	r := <-ch
	return r.dir, r.err
}

func pickFolder(title string) (string, error) {
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err != nil && err != sFalse {
		return "", err
	}
	defer windows.CoUninitialize()

	var dlg *comObj
	hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidFileOpenDialog)), 0,
		windows.CLSCTX_INPROC_SERVER, uintptr(unsafe.Pointer(&iidFileOpenDialog)), uintptr(unsafe.Pointer(&dlg)))
	if failed(hr) {
		return "", syscall.Errno(hr)
	}
	defer dlg.call(vRelease)

	var opts uint32
	dlg.call(vGetOptions, uintptr(unsafe.Pointer(&opts)))
	dlg.call(vSetOptions, uintptr(opts|fosPickFolders|fosForceFileSystem))
	if t, err := windows.UTF16PtrFromString(title); err == nil {
		dlg.call(vSetTitle, uintptr(unsafe.Pointer(t)))
	}
	owner, _, _ := procGetForegroundWindow.Call()
	if hr := dlg.call(vShow, owner); hr == hrCancelled {
		return "", nil
	} else if failed(hr) {
		return "", syscall.Errno(hr)
	}

	var item *comObj
	if hr := dlg.call(vGetResult, uintptr(unsafe.Pointer(&item))); failed(hr) {
		return "", syscall.Errno(hr)
	}
	defer item.call(vRelease)
	var name *uint16
	if hr := item.call(vGetDisplayName, sigdnFileSysPath, uintptr(unsafe.Pointer(&name))); failed(hr) {
		return "", syscall.Errno(hr)
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(name))
	return windows.UTF16PtrToString(name), nil
}

// NetworkDrive reports whether path is on a mapped network drive (Z:\...).
func NetworkDrive(path string) bool {
	vol := filepath.VolumeName(path)
	if len(vol) != 2 || vol[1] != ':' {
		return false
	}
	p, err := windows.UTF16PtrFromString(vol + `\`)
	return err == nil && windows.GetDriveType(p) == windows.DRIVE_REMOTE
}
