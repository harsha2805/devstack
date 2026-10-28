// Command catalog regenerates internal/packages/catalog.json, the pinned list
// of server builds devstack can download. Run it at release time:
//
//	go run ./tools/catalog
//
// PHP hashes come from php.net's releases.json. nginx and MySQL publish no
// sha256, so their archives are downloaded once here and hashed, as are the
// PECL extension builds (sqlsrv, pdo_sqlsrv) php.net's PHP zips don't bundle.
// Composer's hash comes from getcomposer.org, next to the newest stable phar.
// Entries whose URL is unchanged keep their previous hash (use -rehash to force).
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/packages"
)

const userAgent = "devstack-catalog"

func main() {
	out := flag.String("out", "internal/packages/catalog.json", "catalog file to write")
	phpMin := flag.String("php-min", "8.1", "oldest PHP minor to include")
	mysql := flag.String("mysql", "8.4.13:lts,9.7.2:lts", "MySQL versions, comma separated, optional :label")
	exts := flag.String("ext", "sqlsrv,pdo_sqlsrv", "PECL extensions to pin for every PHP minor, comma separated")
	rehash := flag.Bool("rehash", false, "download and hash everything again")
	flag.Parse()
	log.SetFlags(0)
	ctx := context.Background()

	old := map[string]packages.Entry{} // by URL, from the existing catalog
	oldExt := map[string]packages.Extension{}
	if b, err := os.ReadFile(*out); err == nil && !*rehash {
		var c packages.Catalog
		if json.Unmarshal(b, &c) == nil {
			for _, e := range c.Entries {
				old[e.URL] = e
			}
			for _, x := range c.Extensions {
				oldExt[x.URL] = x
			}
		}
	}

	var entries []packages.Entry
	php, err := phpEntries(ctx, *phpMin)
	check(err)
	entries = append(entries, php...)

	ng, err := nginxEntries(ctx)
	check(err)
	entries = append(entries, ng...)

	for _, spec := range strings.Split(*mysql, ",") {
		version, label, _ := strings.Cut(strings.TrimSpace(spec), ":")
		v, err := discover.ParseVersion(version)
		check(err)
		mm := v.MajorMinor()
		file := "mysql-" + version + "-winx64.zip"
		entries = append(entries, packages.Entry{
			Kind: discover.MySQL, Version: version, Label: label,
			URL:     "https://cdn.mysql.com/Downloads/MySQL-" + mm + "/" + file,
			Mirrors: []string{"https://cdn.mysql.com/archives/mysql-" + mm + "/" + file},
			Strip:   "mysql-" + version + "-winx64/",
		})
	}

	for i := range entries {
		e := &entries[i]
		if prev, ok := old[e.URL]; ok && (e.SHA256 == "" || e.SHA256 == prev.SHA256) {
			e.SHA256, e.Size = prev.SHA256, prev.Size
		}
		if e.SHA256 == "" || e.Size == 0 {
			log.Printf("hashing %s %s ...", e.Kind, e.Version)
			sum, size, err := hashURL(ctx, append([]string{e.URL}, e.Mirrors...))
			check(err)
			if e.SHA256 != "" && e.SHA256 != sum {
				log.Fatalf("%s: publisher hash %s != downloaded %s", e, e.SHA256, sum)
			}
			e.SHA256, e.Size = sum, size
		}
	}

	var xs []packages.Extension
	for _, name := range strings.Split(*exts, ",") {
		if name = strings.TrimSpace(name); name == "" {
			continue
		}
		found, err := peclEntries(ctx, name, php)
		check(err)
		xs = append(xs, found...)
	}
	for i := range xs {
		x := &xs[i]
		if prev, ok := oldExt[x.URL]; ok {
			x.SHA256, x.Size = prev.SHA256, prev.Size
			continue
		}
		log.Printf("hashing %s ...", x)
		sum, size, err := hashURL(ctx, []string{x.URL})
		check(err)
		x.SHA256, x.Size = sum, size
	}

	composer, err := composerEntry(ctx)
	check(err)

	c := packages.Catalog{Generated: time.Now().UTC().Truncate(time.Second), Entries: entries, Extensions: xs, Composer: composer}
	b, err := json.MarshalIndent(c, "", "  ")
	check(err)
	check(os.WriteFile(*out, append(b, '\n'), 0o644))
	log.Printf("wrote %s: %d entries, %d extension builds", *out, len(entries), len(xs))
}

// phpEntries reads php.net's release index: the newest NTS x64 build per minor.
// URLs point into archives/, which keeps every release, so they never go stale.
func phpEntries(ctx context.Context, min string) ([]packages.Entry, error) {
	body, err := get(ctx, "https://downloads.php.net/~windows/releases/releases.json")
	if err != nil {
		return nil, err
	}
	var idx map[string]map[string]json.RawMessage
	if err := json.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("releases.json: %w", err)
	}
	minV, _ := discover.ParseVersion(min)
	var out []packages.Entry
	for minor, fields := range idx {
		mv, err := discover.ParseVersion(minor)
		if err != nil || mv.Less(minV) {
			continue
		}
		var version string
		json.Unmarshal(fields["version"], &version)
		// "8.4.27-1" is php.net's rebuild of 8.4.27; php -v still says 8.4.27.
		version, _, _ = strings.Cut(version, "-")
		for name, raw := range fields {
			if !strings.HasPrefix(name, "nts-") || !strings.HasSuffix(name, "-x64") {
				continue
			}
			var build struct {
				Zip struct {
					Path   string `json:"path"`
					SHA256 string `json:"sha256"`
				} `json:"zip"`
			}
			if json.Unmarshal(raw, &build) != nil || build.Zip.Path == "" {
				continue
			}
			out = append(out, packages.Entry{
				Kind: discover.PHP, Version: version,
				URL:    "https://downloads.php.net/~windows/releases/archives/" + build.Zip.Path,
				SHA256: strings.ToLower(build.Zip.SHA256),
			})
		}
	}
	return out, nil
}

// peclEntries finds, for each PHP build, the newest stable release of a PECL
// extension built for that minor, NTS x64 with the same compiler (vs16, vs17).
// php.net publishes no hashes for these; main hashes the downloads.
func peclEntries(ctx context.Context, name string, php []packages.Entry) ([]packages.Extension, error) {
	base := "https://downloads.php.net/~windows/pecl/releases/" + name + "/"
	body, err := get(ctx, base)
	if err != nil {
		return nil, err
	}
	var versions []discover.Version
	for _, m := range regexp.MustCompile(`href="(\d+\.\d+\.\d+)/"`).FindAllStringSubmatch(string(body), -1) {
		if v, err := discover.ParseVersion(m[1]); err == nil {
			versions = append(versions, v)
		}
	}
	sort.Slice(versions, func(i, j int) bool { return versions[j].Less(versions[i]) })
	if len(versions) == 0 {
		return nil, fmt.Errorf("%s: no releases listed at %s", name, base)
	}

	listings := map[string]string{}
	vsRe := regexp.MustCompile(`-nts-Win32-(vs\d+)-x64\.zip$`)
	var out []packages.Extension
	for _, p := range php {
		pv, _ := discover.ParseVersion(p.Version)
		minor := pv.MajorMinor()
		m := vsRe.FindStringSubmatch(p.URL)
		if m == nil {
			return nil, fmt.Errorf("%s: can't tell the compiler from %s", p, p.URL)
		}
		found := false
		for _, v := range versions {
			dir := base + v.String() + "/"
			if _, ok := listings[dir]; !ok {
				b, err := get(ctx, dir)
				if err != nil {
					return nil, err
				}
				listings[dir] = string(b)
			}
			file := fmt.Sprintf("php_%s-%s-%s-nts-%s-x64.zip", name, v, minor, m[1])
			if strings.Contains(listings[dir], `href="`+file+`"`) {
				out = append(out, packages.Extension{Name: name, Version: v.String(), PHP: minor, URL: dir + file})
				found = true
				break
			}
		}
		if !found {
			log.Printf("%s: no build for PHP %s; skipped", name, minor)
		}
	}
	return out, nil
}

// composerEntry is the newest stable Composer 2 that isn't the 2.2 LTS line,
// with the sha256 getcomposer.org publishes beside it, checked by download.
func composerEntry(ctx context.Context) (*packages.Composer, error) {
	body, err := get(ctx, "https://getcomposer.org/versions")
	if err != nil {
		return nil, err
	}
	var idx struct {
		Stable []struct {
			Path    string `json:"path"`
			Version string `json:"version"`
			LTS     bool   `json:"lts"`
		} `json:"stable"`
	}
	if err := json.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("getcomposer.org/versions: %w", err)
	}
	for _, v := range idx.Stable {
		if v.LTS {
			continue
		}
		url := "https://getcomposer.org" + v.Path
		sum, err := get(ctx, url+".sha256sum")
		if err != nil {
			return nil, err
		}
		want, _, _ := strings.Cut(strings.TrimSpace(string(sum)), " ")
		log.Printf("hashing Composer %s ...", v.Version)
		got, size, err := hashURL(ctx, []string{url})
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(got, want) {
			return nil, fmt.Errorf("Composer %s: published hash %s != downloaded %s", v.Version, want, got)
		}
		return &packages.Composer{Version: v.Version, URL: url, SHA256: got, Size: size}, nil
	}
	return nil, fmt.Errorf("getcomposer.org/versions lists no stable release")
}

// nginxEntries takes the current mainline and stable zips from nginx.org's download page.
func nginxEntries(ctx context.Context) ([]packages.Entry, error) {
	body, err := get(ctx, "https://nginx.org/en/download.html")
	if err != nil {
		return nil, err
	}
	page := string(body)
	zipRe := regexp.MustCompile(`nginx-(\d+\.\d+\.\d+)\.zip`)
	var out []packages.Entry
	for _, sec := range []struct{ heading, label string }{{"Mainline version", "mainline"}, {"Stable version", "stable"}} {
		i := strings.Index(page, sec.heading)
		if i < 0 {
			return nil, fmt.Errorf("nginx download page: %q not found; page layout changed", sec.heading)
		}
		m := zipRe.FindStringSubmatch(page[i:])
		if m == nil {
			return nil, fmt.Errorf("nginx download page: no zip under %q", sec.heading)
		}
		out = append(out, packages.Entry{
			Kind: discover.Nginx, Version: m[1], Label: sec.label,
			URL:   "https://nginx.org/download/nginx-" + m[1] + ".zip",
			Strip: "nginx-" + m[1] + "/",
		})
	}
	return out, nil
}

func hashURL(ctx context.Context, urls []string) (string, int64, error) {
	var last error
	for _, u := range urls {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		req.Header.Set("User-Agent", userAgent)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			last = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			last = fmt.Errorf("%s: %s", u, resp.Status)
			continue
		}
		h := sha256.New()
		n, err := io.Copy(h, resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", 0, err
		}
		return hex.EncodeToString(h.Sum(nil)), n, nil
	}
	return "", 0, last
}

func get(ctx context.Context, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
