//go:build windows

package winproc

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// A folder as one made at a drive root gets it: Authenticated Users may modify.
func driveRootLike(t *testing.T, dir string) {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString("D:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1301bf;;;AU)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, _ := sd.DACL()
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
}

func TestProtectFolder(t *testing.T) {
	root := t.TempDir()
	driveRootLike(t, root)
	for _, d := range []string{"data/devstack", "www/site", "notes"} {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}
	os.WriteFile(filepath.Join(root, "data", "devstack", "secrets.toml"), []byte("x"), 0o600)
	os.WriteFile(filepath.Join(root, "stack.toml"), []byte("x"), 0o644)

	if who, err := WritableByOthers(filepath.Join(root, "data", "devstack", "secrets.toml")); err != nil || who == "" {
		t.Fatalf("before: secrets.toml writable by others = %q, %v; want Authenticated Users", who, err)
	}
	if Protected(root) {
		t.Fatal("before: Protected")
	}
	if err := ProtectFolder(root); err != nil {
		t.Fatal(err)
	}
	user, _ := CurrentUserSID()
	for _, p := range []string{"", "data", "data/devstack/secrets.toml", "stack.toml"} {
		if who, err := WritableByOthers(filepath.Join(root, p), user); err != nil || who != "" {
			t.Errorf("after: %s writable by %q, %v", p, who, err)
		}
	}
	if !Protected(root) {
		t.Error("after: not Protected")
	}
	// www and folders devstack doesn't own keep what they had.
	for _, p := range []string{"www", "notes"} {
		if who, _ := WritableByOthers(filepath.Join(root, p), user); who == "" {
			t.Errorf("%s was changed; it should keep its permissions", p)
		}
	}
	// What is created afterwards inherits the protected folder's DACL.
	os.MkdirAll(filepath.Join(root, "www2"), 0o755)
	if who, _ := WritableByOthers(filepath.Join(root, "www2"), user); who != "" {
		t.Errorf("new folder writable by %q", who)
	}
}
