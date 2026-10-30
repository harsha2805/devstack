package control

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/harsha2805/devstack/internal/aiguide"
	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/sites"
	"github.com/harsha2805/devstack/internal/stack"
	"github.com/harsha2805/devstack/internal/winproc"
)

// Added is what AddProject did: the name the folder is served under, and
// anything about where it lives that will make it slow or flaky.
type Added struct {
	Name     string   `json:"name"`
	Warnings []string `json:"warnings,omitempty"`
}

// AddProject serves a folder outside www: it lists dir in stack.toml's
// [projects] as name (the folder's name made DNS-safe when name is "") and
// reloads the running servers.
func AddProject(ctx context.Context, root, dir, name string) (Added, error) {
	dir, err := filepath.Abs(strings.TrimSpace(dir))
	if err != nil {
		return Added{}, err
	}
	dir = winproc.LongPath(dir)
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return Added{}, fmt.Errorf("%s is not a folder", dir)
	}
	l, err := stack.Load(ctx, root)
	if l == nil {
		return Added{}, err
	}
	used := map[string]string{} // name -> the folder that has it
	for _, p := range l.Projects {
		if same(p.Dir, fi) {
			return Added{}, fmt.Errorf("%s is already listed as %s", dir, p.Name)
		}
		used[p.Name] = p.Dir
	}
	for _, s := range l.Sites {
		if same(s.Dir, fi) {
			return Added{}, fmt.Errorf("%s is already served as %s", dir, s.Slug)
		}
		used[s.Slug] = s.Dir
	}
	if name == "" {
		name = sites.Slugify(filepath.Base(dir))
		if name == "" {
			return Added{}, &NameError{Msg: fmt.Sprintf("%q has no letters or digits to make an address from; pick a name", filepath.Base(dir)), Suggestion: freeName("project", used)}
		}
	}
	if !config.ValidName(name) {
		return Added{}, &NameError{Msg: fmt.Sprintf("%q can't be a name: use lowercase letters, digits and dashes", name), Suggestion: freeName(sites.Slugify(name), used)}
	}
	if other, ok := used[name]; ok {
		return Added{}, &NameError{Msg: fmt.Sprintf("%s is already used by %s; pick another name", name, other), Suggestion: freeName(name, used)}
	}
	if err := config.AddProject(root, name, dir); err != nil {
		return Added{}, err
	}
	if err := reloadIfRunning(ctx, root); err != nil {
		return Added{}, fmt.Errorf("saved to %s, but the reload failed: %w", config.FileName, err)
	}
	return Added{Name: name, Warnings: LocationWarnings(dir)}, nil
}

// NameError is a project name that can't be used, with a free one to offer
// instead. Front ends ask for a name only when they get one of these.
type NameError struct {
	Msg        string
	Suggestion string
}

func (e *NameError) Error() string { return e.Msg }

// freeName is base, or base-2, base-3... whichever no site uses yet.
func freeName(base string, used map[string]string) string {
	if base == "" {
		base = "project"
	}
	if len(base) > 59 { // room for "-99" within a 63-character DNS label
		base = strings.TrimRight(base[:59], "-")
	}
	name := base
	for i := 2; used[name] != ""; i++ {
		name = fmt.Sprintf("%s-%d", base, i)
	}
	return name
}

// RemoveProject stops serving a listed project. The folder stays as it is,
// except for devstack's own AI guide, whose addresses would no longer work.
func RemoveProject(ctx context.Context, root, name string) error {
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	var dir string
	for _, p := range stack.Projects(cfg) {
		if p.Name == name {
			dir = p.Dir
		}
	}
	if err := config.RemoveProject(root, name); err != nil {
		return err
	}
	if dir != "" && aiguide.Installed(dir) {
		if err := aiguide.Remove(dir); err != nil {
			return fmt.Errorf("%s is no longer served, but its AI guide is still there: %w", name, err)
		}
	}
	return reloadIfRunning(ctx, root)
}

// LocationWarnings says what will go wrong serving a project from dir: PHP
// opens thousands of files per request, so slow or synced folders hurt.
func LocationWarnings(dir string) []string {
	var out []string
	lower := strings.ToLower(dir)
	switch {
	case strings.HasPrefix(lower, `\\wsl`), strings.HasPrefix(lower, `\\?\unc\wsl`):
		out = append(out, "This folder is inside WSL. Windows PHP reads it over a slow bridge, so pages load slowly; keep the project on a Windows drive.")
	case strings.HasPrefix(lower, `\\`), winproc.NetworkDrive(dir):
		out = append(out, "This folder is on a network drive. Pages load slowly, and the site stops when the drive disconnects.")
	}
	for _, v := range []string{"OneDrive", "OneDriveConsumer", "OneDriveCommercial"} {
		if od := os.Getenv(v); od != "" && within(lower, strings.ToLower(filepath.Clean(od))) {
			out = append(out, "This folder is synced by OneDrive. Syncing vendor/ and node_modules/ slows things down and can lock files while PHP writes them.")
			break
		}
	}
	return out
}

func within(dir, base string) bool {
	return dir == base || strings.HasPrefix(dir, base+string(filepath.Separator))
}

func same(dir string, fi os.FileInfo) bool {
	st, err := os.Stat(dir)
	return err == nil && os.SameFile(st, fi)
}
