//go:build windows

package winproc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Folder permissions. devstack installs into a folder the user picks, by
// default at a drive root (C:\devstack), and a folder made there inherits the
// drive's "Authenticated Users: Modify": every local account could read the
// MySQL root password and the API token, or swap devstack's programs. The
// stack folder gets a DACL of its own instead (ProtectFolder), and the copy of
// the shims on the system PATH lives in an administrators-only folder
// (SystemShimDir).

// Rights that let someone change what a folder or file holds or who may.
const writeRights = windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA | 0x40 /* FILE_DELETE_CHILD */ |
	windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER | windows.GENERIC_WRITE | windows.GENERIC_ALL

const (
	sidSystem         = "S-1-5-18"
	sidAdministrators = "S-1-5-32-544"
	sidTrustedInstall = "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
	inheritOnlyAce    = 0x08
	accessAllowedAce  = 0
)

// CurrentUserSID is the signed-in account's SID ("S-1-5-21-…").
func CurrentUserSID() (string, error) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	return u.User.Sid.String(), nil
}

// WritableByOthers reports an account, other than SYSTEM, Administrators,
// TrustedInstaller and the SIDs in allow, that may change path or its
// contents. who is "" when there is none.
func WritableByOthers(path string, allow ...string) (who string, err error) {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return "", err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return "", err
	}
	if dacl == nil {
		return "Everyone", nil // a NULL DACL grants everything to everyone
	}
	ok := map[string]bool{sidSystem: true, sidAdministrators: true, sidTrustedInstall: true}
	for _, s := range allow {
		ok[s] = true
	}
	for i := uint16(0); i < dacl.AceCount; i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(i), &ace); err != nil {
			return "", err
		}
		if ace.Header.AceType != accessAllowedAce || ace.Header.AceFlags&inheritOnlyAce != 0 || ace.Mask&writeRights == 0 {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if ok[sid.String()] {
			continue
		}
		if acct, dom, _, err := sid.LookupAccount(""); err == nil {
			if dom != "" {
				return dom + `\` + acct, nil
			}
			return acct, nil
		}
		return sid.String(), nil
	}
	return "", nil
}

// Protected reports whether dir already has the DACL ProtectFolder gives it:
// not inherited, and nobody but the user, SYSTEM and Administrators can write.
func Protected(dir string) bool {
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false
	}
	ctl, _, err := sd.Control()
	if err != nil || ctl&windows.SE_DACL_PROTECTED == 0 {
		return false
	}
	user, err := CurrentUserSID()
	if err != nil {
		return false
	}
	who, err := WritableByOthers(dir, user)
	return err == nil && who == ""
}

// ownedNames are the parts of a stack folder devstack makes and owns; files
// directly in it count too. ProtectFolder re-applies the folder's permissions
// to these. Everything else (www, folders the user keeps there) keeps what it
// has, because www can hold hundreds of thousands of files.
var ownedNames = []string{"bin", "data", "etc", "etc-custom", "logs", "shims", "tmp"}

// ProtectFolder gives dir a DACL of its own: full control for the signed-in
// user, SYSTEM and Administrators, and nothing inherited from the drive.
// Files and folders created in dir afterwards inherit it. Of what is already
// there, devstack's own folders and the files directly in dir take it on;
// www and anything else keep their permissions.
func ProtectFolder(dir string) error {
	user, err := CurrentUserSID()
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString(fmt.Sprintf("D:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;%s)", user))
	if err != nil {
		return err
	}
	// SetFileSecurity, not SetNamedSecurityInfo: the latter would walk every
	// file below dir, www included, to re-inherit.
	if err := setFileSecurity(dir, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, sd); err != nil {
		return fmt.Errorf("protecting %s: %w", dir, err)
	}
	// An empty, unprotected DACL on a child means "inherit only from dir";
	// SetNamedSecurityInfo carries that down its tree.
	empty, err := windows.SecurityDescriptorFromString("D:AI")
	if err != nil {
		return err
	}
	emptyACL, _, err := empty.DACL()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		owned := !e.IsDir()
		for _, n := range ownedNames {
			if e.IsDir() && strings.EqualFold(e.Name(), n) {
				owned = true
			}
		}
		p := filepath.Join(dir, e.Name())
		if !owned || reparsePoint(p) {
			continue
		}
		if err := windows.SetNamedSecurityInfo(p, windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION|windows.UNPROTECTED_DACL_SECURITY_INFORMATION, nil, nil, emptyACL, nil); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p, err))
		}
	}
	return errors.Join(errs...)
}

// ProtectAdminFolder gives dir a DACL where only SYSTEM and Administrators
// may write and other accounts may read and run. Needs administrator rights.
func ProtectAdminFolder(dir string) error {
	sd, err := windows.SecurityDescriptorFromString("D:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1200a9;;;BU)")
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}

// SystemShimDir is where the administrator-only copy of the shims goes when
// they must come first on the system PATH: %ProgramFiles%\devstack\shims.
func SystemShimDir() string {
	pf, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, 0)
	if err != nil || pf == "" {
		pf = os.Getenv("ProgramW6432")
	}
	if pf == "" {
		return ""
	}
	return filepath.Join(pf, "devstack", "shims")
}

var procSetFileSecurity = windows.NewLazySystemDLL("advapi32.dll").NewProc("SetFileSecurityW")

func setFileSecurity(path string, info windows.SECURITY_INFORMATION, sd *windows.SECURITY_DESCRIPTOR) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if r, _, e := procSetFileSecurity.Call(uintptr(unsafe.Pointer(p)), uintptr(info), uintptr(unsafe.Pointer(sd))); r == 0 {
		return e
	}
	return nil
}

func reparsePoint(path string) bool {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return true
	}
	a, err := windows.GetFileAttributes(p)
	return err != nil || a&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
