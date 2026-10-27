// Package certs gives local sites trusted HTTPS.
//
// devstack keeps one certificate authority per Windows user in
// %LOCALAPPDATA%\devstack\ca and adds it to the user's Root store (Windows
// asks the user once; no administrator rights). The CA carries name
// constraints: it can only sign localhost names, the stack's TLD and
// loopback addresses, so even a stolen CA key can't mint certificates for
// real websites. Each site gets its own certificate in
// <root>/data/devstack/certs, reissued when its names change or it nears
// expiry.
package certs

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/harsha2805/devstack/internal/appconf"
)

const (
	// Org marks devstack's CA in the Root store, so it can be found and removed.
	Org = "devstack"

	caFile  = "ca.crt"
	keyFile = "ca.key"
	crlFile = "ca.crl"

	caLife   = 10 * 365 * 24 * time.Hour
	leafLife = 397 * 24 * time.Hour // the longest browsers accept
	crlLife  = 400 * 24 * time.Hour // outlives every leaf issued alongside it
	renewAt  = 30 * 24 * time.Hour  // reissue when less than this is left
)

// CA is devstack's local certificate authority.
type CA struct {
	Cert *x509.Certificate
	Key  crypto.Signer
	Path string // ca.crt, for tools that need it (PHP's curl.cainfo, NODE_EXTRA_CA_CERTS)
}

// Dir is where the CA lives.
func Dir() (string, error) {
	d, err := appconf.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "ca"), nil
}

// ErrNoCA means no CA has been made yet (HTTPS was never turned on).
var ErrNoCA = errors.New("no devstack CA yet; turn HTTPS on first")

// Load reads the CA from Dir.
func Load() (*CA, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	certPEM, err := os.ReadFile(filepath.Join(dir, caFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoCA
	}
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(filepath.Join(dir, keyFile))
	if err != nil {
		return nil, err
	}
	cert, err := parseCert(certPEM)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(keyPEM)
	if err != nil {
		return nil, err
	}
	return &CA{Cert: cert, Key: key, Path: filepath.Join(dir, caFile)}, nil
}

// Covers reports whether the CA may sign names under tld.
func (ca *CA) Covers(tld string) bool {
	return slices.Contains(ca.Cert.PermittedDNSDomains, tld)
}

// LoadOrCreate returns the CA, making one that covers tld if there is none.
// An existing CA made for a different TLD is an error: replacing it means
// trusting a new one, which the caller must ask for.
func LoadOrCreate(tld string) (*CA, error) {
	ca, err := Load()
	if err == nil {
		if !ca.Covers(tld) {
			return nil, fmt.Errorf("the devstack CA was made for other names and can't sign *.%s; turn HTTPS off and on again to replace it", tld)
		}
		return ca, nil
	}
	if !errors.Is(err, ErrNoCA) {
		return nil, err
	}
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	return create(dir, tld)
}

func create(dir, tld string) (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	pubDER, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return nil, err
	}
	ski := sha1.Sum(pubDER)
	host, _ := os.Hostname()
	user := os.Getenv("USERNAME")
	tmpl := &x509.Certificate{
		SerialNumber: serial(),
		Subject: pkix.Name{
			Organization:       []string{Org},
			OrganizationalUnit: []string{user + "@" + host},
			CommonName:         "devstack local CA (" + user + "@" + host + ")",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(caLife),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
		SubjectKeyId:          ski[:],
		// The CA can only vouch for local names and loopback addresses.
		PermittedDNSDomainsCritical: true,
		PermittedDNSDomains:         []string{"localhost", tld},
		PermittedIPRanges: []*net.IPNet{
			{IP: net.IPv4(127, 0, 0, 0).To4(), Mask: net.CIDRMask(8, 32)},
			{IP: net.IPv6loopback, Mask: net.CIDRMask(128, 128)},
		},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, keyFile), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, caFile), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		return nil, err
	}
	return &CA{Cert: cert, Key: key, Path: filepath.Join(dir, caFile)}, nil
}

// Delete removes the CA files (after it was removed from the Root store).
func Delete() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// Site is one certificate to keep current.
type Site struct {
	Name     string   // file name stem, the site slug
	DNSNames []string // names to cover; ones the CA may not sign are dropped
	CertFile string
	KeyFile  string
}

// Allowed filters names to those the CA may sign (a name the constraints
// forbid would make browsers reject the whole certificate).
func (ca *CA) Allowed(names []string) []string {
	var out []string
	for _, n := range names {
		bare := strings.TrimPrefix(strings.ToLower(n), "*.")
		for _, d := range ca.Cert.PermittedDNSDomains {
			if bare == d || strings.HasSuffix(bare, "."+d) {
				out = append(out, strings.ToLower(n))
				break
			}
		}
	}
	return out
}

// Ensure issues or renews each site's certificate as needed, and removes
// certificates in dir that no site uses. crl is the http URL where the web
// server publishes CRLPath. It returns the names it (re)issued.
func (ca *CA) Ensure(dir, crl string, sites []Site) ([]string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := ca.ensureCRL(); err != nil {
		return nil, err
	}
	keep := map[string]bool{}
	var issued []string
	var errs []error
	for _, s := range sites {
		keep[filepath.Base(s.CertFile)] = true
		keep[filepath.Base(s.KeyFile)] = true
		names := ca.Allowed(s.DNSNames)
		if ca.current(s.CertFile, names, crl) {
			continue
		}
		if err := ca.issue(s, names, crl); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
			continue
		}
		issued = append(issued, s.Name)
	}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && !keep[e.Name()] {
				os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}
	return issued, errors.Join(errs...)
}

// CRLPath is the CA's (empty) revocation list. Windows' Schannel (curl.exe,
// some .NET settings) refuses a certificate whose revocation it can't check,
// and it won't read file:// URLs, so the web server publishes this file on
// localhost and every certificate points there.
func (ca *CA) CRLPath() string { return filepath.Join(filepath.Dir(ca.Path), crlFile) }

// ensureCRL keeps CRLPath signed and current.
func (ca *CA) ensureCRL() error {
	path := ca.CRLPath()
	if b, err := os.ReadFile(path); err == nil {
		if l, err := x509.ParseRevocationList(b); err == nil && l.CheckSignatureFrom(ca.Cert) == nil && time.Until(l.NextUpdate) > renewAt {
			return nil
		}
	}
	der, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		Number:     big.NewInt(time.Now().Unix()),
		ThisUpdate: time.Now().Add(-time.Hour),
		NextUpdate: time.Now().Add(crlLife),
	}, ca.Cert, ca.Key)
	if err != nil {
		return err
	}
	return os.WriteFile(path, der, 0o644)
}

// current reports whether the certificate at path was signed by this CA,
// covers exactly names, points at crl, and has time left.
func (ca *CA) current(path string, names []string, crl string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	cert, err := parseCert(b)
	if err != nil || !bytes.Equal(cert.AuthorityKeyId, ca.Cert.SubjectKeyId) {
		return false
	}
	if time.Until(cert.NotAfter) < renewAt || !slices.Equal(cert.CRLDistributionPoints, []string{crl}) {
		return false
	}
	have := slices.Clone(cert.DNSNames)
	want := slices.Clone(names)
	slices.Sort(have)
	slices.Sort(want)
	return slices.Equal(have, want)
}

func (ca *CA) issue(s Site, names []string, crl string) error {
	if len(names) == 0 {
		return errors.New("no names the devstack CA may sign")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{Organization: []string{Org}, CommonName: names[0]},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(leafLife),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              names,
		CRLDistributionPoints: []string{crl},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.Cert, key.Public(), ca.Key)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.KeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return err
	}
	return os.WriteFile(s.CertFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
}

func serial() *big.Int {
	n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	return n
}

func parseCert(b []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(b)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("not a PEM certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parseKey(b []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("not a PEM key")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	signer, ok := k.(crypto.Signer)
	if !ok {
		return nil, errors.New("CA key can't sign")
	}
	return signer, nil
}
