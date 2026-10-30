package daemon

import (
	"testing"

	"github.com/harsha2805/devstack/internal/sites"
)

func TestSettler(t *testing.T) {
	var s settler
	steps := []struct {
		current, running string
		want             bool
	}{
		{"a", "a", false}, // nothing changed
		{"b", "a", false}, // changed: wait one tick
		{"b", "a", true},  // same again: reload
		{"c", "b", false}, // changed again
		{"d", "b", false}, // still moving (a clone in progress)
		{"d", "b", true},  // settled
		{"e", "d", false}, // changed...
		{"d", "d", false}, // ...and back before it settled: nothing to do
		{"e", "d", false}, // a fresh change waits its own tick
	}
	for i, st := range steps {
		if got := s.observe(st.current, st.running); got != st.want {
			t.Fatalf("step %d: observe(%q, %q) = %v", i, st.current, st.running, got)
		}
	}
}

func TestFingerprint(t *testing.T) {
	base := []sites.Site{{Name: "app", Slug: "app", Docroot: `C:\www\app`}}
	same := []sites.Site{{Name: "app", Slug: "app", Docroot: `C:\www\app`, Dir: `ignored`}}
	if fingerprint(base, nil) != fingerprint(same, nil) {
		t.Error("fields that don't affect config changed the fingerprint")
	}
	for name, changed := range map[string][]sites.Site{
		"docroot":  {{Name: "app", Slug: "app", Docroot: `C:\www\app\public`}},
		"php":      {{Name: "app", Slug: "app", Docroot: `C:\www\app`, PHP: "8.2"}},
		"index":    {{Name: "app", Slug: "app", Docroot: `C:\www\app`, HasIndex: true}},
		"aliases":  {{Name: "app", Slug: "app", Docroot: `C:\www\app`, Aliases: []string{"a.test"}}},
		"new site": append(base, sites.Site{Name: "b", Slug: "b", Docroot: `C:\www\b`}),
		"removed":  nil,
	} {
		if fingerprint(changed, nil) == fingerprint(base, nil) {
			t.Errorf("%s: fingerprint unchanged", name)
		}
	}
}
