package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func testKey(t *testing.T) (ed25519.PrivateKey, []string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return priv, []string{base64.StdEncoding.EncodeToString(pub)}
}

var sample = Info{
	Version: "0.2.0",
	URL:     "https://devstack.holostack.in/devstack_setup_0.2.0.exe",
	SHA256:  strings.Repeat("ab", 32),
	Notes:   "Folder picker.\nLive theme switch.",
	Page:    "https://devstack.holostack.in",
}

func TestSignParse(t *testing.T) {
	priv, keys := testKey(t)
	b, err := Sign(sample, priv)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(b, keys)
	if err != nil {
		t.Fatal(err)
	}
	if got != sample {
		t.Fatalf("got %+v", got)
	}
	// PowerShell 5.1 writes UTF-8 with a BOM.
	if _, err := Parse(append([]byte("\xef\xbb\xbf"), b...), keys); err != nil {
		t.Fatalf("with BOM: %v", err)
	}
}

func TestParseRejects(t *testing.T) {
	priv, keys := testKey(t)
	_, otherKeys := testKey(t)
	signed, _ := Sign(sample, priv)

	tamper := func(field, value string) []byte {
		var m map[string]any
		json.Unmarshal(signed, &m)
		m[field] = value
		b, _ := json.Marshal(m)
		return b
	}
	resign := func(i Info) []byte {
		b, err := Sign(i, priv)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	httpURL := sample
	httpURL.URL = "http://devstack.holostack.in/setup.exe"
	badHash := sample
	badHash.SHA256 = "abc"
	badVersion := sample
	badVersion.Version = "latest"

	cases := []struct {
		name string
		feed []byte
		keys []string
		want string
	}{
		{"unsigned", tamper("signature", ""), keys, "signature"},
		{"other key", signed, otherKeys, "signature"},
		{"url changed", tamper("url", "https://evil.example/setup.exe"), keys, "signature"},
		{"hash changed", tamper("sha256", strings.Repeat("cd", 32)), keys, "signature"},
		{"notes changed", tamper("notes", "hi"), keys, "signature"},
		{"http url", resign(httpURL), keys, "https"},
		{"bad hash", resign(badHash), keys, "sha256"},
		{"bad version", resign(badVersion), keys, "version"},
		{"not json", []byte("<html>"), keys, "malformed"},
	}
	for _, c := range cases {
		_, err := Parse(c.feed, c.keys)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
	}

	multi := sample
	multi.URL += "\nversion=9.9.9"
	if _, err := Sign(multi, priv); err == nil {
		t.Error("signed a multi-line url")
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.2.0", "0.1.9", true},
		{"0.10.0", "0.9.0", true},
		{"1.0.0", "1.0.0", false},
		{"0.1.0", "0.2.0", false},
		{"0.2.0", "dev", false},
		{"v0.2.0", "0.1.0", true},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
	if Release("dev") || !Release("0.1.0") {
		t.Error("Release")
	}
}

func TestDownload(t *testing.T) {
	body := []byte("pretend installer")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
	defer srv.Close()
	sum := sha256.Sum256(body)
	i := Info{Version: "0.2.0", URL: srv.URL, SHA256: hex.EncodeToString(sum[:])}

	dir := t.TempDir()
	var last int64
	path, err := Download(context.Background(), i, dir, func(done, total int64) { last = done })
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != string(body) || last != int64(len(body)) {
		t.Fatalf("got %q, progress %d", b, last)
	}

	if n := installerSize(context.Background(), srv.URL); n != int64(len(body)) {
		t.Errorf("installerSize = %d, want %d", n, len(body))
	}

	// A streamed download with no Content-Length reports progress against
	// the size Check found.
	chunked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		w.Write(body)
	}))
	defer chunked.Close()
	if n := installerSize(context.Background(), chunked.URL); n != 0 {
		t.Errorf("installerSize without a length = %d, want 0", n)
	}
	i.URL, i.Size = chunked.URL, int64(len(body))
	var total int64
	if _, err := Download(context.Background(), i, t.TempDir(), func(_, n int64) { total = n }); err != nil {
		t.Fatal(err)
	}
	if total != i.Size {
		t.Errorf("progress total = %d, want %d", total, i.Size)
	}

	i.SHA256 = strings.Repeat("00", 32)
	dir = t.TempDir()
	if _, err := Download(context.Background(), i, dir, nil); !errors.Is(err, errIntegrity) {
		t.Fatalf("err = %v", err)
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Fatalf("left behind: %v", left)
	}
}
