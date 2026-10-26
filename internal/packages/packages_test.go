package packages

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/harsha2805/devstack/internal/discover"
)

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(body))
	}
	w.Close()
	return buf.Bytes()
}

func serve(t *testing.T, routes map[string][]byte) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestBuiltInCatalog(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []discover.Kind{discover.Nginx, discover.PHP, discover.MySQL} {
		if _, err := c.Find(k, ""); err != nil {
			t.Errorf("catalog has no %s: %v", k, err)
		}
	}
	for _, e := range c.Entries {
		if len(e.SHA256) != 64 || e.Size == 0 || !strings.HasPrefix(e.URL, "https://") {
			t.Errorf("incomplete entry %+v", e)
		}
	}
}

func TestFindPicksNewestMatch(t *testing.T) {
	c := Catalog{Entries: []Entry{
		{Kind: discover.PHP, Version: "8.3.9"}, {Kind: discover.PHP, Version: "8.10.1"}, {Kind: discover.PHP, Version: "8.3.36"},
	}}
	c.sort()
	if e, _ := c.Find(discover.PHP, ""); e.Version != "8.10.1" {
		t.Errorf("newest = %s", e.Version)
	}
	if e, _ := c.Find(discover.PHP, "8.3"); e.Version != "8.3.36" {
		t.Errorf("8.3 = %s", e.Version)
	}
	if _, err := c.Find(discover.PHP, "7"); err == nil {
		t.Error("expected no match for 7")
	}
}

func TestInstallStripsAndVerifies(t *testing.T) {
	z := makeZip(t, map[string]string{"nginx-1.2.3/nginx.exe": "exe", "nginx-1.2.3/conf/mime.types": "types"})
	srv := serve(t, map[string][]byte{"/archive/n.zip": z})
	bin := t.TempDir()
	e := Entry{
		Kind: discover.Nginx, Version: "1.2.3", SHA256: sum(z), Strip: "nginx-1.2.3/",
		URL: srv.URL + "/gone/n.zip", Mirrors: []string{srv.URL + "/archive/n.zip"}, // first URL 404s
	}
	dir, err := Install(context.Background(), bin, e, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "conf", "mime.types")); string(b) != "types" {
		t.Errorf("not unpacked with prefix stripped: %s", dir)
	}
	if _, err := Install(context.Background(), bin, e, nil); !errors.Is(err, ErrInstalled) {
		t.Errorf("second install: %v", err)
	}
	leftovers, _ := filepath.Glob(filepath.Join(bin, "nginx", ".*"))
	if len(leftovers) != 0 {
		t.Errorf("temporary files left: %v", leftovers)
	}
}

func TestInstallRejectsBadChecksum(t *testing.T) {
	z := makeZip(t, map[string]string{"php.exe": "x"})
	srv := serve(t, map[string][]byte{"/p.zip": z})
	bin := t.TempDir()
	e := Entry{Kind: discover.PHP, Version: "8.4.1", URL: srv.URL + "/p.zip", SHA256: strings.Repeat("0", 64)}
	_, err := Install(context.Background(), bin, e, nil)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want checksum error, got %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(bin, "php")); len(entries) != 0 {
		t.Errorf("failed install left files: %v", entries)
	}
}

func TestInstallRejectsPathTraversal(t *testing.T) {
	for _, name := range []string{"../evil.txt", "a/../../evil.txt", "C:/evil.txt"} {
		z := makeZip(t, map[string]string{name: "x"})
		srv := serve(t, map[string][]byte{"/z.zip": z})
		bin := t.TempDir()
		e := Entry{Kind: discover.PHP, Version: "8.4.1", URL: srv.URL + "/z.zip", SHA256: sum(z)}
		if _, err := Install(context.Background(), bin, e, nil); err == nil || !strings.Contains(err.Error(), "unsafe path") {
			t.Errorf("%s: want unsafe path error, got %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(bin), "evil.txt")); err == nil {
			t.Errorf("%s: file escaped the install folder", name)
		}
	}
}

func TestInstallRefusesUnpinned(t *testing.T) {
	if _, err := Install(context.Background(), t.TempDir(), Entry{Kind: discover.PHP, Version: "1"}, nil); err == nil {
		t.Error("entries without sha256 must be refused")
	}
}
