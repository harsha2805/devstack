package daemon

import (
	"context"
	"strings"
	"time"

	"github.com/harsha2805/devstack/internal/api"
	"github.com/harsha2805/devstack/internal/sites"
)

// watchInterval is how often www is rescanned. Polling a scan, rather than
// file notifications, sees exactly what decides the config (folders,
// .devstack.toml, docroot candidates, index.php) and ignores churn inside
// projects such as vendor/ or node_modules/.
var watchInterval = 1500 * time.Millisecond

// watchSites reloads when sites in www change: a folder added, removed or
// renamed, a .devstack.toml edited, a public/ folder or index.php appearing.
// stack.toml is not watched; it still takes an explicit reload.
func (d *Daemon) watchSites(ctx context.Context) {
	t := time.NewTicker(watchInterval)
	defer t.Stop()
	var s settler
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.shutdown:
			return
		case <-t.C:
		}
		d.mu.Lock()
		www, running, phase := d.www, d.scan, d.phase
		d.mu.Unlock()
		if phase != api.PhaseRunning || www == "" {
			continue
		}
		list, _, err := sites.Scan(www)
		if err != nil {
			continue // www missing or unreadable; the next reload reports it
		}
		current := fingerprint(list)
		if !s.observe(current, running) {
			continue
		}
		d.log.Printf("www changed; reloading")
		actions, err := d.reload(ctx)
		for _, a := range actions {
			d.log.Printf("  %s", a)
		}
		if err != nil {
			d.log.Printf("reload after www change: %v", err)
			// Don't retry the same state every tick; the next change tries again.
			d.mu.Lock()
			if d.scan == running {
				d.scan = current
			}
			d.mu.Unlock()
		}
	}
}

// settler decides when www has changed for good. A scan that differs from
// what is running must be seen on two ticks in a row, so a folder being
// copied or cloned is picked up once it settles rather than half-made.
type settler struct{ pending string }

func (s *settler) observe(current, running string) bool {
	if current == running {
		s.pending = ""
		return false
	}
	if current != s.pending {
		s.pending = current
		return false
	}
	s.pending = ""
	return true
}

// fingerprint is everything about the site list that affects config.
func fingerprint(list []sites.Site) string {
	var b strings.Builder
	for _, s := range list {
		b.WriteString(s.Name)
		b.WriteByte(0)
		b.WriteString(s.Slug)
		b.WriteByte(0)
		b.WriteString(s.Docroot)
		b.WriteByte(0)
		b.WriteString(s.PHP)
		b.WriteByte(0)
		b.WriteString(strings.Join(s.Aliases, ","))
		if s.HasIndex {
			b.WriteString("\x00index")
		}
		b.WriteByte('\n')
	}
	return b.String()
}
