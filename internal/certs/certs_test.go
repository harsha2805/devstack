package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func newCA(t *testing.T) *CA {
	t.Helper()
	t.Setenv("DEVSTACK_APPDATA", t.TempDir())
	ca, err := LoadOrCreate("test")
	if err != nil {
		t.Fatal(err)
	}
	return ca
}

func site(dir, slug string, names ...string) Site {
	return Site{Name: slug, DNSNames: names,
		CertFile: filepath.Join(dir, slug+".crt"), KeyFile: filepath.Join(dir, slug+".key")}
}

func verify(ca *CA, cert *x509.Certificate, host string) error {
	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	_, err := cert.Verify(x509.VerifyOptions{Roots: pool, DNSName: host})
	return err
}

func TestIssueAndVerify(t *testing.T) {
	ca := newCA(t)
	dir := t.TempDir()
	s := site(dir, "my-app", "my-app.test", "*.my-app.test", "my-app.localhost", "*.my-app.localhost", "my-app.example.com")
	issued, err := ca.Ensure(dir, testCRL, []Site{s})
	if err != nil || !slices.Equal(issued, []string{"my-app"}) {
		t.Fatalf("issued %v, %v", issued, err)
	}
	pair, err := tls.LoadX509KeyPair(s.CertFile, s.KeyFile)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(pair.Certificate[0])
	if slices.Contains(leaf.DNSNames, "my-app.example.com") {
		t.Error("a name outside the CA's constraints made it into the certificate")
	}
	for _, host := range []string{"my-app.test", "api.my-app.test", "my-app.localhost"} {
		if err := verify(ca, leaf, host); err != nil {
			t.Errorf("%s: %v", host, err)
		}
	}
	if leaf.NotAfter.Sub(leaf.NotBefore) > 398*24*time.Hour {
		t.Error("leaf lives longer than browsers accept")
	}

	// Unchanged: nothing reissued. Names changed: reissued.
	if issued, _ := ca.Ensure(dir, testCRL, []Site{s}); len(issued) != 0 {
		t.Errorf("reissued an unchanged certificate: %v", issued)
	}
	s.DNSNames = append(s.DNSNames, "extra.my-app.test")
	if issued, _ := ca.Ensure(dir, testCRL, []Site{s}); len(issued) != 1 {
		t.Errorf("didn't reissue after a name change: %v", issued)
	}

	// A site that is gone loses its files.
	ca.Ensure(dir, testCRL, []Site{site(dir, "other", "other.test")})
	if _, err := os.Stat(s.CertFile); !os.IsNotExist(err) {
		t.Error("old site's certificate left behind")
	}
}

// The point of the name constraints: even with the CA key, a certificate for
// a real site doesn't verify.
func TestConstraintsBlockRealDomains(t *testing.T) {
	ca := newCA(t)
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: serial(), Subject: pkix.Name{CommonName: "www.google.com"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames: []string{"www.google.com"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.Cert, key.Public(), ca.Key)
	if err != nil {
		t.Fatal(err)
	}
	evil, _ := x509.ParseCertificate(der)
	err = verify(ca, evil, "www.google.com")
	if err == nil || !strings.Contains(err.Error(), "constraint") {
		t.Fatalf("a google.com certificate verified (err = %v)", err)
	}
}

func TestCAReuseAndTLD(t *testing.T) {
	ca := newCA(t)
	again, err := LoadOrCreate("test")
	if err != nil || !again.Cert.Equal(ca.Cert) {
		t.Fatalf("CA not reused: %v", err)
	}
	if _, err := LoadOrCreate("dev"); err == nil {
		t.Error("a CA made for .test claimed to cover .dev")
	}
	if !ca.Cert.PermittedDNSDomainsCritical || ca.Cert.MaxPathLen != 0 || !ca.Cert.MaxPathLenZero {
		t.Error("CA is missing its limits")
	}
}

const testCRL = "http://localhost/.well-known/devstack-ca.crl"

func TestCRL(t *testing.T) {
	ca := newCA(t)
	dir := t.TempDir()
	s := site(dir, "a", "a.test")
	if _, err := ca.Ensure(dir, testCRL, []Site{s}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(ca.CRLPath())
	if err != nil {
		t.Fatal(err)
	}
	l, err := x509.ParseRevocationList(b)
	if err != nil || l.CheckSignatureFrom(ca.Cert) != nil || len(l.RevokedCertificateEntries) != 0 {
		t.Fatalf("bad CRL: %v", err)
	}
	pair, _ := tls.LoadX509KeyPair(s.CertFile, s.KeyFile)
	leaf, _ := x509.ParseCertificate(pair.Certificate[0])
	if !slices.Equal(leaf.CRLDistributionPoints, []string{testCRL}) {
		t.Fatalf("CDP = %v", leaf.CRLDistributionPoints)
	}
	// A new CRL URL (the HTTP port changed) reissues.
	if issued, _ := ca.Ensure(dir, "http://localhost:8080/.well-known/devstack-ca.crl", []Site{s}); len(issued) != 1 {
		t.Error("didn't reissue after the CRL URL changed")
	}
}
