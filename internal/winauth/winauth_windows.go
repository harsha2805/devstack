// Package winauth asks the signed-in Windows user to prove it's them, the way
// browsers do before showing a saved password: Windows Hello (PIN, face,
// fingerprint) when it's set up, else the account's Windows password.
package winauth

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ErrCanceled means the user closed the prompt.
var ErrCanceled = errors.New("verification canceled")

var (
	combase                       = windows.NewLazySystemDLL("combase.dll")
	procRoInitialize              = combase.NewProc("RoInitialize")
	procRoGetActivationFactory    = combase.NewProc("RoGetActivationFactory")
	procWindowsCreateString       = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString       = combase.NewProc("WindowsDeleteString")
	credui                        = windows.NewLazySystemDLL("credui.dll")
	procCredUIPrompt              = credui.NewProc("CredUIPromptForWindowsCredentialsW")
	procCredUnPack                = credui.NewProc("CredUnPackAuthenticationBufferW")
	ole32                         = windows.NewLazySystemDLL("ole32.dll")
	procCoTaskMemFree             = ole32.NewProc("CoTaskMemFree")
	advapi32                      = windows.NewLazySystemDLL("advapi32.dll")
	procLogonUser                 = advapi32.NewProc("LogonUserW")
	user32                        = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWindow       = user32.NewProc("GetForegroundWindow")
	iidUserConsentVerifierInterop = windows.GUID{Data1: 0x39E050C3, Data2: 0x4E74, Data3: 0x441A, Data4: [8]byte{0x8D, 0xC0, 0xB8, 0x11, 0x04, 0xDF, 0x94, 0x9C}}
	// IAsyncOperation<UserConsentVerificationResult>, the WinRT parameterized
	// IID of pinterface({9fc2b0bb-...};enum(...UserConsentVerificationResult;i4)).
	iidAsyncOpResult = windows.GUID{Data1: 0xFD596FFD, Data2: 0x2318, Data3: 0x558F, Data4: [8]byte{0x9D, 0xBE, 0xD2, 0x1D, 0xF4, 0x37, 0x64, 0xA5}}
	iidAsyncInfo     = windows.GUID{Data1: 0x00000036, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
)

// Verify asks the user to confirm with Windows Hello, or with their Windows
// password when Hello isn't set up. owner is the window the prompt belongs to
// (0 = the foreground window). It returns nil once the user is verified.
func Verify(owner uintptr, message string) error {
	if owner == 0 {
		owner, _, _ = procGetForegroundWindow.Call()
	}
	// WinRT and the credential UI want one thread for the whole exchange.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ok, err := hello(owner, message)
	if err == nil {
		if ok {
			return nil
		}
		return ErrCanceled
	}
	if !errors.Is(err, errNoHello) {
		return err
	}
	return password(owner, message)
}

var errNoHello = errors.New("windows hello is not available")

// UserConsentVerificationResult values.
const (
	verified             = 0
	deviceNotPresent     = 1
	notConfiguredForUser = 2
	disabledByPolicy     = 3
	deviceBusy           = 4
	retriesExhausted     = 5
	canceled             = 6
)

// hello runs UserConsentVerifier through IUserConsentVerifierInterop, which
// parents the dialog to owner. It returns errNoHello when Hello isn't set up.
func hello(owner uintptr, message string) (bool, error) {
	procRoInitialize.Call(1) // RO_INIT_MULTITHREADED; already-initialized is fine

	class, err := hstring("Windows.Security.Credentials.UI.UserConsentVerifier")
	if err != nil {
		return false, errNoHello
	}
	defer procWindowsDeleteString.Call(class)
	var factory *comObject
	if hr, _, _ := procRoGetActivationFactory.Call(class, uintptr(unsafe.Pointer(&iidUserConsentVerifierInterop)), uintptr(unsafe.Pointer(&factory))); hr != 0 || factory == nil {
		return false, errNoHello
	}
	defer factory.release()

	msg, err := hstring(message)
	if err != nil {
		return false, err
	}
	defer procWindowsDeleteString.Call(msg)
	var op *comObject
	// IUserConsentVerifierInterop::RequestVerificationForWindowAsync is slot 6, after IInspectable.
	if hr := factory.call(6, owner, msg, uintptr(unsafe.Pointer(&iidAsyncOpResult)), uintptr(unsafe.Pointer(&op))); hr != 0 || op == nil {
		return false, errNoHello
	}
	defer op.release()

	var info *comObject
	if hr := op.call(0, uintptr(unsafe.Pointer(&iidAsyncInfo)), uintptr(unsafe.Pointer(&info))); hr != 0 || info == nil {
		return false, fmt.Errorf("windows hello: no IAsyncInfo (0x%x)", hr)
	}
	defer info.release()
	for {
		var status int32
		info.call(7, uintptr(unsafe.Pointer(&status))) // IAsyncInfo::get_Status
		if status != 0 {                               // 0 = Started
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	var result int32
	if hr := op.call(8, uintptr(unsafe.Pointer(&result))); hr != 0 { // IAsyncOperation::GetResults
		return false, errNoHello
	}
	switch result {
	case verified:
		return true, nil
	case deviceNotPresent, notConfiguredForUser, disabledByPolicy:
		return false, errNoHello
	case deviceBusy:
		return false, errors.New("Windows Hello is busy; try again")
	case retriesExhausted:
		return false, errors.New("too many attempts; Windows Hello is locked for now")
	}
	return false, nil // canceled
}

// password asks for the current user's Windows password in the standard
// credential prompt and checks it with LogonUser. Wrong passwords ask again,
// and so does another account's valid password: only the signed-in user passes.
func password(owner uintptr, message string) error {
	caption, _ := windows.UTF16PtrFromString("devstack")
	text, _ := windows.UTF16PtrFromString(message)
	info := struct {
		size    uint32
		parent  uintptr
		message *uint16
		caption *uint16
		banner  uintptr
	}{parent: owner, message: text, caption: caption}
	info.size = uint32(unsafe.Sizeof(info))

	const (
		enumerateCurrentUser = 0x200 // CREDUIWIN_ENUMERATE_CURRENT_USER
		errorCancelled       = 1223
		errorLogonFailure    = 1326
	)
	authError := uintptr(0)
	for attempt := 0; attempt < 5; attempt++ {
		var pkg uint32
		var out *byte
		var outSize uint32
		var save int32
		r, _, _ := procCredUIPrompt.Call(uintptr(unsafe.Pointer(&info)), authError, uintptr(unsafe.Pointer(&pkg)),
			0, 0, uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&outSize)), uintptr(unsafe.Pointer(&save)), enumerateCurrentUser)
		if r == errorCancelled {
			return ErrCanceled
		}
		if r != 0 {
			return fmt.Errorf("Windows sign-in prompt failed: %w", syscall.Errno(r))
		}
		user, domain, pass, err := unpack(uintptr(unsafe.Pointer(out)), outSize)
		if out != nil {
			clear(unsafe.Slice(out, outSize)) // the packed credential holds the password
		}
		procCoTaskMemFree.Call(uintptr(unsafe.Pointer(out)))
		if err != nil {
			return err
		}
		ok := logon(user, domain, pass)
		for i := range pass {
			pass[i] = 0
		}
		if ok {
			return nil
		}
		authError = errorLogonFailure // the prompt says the password was wrong
	}
	return errors.New("too many wrong passwords")
}

func unpack(buf uintptr, size uint32) (user, domain string, pass []uint16, err error) {
	u := make([]uint16, 514)
	d := make([]uint16, 338)
	p := make([]uint16, 514)
	ul, dl, pl := uint32(len(u)), uint32(len(d)), uint32(len(p))
	r, _, e := procCredUnPack.Call(0, buf, uintptr(size), uintptr(unsafe.Pointer(&u[0])), uintptr(unsafe.Pointer(&ul)),
		uintptr(unsafe.Pointer(&d[0])), uintptr(unsafe.Pointer(&dl)), uintptr(unsafe.Pointer(&p[0])), uintptr(unsafe.Pointer(&pl)))
	if r == 0 {
		return "", "", nil, fmt.Errorf("read the Windows sign-in: %w", e)
	}
	user, domain = windows.UTF16ToString(u), windows.UTF16ToString(d)
	// "DOMAIN\user" comes back in the user field for local and Azure AD accounts.
	if domain == "" {
		if i := strings.IndexByte(user, '\\'); i >= 0 {
			domain, user = user[:i], user[i+1:]
		}
	}
	return user, domain, p[:pl], nil
}

func logon(user, domain string, pass []uint16) bool {
	u, _ := windows.UTF16PtrFromString(user)
	var d *uint16
	if domain != "" {
		d, _ = windows.UTF16PtrFromString(domain)
	}
	if len(pass) == 0 || pass[len(pass)-1] != 0 {
		pass = append(pass, 0)
	}
	var token windows.Token
	const interactive, defaultProvider = 2, 0
	r, _, _ := procLogonUser.Call(uintptr(unsafe.Pointer(u)), uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(&pass[0])),
		interactive, defaultProvider, uintptr(unsafe.Pointer(&token)))
	if r == 0 {
		return false
	}
	defer token.Close()
	return sameAccount(token, windows.GetCurrentProcessToken())
}

// sameAccount reports whether two tokens belong to the same Windows account.
// Compared by SID, since one account has many names (DOMAIN\user, .\user,
// MicrosoftAccount\name@example.com).
func sameAccount(a, b windows.Token) bool {
	ua, err := a.GetTokenUser()
	if err != nil {
		return false
	}
	ub, err := b.GetTokenUser()
	if err != nil {
		return false
	}
	return windows.EqualSid(ua.User.Sid, ub.User.Sid)
}

func hstring(s string) (uintptr, error) {
	u, err := windows.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	var h uintptr
	if hr, _, _ := procWindowsCreateString.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&h))); hr != 0 {
		return 0, fmt.Errorf("WindowsCreateString: 0x%x", hr)
	}
	return h, nil
}

// comObject is a COM/WinRT interface pointer: its first word is the vtable.
type comObject struct{ vtbl *[16]uintptr }

func (o *comObject) call(slot int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.vtbl[slot], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return r
}

func (o *comObject) release() { o.call(2) }
