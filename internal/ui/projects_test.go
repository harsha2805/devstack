package ui

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/harsha2805/devstack/internal/config"
)

func TestProjectAddRemove(t *testing.T) {
	s := testServer(t)
	root, outside := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(root, "www"), 0o755)
	os.MkdirAll(filepath.Join(outside, "public"), 0o755)
	os.WriteFile(filepath.Join(root, config.FileName), []byte("www = \"www\"\n\n[paths]\nbin = [\"bin\"]\n"), 0o644)
	s.settings.Root = root

	post := func(path, body string) *http.Response {
		return do(t, s, "POST", path, func(r *http.Request) {
			r.Header.Set("X-Token", s.token)
			r.Header.Set("X-Devstack", "1")
			r.Body = io.NopCloser(strings.NewReader(body))
		}).Result()
	}
	js := func(p string) string { return strings.ReplaceAll(p, `\`, `\\`) }

	if r := post("/api/project/add", `{"dir":"`+js(outside)+`","name":"shop"}`); r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("add: %d %s", r.StatusCode, b)
	}
	cfg, err := config.Load(root)
	if err != nil || cfg.Projects["shop"] == "" {
		t.Fatalf("not listed: %v %v", cfg.Projects, err)
	}
	if !inProject(root, filepath.Join(outside, "public")) || inProject(root, `C:\Windows`) {
		t.Error("open folder should allow the project's folders and nothing else")
	}
	if r := post("/api/project/add", `{"dir":"`+js(outside)+`","name":"other"}`); r.StatusCode != http.StatusBadRequest {
		t.Errorf("same folder twice: got %d", r.StatusCode)
	}
	// No name given: the folder's own name, unless a site has it; only then
	// is the user asked, with a free name to start from.
	blog := filepath.Join(t.TempDir(), "Blog")
	os.MkdirAll(blog, 0o755)
	if r := post("/api/project/add", `{"dir":"`+js(blog)+`"}`); r.StatusCode != http.StatusOK {
		t.Fatalf("add by folder name: %d", r.StatusCode)
	}
	if cfg, _ := config.Load(root); cfg.Projects["blog"] == "" {
		t.Fatalf("not listed as blog: %v", cfg.Projects)
	}
	other := filepath.Join(t.TempDir(), "blog")
	os.MkdirAll(other, 0o755)
	r := post("/api/project/add", `{"dir":"`+js(other)+`"}`)
	b, _ := io.ReadAll(r.Body)
	if r.StatusCode != http.StatusConflict || !strings.Contains(string(b), `"suggestion":"blog-2"`) {
		t.Errorf("name clash: got %d %s, want 409 suggesting blog-2", r.StatusCode, b)
	}

	if r := post("/api/project/remove", `{"name":"shop"}`); r.StatusCode != http.StatusOK {
		t.Fatalf("remove: %d", r.StatusCode)
	}
	if cfg, _ := config.Load(root); len(cfg.Projects) != 1 {
		t.Errorf("still listed: %v", cfg.Projects)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Error("removing a project must keep its folder")
	}
}
