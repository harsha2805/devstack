// Package packages downloads server builds from their publishers, verifies
// them against the pinned catalog, and unpacks them into bin/<kind>/<version>/
// where discover finds them.
package packages

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/harsha2805/devstack/internal/discover"
)

// Entry is one downloadable build.
type Entry struct {
	Kind    discover.Kind `json:"kind"`
	Version string        `json:"version"`
	URL     string        `json:"url"`
	// Mirrors are tried in order when URL fails. MySQL moves superseded
	// releases from Downloads/ to archives/, so entries list both.
	Mirrors []string `json:"mirrors,omitempty"`
	SHA256  string   `json:"sha256"`
	Size    int64    `json:"size"`
	// Strip is the top-level folder inside the zip ("nginx-1.30.6/"), removed on extract.
	Strip string `json:"strip,omitempty"`
	// Label marks notable lines: "lts", "stable", "mainline".
	Label string `json:"label,omitempty"`
}

func (e Entry) String() string { return fmt.Sprintf("%s %s", e.Kind, e.Version) }

// Title is the name people know the product by: "PHP 8.4.27", "MySQL 8.4.13".
func (e Entry) Title() string { return ProductName(e.Kind) + " " + e.Version }

// ProductName spells a server kind the way its project does.
func ProductName(k discover.Kind) string {
	switch k {
	case discover.PHP:
		return "PHP"
	case discover.MySQL:
		return "MySQL"
	}
	return string(k)
}

type Catalog struct {
	Generated  time.Time   `json:"generated"`
	Entries    []Entry     `json:"entries"`
	Extensions []Extension `json:"extensions,omitempty"`
	Composer   *Composer   `json:"composer,omitempty"`
}

// Composer is the composer.phar devstack's composer command runs.
type Composer struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

func (c Composer) String() string { return "Composer " + c.Version }

// ComposerDir is where composer.phar versions live under a bin root.
func ComposerDir(binRoot string) string { return filepath.Join(binRoot, "composer") }

// FindComposer returns the newest installed composer.phar under binRoot.
func FindComposer(binRoot string) (string, bool) {
	entries, _ := os.ReadDir(ComposerDir(binRoot))
	var best discover.Version
	path := ""
	for _, e := range entries {
		v, err := discover.ParseVersion(e.Name())
		phar := filepath.Join(ComposerDir(binRoot), e.Name(), "composer.phar")
		if _, serr := os.Stat(phar); err != nil || serr != nil {
			continue
		}
		if path == "" || best.Less(v) {
			best, path = v, phar
		}
	}
	return path, path != ""
}

// InstallComposer downloads and verifies c into bin/composer/<version>/.
func InstallComposer(ctx context.Context, binRoot string, c Composer, progress Progress) (string, error) {
	if len(c.SHA256) != 64 {
		return "", fmt.Errorf("%s: catalog entry has no sha256; refusing to install unverified files", c)
	}
	dir := filepath.Join(ComposerDir(binRoot), c.Version)
	dest := filepath.Join(dir, "composer.phar")
	if _, err := os.Stat(dest); err == nil {
		return dest, fmt.Errorf("%s: %w (%s)", c, ErrInstalled, dest)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	partial := dest + ".partial"
	defer os.Remove(partial)
	if err := download(ctx, c.String(), c.SHA256, c.Size, c.URL, partial, progress); err != nil {
		return "", err
	}
	if err := os.Rename(partial, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// Extension is a PHP extension DLL that php.net's Windows builds don't bundle,
// built for one PHP minor (NTS x64, matching compiler) by its publisher.
type Extension struct {
	Name    string `json:"name"`    // "sqlsrv", as written in php.extensions
	Version string `json:"version"` // the extension's own version, "5.13.3"
	PHP     string `json:"php"`     // PHP minor it is built for, "8.4"
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

func (x Extension) String() string { return fmt.Sprintf("%s %s for PHP %s", x.Name, x.Version, x.PHP) }

// ODBCDriverURL is Microsoft's page for the ODBC Driver for SQL Server, which
// sqlsrv and pdo_sqlsrv need and which only Microsoft's installer can add.
const ODBCDriverURL = "https://learn.microsoft.com/sql/connect/odbc/download-odbc-driver-for-sql-server"

// NeedsODBC reports whether an extension only works with Microsoft's ODBC
// Driver for SQL Server installed.
func NeedsODBC(name string) bool { return name == "sqlsrv" || name == "pdo_sqlsrv" }

//go:embed catalog.json
var catalogJSON []byte

// Default is the catalog built into this executable.
func Default() (Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(catalogJSON, &c); err != nil {
		return c, fmt.Errorf("built-in catalog: %w", err)
	}
	c.sort()
	return c, nil
}

func (c *Catalog) sort() {
	sort.SliceStable(c.Entries, func(i, j int) bool {
		a, b := c.Entries[i], c.Entries[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		va, _ := discover.ParseVersion(a.Version)
		vb, _ := discover.ParseVersion(b.Version)
		return vb.Less(va) // newest first
	})
}

// Find returns the newest entry of kind matching constraint ("" = newest, "8.4", "8.4.27").
func (c Catalog) Find(kind discover.Kind, constraint string) (Entry, error) {
	var want discover.Version
	if constraint != "" {
		v, err := discover.ParseVersion(constraint)
		if err != nil {
			return Entry{}, err
		}
		want = v
	}
	var have []string
	for _, e := range c.Entries {
		if e.Kind != kind {
			continue
		}
		have = append(have, e.Version)
		v, err := discover.ParseVersion(e.Version)
		if err == nil && (want == nil || v.Matches(want)) {
			return e, nil
		}
	}
	if len(have) == 0 {
		return Entry{}, fmt.Errorf("no %s builds in the catalog", kind)
	}
	return Entry{}, fmt.Errorf("no %s %s in the catalog (available: %s)", kind, constraint, strings.Join(have, ", "))
}

// ExtensionNames lists the extensions the catalog can install, sorted.
func (c Catalog) ExtensionNames() []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range c.Extensions {
		if !seen[x.Name] {
			seen[x.Name] = true
			out = append(out, x.Name)
		}
	}
	sort.Strings(out)
	return out
}

// FindExtension returns the build of extension name for PHP minor php ("8.4").
func (c Catalog) FindExtension(name, php string) (Extension, error) {
	known := false
	for _, x := range c.Extensions {
		if x.Name != name {
			continue
		}
		known = true
		if x.PHP == php {
			return x, nil
		}
	}
	if !known {
		return Extension{}, fmt.Errorf("devstack can't install the %s extension (it can install: %s)", name, strings.Join(c.ExtensionNames(), ", "))
	}
	return Extension{}, fmt.Errorf("no %s build for PHP %s", name, php)
}

// Dir is where an entry is installed under a bin root.
func Dir(binRoot string, kind discover.Kind, version string) string {
	return filepath.Join(binRoot, string(kind), version)
}

// Progress reports bytes downloaded so far out of total (0 if unknown).
type Progress func(done, total int64)

// UserAgent identifies devstack to download servers (some reject Go's default).
var UserAgent = "devstack"

var client = &http.Client{Timeout: 0} // large downloads; cancellation comes from ctx

// ErrInstalled means the version is already present.
var ErrInstalled = errors.New("already installed")

// Install downloads, verifies and unpacks e into binRoot. On any failure,
// including cancellation, nothing is left in bin/.
func Install(ctx context.Context, binRoot string, e Entry, progress Progress) (string, error) {
	if len(e.SHA256) != 64 {
		return "", fmt.Errorf("%s: catalog entry has no sha256; refusing to install unverified files", e)
	}
	dest := Dir(binRoot, e.Kind, e.Version)
	if _, err := os.Stat(dest); err == nil {
		return dest, fmt.Errorf("%s: %w (%s)", e, ErrInstalled, dest)
	}
	kindDir := filepath.Dir(dest)
	if err := os.MkdirAll(kindDir, 0o755); err != nil {
		return "", err
	}
	zipPath := filepath.Join(kindDir, "."+e.Version+".zip.partial")
	partial := filepath.Join(kindDir, "."+e.Version+".partial")
	defer os.Remove(zipPath)
	defer os.RemoveAll(partial)

	var errs []error
	for _, u := range append([]string{e.URL}, e.Mirrors...) {
		err := download(ctx, e.String(), e.SHA256, e.Size, u, zipPath, progress)
		if err == nil {
			errs = nil
			break
		}
		errs = append(errs, err)
		var mismatch checksumError
		if errors.As(err, &mismatch) || ctx.Err() != nil {
			break // a mirror can't fix a bad file or a cancel
		}
	}
	if len(errs) > 0 {
		return "", errors.Join(errs...)
	}
	os.RemoveAll(partial)
	if err := extract(zipPath, partial, e.Strip); err != nil {
		return "", fmt.Errorf("%s: unpack: %w", e, err)
	}
	if err := os.Rename(partial, dest); err != nil {
		return "", err
	}
	return dest, nil
}

type checksumError struct{ msg string }

func (c checksumError) Error() string { return c.msg }

// InstallExtension downloads and verifies x and puts its DLL into extDir
// (a PHP build's ext folder). It returns the DLL's path.
func InstallExtension(ctx context.Context, extDir string, x Extension, progress Progress) (string, error) {
	if len(x.SHA256) != 64 {
		return "", fmt.Errorf("%s: catalog entry has no sha256; refusing to install unverified files", x)
	}
	dll := "php_" + x.Name + ".dll"
	dest := filepath.Join(extDir, dll)
	if _, err := os.Stat(dest); err == nil {
		return dest, fmt.Errorf("%s: %w (%s)", x, ErrInstalled, dest)
	}
	if _, err := os.Stat(extDir); err != nil {
		return "", fmt.Errorf("%s: %w", x, err)
	}
	zipPath := filepath.Join(extDir, "."+x.Name+".zip.partial")
	partial := dest + ".partial"
	defer os.Remove(zipPath)
	defer os.Remove(partial)
	if err := download(ctx, x.String(), x.SHA256, x.Size, x.URL, zipPath, progress); err != nil {
		return "", err
	}
	if err := extractFile(zipPath, dll, partial); err != nil {
		return "", fmt.Errorf("%s: unpack: %w", x, err)
	}
	if err := os.Rename(partial, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// extractFile copies the archive's top-level file name to target.
func extractFile(zipPath, name, target string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if strings.EqualFold(f.Name, name) {
			return writeEntry(f, target)
		}
	}
	return fmt.Errorf("no %s in the archive", name)
}

func download(ctx context.Context, what, want string, size int64, url, to string, progress Progress) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: download: %w", what, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: download %s: %s", what, url, resp.Status)
	}
	total := resp.ContentLength
	if total <= 0 {
		total = size
	}

	f, err := os.Create(to)
	if err != nil {
		return err
	}
	h := sha256.New()
	var done int64
	buf := make([]byte, 256<<10)
	last := time.Time{}
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				return werr
			}
			h.Write(buf[:n])
			done += int64(n)
			if progress != nil && time.Since(last) > 100*time.Millisecond {
				progress(done, total)
				last = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			return fmt.Errorf("%s: download: %w", what, rerr)
		}
	}
	if progress != nil {
		progress(done, total)
	}
	if err := f.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, want) {
		return checksumError{fmt.Sprintf("%s: checksum mismatch: got %s, catalog says %s; the download was altered or corrupted", what, got, want)}
	}
	return nil
}

// extract unpacks zipPath into dir, removing the strip prefix and refusing
// any entry that would land outside dir.
func extract(zipPath, dir, strip string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	strip = strings.TrimSuffix(strip, "/")
	for _, f := range r.File {
		name := strings.ReplaceAll(f.Name, `\`, "/")
		if strip != "" {
			if name == strip+"/" || name == strip {
				continue
			}
			if !strings.HasPrefix(name, strip+"/") {
				return fmt.Errorf("unexpected entry %q outside %s/", f.Name, strip)
			}
			name = strings.TrimPrefix(name, strip+"/")
		}
		clean := path.Clean(name)
		if clean == "." || path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
			return fmt.Errorf("unsafe path %q in archive", f.Name)
		}
		target := filepath.Join(dir, filepath.FromSlash(clean))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeEntry(f, target); err != nil {
			return err
		}
	}
	return nil
}

func writeEntry(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Remove deletes an installed version.
func Remove(binRoot string, kind discover.Kind, version string) error {
	dir := Dir(binRoot, kind, version)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("%s %s is not installed in %s", kind, version, binRoot)
	}
	return os.RemoveAll(dir)
}
