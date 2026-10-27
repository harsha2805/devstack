package certs

import (
	"bytes"
	"crypto/x509"
	"errors"
	"slices"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ErrDeclined means the user answered No when Windows asked to trust the CA.
var ErrDeclined = errors.New("Windows asked to trust the devstack certificate and the answer was No, so nothing changed")

// eachRoot calls fn for every certificate in the current user's Root store.
// fn returns true to stop.
func eachRoot(fn func(ctx *windows.CertContext, cert *x509.Certificate) bool) error {
	name, _ := windows.UTF16PtrFromString("ROOT")
	store, err := windows.CertOpenSystemStore(0, name)
	if err != nil {
		return err
	}
	defer windows.CertCloseStore(store, 0)
	var ctx *windows.CertContext
	for {
		ctx, err = windows.CertEnumCertificatesInStore(store, ctx)
		if ctx == nil {
			return nil // end of store (err is CRYPT_E_NOT_FOUND)
		}
		raw := unsafe.Slice(ctx.EncodedCert, ctx.Length)
		cert, perr := x509.ParseCertificate(bytes.Clone(raw))
		if perr != nil {
			continue
		}
		if fn(ctx, cert) {
			windows.CertFreeCertificateContext(ctx)
			return nil
		}
	}
}

// Trusted reports whether this CA is in the user's Root store.
func (ca *CA) Trusted() bool {
	found := false
	eachRoot(func(_ *windows.CertContext, c *x509.Certificate) bool {
		found = bytes.Equal(c.Raw, ca.Cert.Raw)
		return found
	})
	return found
}

// Trust adds the CA to the user's Root store. Windows shows its own
// confirmation; no administrator rights are needed.
func (ca *CA) Trust() error {
	name, _ := windows.UTF16PtrFromString("ROOT")
	store, err := windows.CertOpenSystemStore(0, name)
	if err != nil {
		return err
	}
	defer windows.CertCloseStore(store, 0)
	raw := ca.Cert.Raw
	ctx, err := windows.CertCreateCertificateContext(windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING, &raw[0], uint32(len(raw)))
	if err != nil {
		return err
	}
	defer windows.CertFreeCertificateContext(ctx)
	if err := windows.CertAddCertificateContextToStore(store, ctx, windows.CERT_STORE_ADD_REPLACE_EXISTING, nil); err != nil {
		if declined(err) {
			return ErrDeclined
		}
		return err
	}
	return nil
}

// Untrust removes every devstack CA from the user's Root store (Windows asks
// to confirm each). It returns how many were removed, checked by looking
// again afterwards rather than trusting the delete calls.
func Untrust() (int, error) {
	before := countDevstack()
	if before == 0 {
		return 0, nil
	}
	name, _ := windows.UTF16PtrFromString("ROOT")
	store, err := windows.CertOpenSystemStore(0, name)
	if err != nil {
		return 0, err
	}
	// Delete while the store is open: a context deleted after its store was
	// closed only changes a stale copy.
	var errs []error
	var ctx *windows.CertContext
	for {
		ctx, _ = windows.CertEnumCertificatesInStore(store, ctx)
		if ctx == nil {
			break
		}
		cert, perr := x509.ParseCertificate(bytes.Clone(unsafe.Slice(ctx.EncodedCert, ctx.Length)))
		if perr != nil || !cert.IsCA || !slices.Contains(cert.Subject.Organization, Org) {
			continue
		}
		// Delete frees the context it is given, so hand it a duplicate and
		// keep ctx for the enumeration.
		if err := windows.CertDeleteCertificateFromStore(windows.CertDuplicateCertificateContext(ctx)); err != nil {
			if declined(err) {
				err = ErrDeclined
			}
			errs = append(errs, err)
		}
	}
	windows.CertCloseStore(store, 0)

	after := countDevstack()
	if after > 0 && len(errs) == 0 {
		errs = append(errs, errors.New("Windows kept the devstack certificate in the Root store"))
	}
	return before - after, errors.Join(errs...)
}

func countDevstack() int {
	n := 0
	eachRoot(func(_ *windows.CertContext, c *x509.Certificate) bool {
		if c.IsCA && slices.Contains(c.Subject.Organization, Org) {
			n++
		}
		return false
	})
	return n
}

func declined(err error) bool {
	var errno windows.Errno
	if errors.As(err, &errno) {
		return errno == windows.ERROR_CANCELLED || uint32(errno) == 0x800704C7
	}
	return false
}
