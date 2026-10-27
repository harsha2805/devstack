// Package update checks devstack's release feed and installs a newer release
// in place.
//
// The feed (latest.json) is signed offline with an Ed25519 key (tools/release),
// and the app accepts only keys pinned in TrustedKeys. A compromised web server
// therefore can't point the app at a different installer: the installer's
// sha256 is trusted only once the signature is.
package update

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FeedURL is where released apps look for updates.
const FeedURL = "https://devstack.holostack.in/latest.json"

// TrustedKeys are the release signing keys (base64 Ed25519 public keys). Add a
// new key here and ship it before retiring an old one, so feeds signed with
// either stay valid.
var TrustedKeys = []string{
	"n40UCrPqQCdg6/uEsdNj83sUDJp7kFTj3EpIf1tpFsQ=", // 2026-10-26
}

// UserAgent identifies devstack to the update server.
var UserAgent = "devstack"

// Info is a release advertised by the feed.
type Info struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Notes   string `json:"notes,omitempty"`
	Page    string `json:"page,omitempty"`
}

type feed struct {
	Info
	Signature string `json:"signature,omitempty"`
}

var client = &http.Client{Timeout: 0} // the installer download is cancelled through ctx

// Payload is the exact bytes the signature covers: every field the app acts
// on, one per line, with free-text notes last so their line breaks can't
// shift another field.
func Payload(i Info) ([]byte, error) {
	for name, v := range map[string]string{"version": i.Version, "url": i.URL, "sha256": i.SHA256, "page": i.Page} {
		if strings.ContainsAny(v, "\r\n") {
			return nil, fmt.Errorf("%s must be a single line", name)
		}
	}
	return []byte("devstack update feed v1\n" +
		"version=" + i.Version + "\n" +
		"url=" + i.URL + "\n" +
		"sha256=" + i.SHA256 + "\n" +
		"page=" + i.Page + "\n" +
		"notes=" + i.Notes), nil
}

// Sign returns the feed JSON for i, signed with key. tools/release uses it.
func Sign(i Info, key ed25519.PrivateKey) ([]byte, error) {
	p, err := Payload(i)
	if err != nil {
		return nil, err
	}
	f := feed{Info: i, Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, p))}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// ErrUnsigned means the feed had no valid signature from a trusted key.
var ErrUnsigned = errors.New("the update information failed its signature check, so it was ignored")

// Parse verifies a feed against keys and checks its fields.
func Parse(b []byte, keys []string) (Info, error) {
	var f feed
	if err := json.Unmarshal(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), &f); err != nil {
		return Info{}, fmt.Errorf("the update information is malformed: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(f.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return Info{}, ErrUnsigned
	}
	p, err := Payload(f.Info)
	if err != nil {
		return Info{}, fmt.Errorf("the update information is malformed: %w", err)
	}
	trusted := false
	for _, k := range keys {
		pub, err := base64.StdEncoding.DecodeString(k)
		if err == nil && len(pub) == ed25519.PublicKeySize && ed25519.Verify(pub, p, sig) {
			trusted = true
			break
		}
	}
	if !trusted {
		return Info{}, ErrUnsigned
	}

	i := f.Info
	i.Notes = strings.TrimSpace(i.Notes)
	if _, ok := parseVersion(i.Version); !ok {
		return Info{}, fmt.Errorf("the update information is malformed: invalid version %q", i.Version)
	}
	if !httpsURL(i.URL) {
		return Info{}, errors.New("the update information is malformed: url must be https")
	}
	if i.Page != "" && !httpsURL(i.Page) {
		return Info{}, errors.New("the update information is malformed: page must be https")
	}
	if h, err := hex.DecodeString(i.SHA256); err != nil || len(h) != sha256.Size {
		return Info{}, errors.New("the update information is malformed: sha256 must be 64 hex characters")
	}
	return i, nil
}

// Release reports whether v is a release version (x.y.z), as opposed to a
// development build, which never updates.
func Release(v string) bool {
	_, ok := parseVersion(v)
	return ok
}

// Newer reports whether release a is newer than b.
func Newer(a, b string) bool {
	va, ok1 := parseVersion(a)
	vb, ok2 := parseVersion(b)
	if !ok1 || !ok2 {
		return false
	}
	for i := range va {
		if va[i] != vb[i] {
			return va[i] > vb[i]
		}
	}
	return false
}

func parseVersion(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

func httpsURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

// Check fetches the feed and returns the advertised release when it is newer
// than current, or nil when current is up to date.
func Check(ctx context.Context, current string) (*Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, FeedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("the update server didn't respond in time")
		}
		return nil, errors.New("couldn't reach the update server")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the update server answered %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, errors.New("couldn't read the update information")
	}
	i, err := Parse(b, TrustedKeys)
	if err != nil {
		return nil, err
	}
	if !Newer(i.Version, current) {
		return nil, nil
	}
	return &i, nil
}

// Download fetches the installer into dir, verifying its sha256 before it
// gets its final name. A failed or cancelled download leaves nothing behind.
func Download(ctx context.Context, i Info, dir string, progress func(done, total int64)) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(dir, "devstack_setup_"+i.Version+".exe")
	partial := target + ".partial"
	defer os.Remove(partial)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("the download failed: couldn't reach the update server")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the download failed: the server answered %s", resp.Status)
	}

	f, err := os.Create(partial)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	var done int64
	buf := make([]byte, 64<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
			if _, err := f.Write(buf[:n]); err != nil {
				f.Close()
				return "", err
			}
			done += int64(n)
			if progress != nil {
				progress(done, resp.ContentLength)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			return "", errors.New("the download was interrupted")
		}
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), i.SHA256) {
		return "", errIntegrity
	}
	if err := os.Rename(partial, target); err != nil {
		return "", err
	}
	return target, nil
}

var errIntegrity = errors.New("the downloaded installer failed its integrity check, so it wasn't run")

// InstallerArgs upgrade in place without questions: Setup closes devstack,
// keeps the user's choices, and reopens devstack when it's done.
var InstallerArgs = []string{"/SILENT", "/SP-", "/SUPPRESSMSGBOXES", "/NORESTART", "/CLOSEAPPLICATIONS"}
