//go:build windows

package winauth

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestHelloFactory checks the WinRT class and interop IID resolve, without
// showing any prompt.
func TestHelloFactory(t *testing.T) {
	for _, p := range []interface{ Find() error }{procRoGetActivationFactory, procWindowsCreateString, procCredUIPrompt, procCredUnPack, procLogonUser, procCoTaskMemFree} {
		if err := p.Find(); err != nil {
			t.Fatal(err)
		}
	}
	procRoInitialize.Call(1)
	class, err := hstring("Windows.Security.Credentials.UI.UserConsentVerifier")
	if err != nil {
		t.Fatal(err)
	}
	defer procWindowsDeleteString.Call(class)
	var factory *comObject
	hr, _, _ := procRoGetActivationFactory.Call(class, uintptr(unsafe.Pointer(&iidUserConsentVerifierInterop)), uintptr(unsafe.Pointer(&factory)))
	if hr != 0 || factory == nil {
		t.Fatalf("RoGetActivationFactory(interop) = 0x%x", hr)
	}
	factory.release()
}

func TestSameAccount(t *testing.T) {
	me := windows.GetCurrentProcessToken()
	if !sameAccount(me, me) {
		t.Error("the signed-in user's token doesn't match itself")
	}
	if sameAccount(me, windows.Token(0)) {
		t.Error("an invalid token matches")
	}
}
