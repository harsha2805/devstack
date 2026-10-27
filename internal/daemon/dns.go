package daemon

import (
	"github.com/harsha2805/devstack/internal/dns"
	"github.com/harsha2805/devstack/internal/model"
)

// applyDNS starts, retunes or stops the *.<tld> responder to match st, and
// adds a warning to st when names won't resolve. Called on start and reload.
func (d *Daemon) applyDNS(st *model.Stack) {
	d.dnsMu.Lock()
	defer d.dnsMu.Unlock()
	if !st.DNS {
		if d.dnsSrv != nil {
			d.dnsSrv.Close()
			d.dnsSrv = nil
			d.log.Printf("dns: stopped")
		}
		return
	}
	if d.dnsSrv != nil {
		d.dnsSrv.Set(st.TLD, st.IPv6)
	} else if srv, err := dns.Listen(dns.Addr, st.TLD, st.IPv6); err != nil {
		st.Warnings = append(st.Warnings, "dns: can't listen on "+dns.Addr+" ("+err.Error()+"), so *."+st.TLD+" names won't resolve")
		d.log.Printf("dns: %v", err)
	} else {
		d.dnsSrv = srv
		d.log.Printf("dns: answering *.%s on %s", st.TLD, dns.Addr)
	}
	if !dns.RuleInstalled(st.TLD) {
		st.Warnings = append(st.Warnings, "dns: services.dns is on, but Windows doesn't send *."+st.TLD+
			" lookups to devstack; turn .test addresses on again in the app (or run `devstack dns on`)")
	}
}

func (d *Daemon) stopDNS() {
	d.dnsMu.Lock()
	defer d.dnsMu.Unlock()
	if d.dnsSrv != nil {
		d.dnsSrv.Close()
		d.dnsSrv = nil
	}
}
