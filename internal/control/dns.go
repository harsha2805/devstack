package control

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/harsha2805/devstack/internal/api"
	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/dns"
	"github.com/harsha2805/devstack/internal/winproc"
)

// SetDNS turns <site>.<tld> addresses on or off for the stack at root.
//
// On: the Windows rule first (one administrator prompt), so saying no changes
// nothing; then services.dns = "on" and a reload. Off: services.dns = "off"
// and a reload first, so devstack stops answering even if the prompt to
// remove the rule is declined.
func SetDNS(ctx context.Context, root string, on bool) error {
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	value := config.Off
	if on {
		value = "on"
		if err := dns.SetRule(true, cfg.TLD); err != nil {
			if errors.Is(err, winproc.ErrDeclined) {
				return ErrDNSDeclined
			}
			return err
		}
	}
	if err := config.SetService(root, "dns", value); err != nil {
		return err
	}
	if c, err := api.Connect(root); err == nil {
		if _, err := c.Reload(ctx); err != nil {
			return err
		}
	}
	if !on {
		if err := dns.SetRule(false, cfg.TLD); err != nil {
			return fmt.Errorf("devstack no longer answers *.%s, but the Windows rule is still there: %w", cfg.TLD, err)
		}
	}
	return nil
}

// DNSStatus is what `devstack dns status`, doctor and the app show.
type DNSStatus struct {
	TLD     string `json:"tld"`
	On      bool   `json:"on"`      // services.dns = "on"
	Rule    bool   `json:"rule"`    // Windows sends *.tld lookups to devstack
	Answers bool   `json:"answers"` // a lookup through Windows resolved (servers running)
}

// CheckDNS reports the setting, the rule and, when probe is set, whether a
// lookup through Windows actually reaches devstack.
func CheckDNS(ctx context.Context, root string, probe bool) (DNSStatus, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return DNSStatus{}, err
	}
	s := DNSStatus{TLD: cfg.TLD, On: cfg.Services.DNS == "on", Rule: dns.RuleInstalled(cfg.TLD)}
	if probe && s.On && s.Rule {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		addrs, err := net.DefaultResolver.LookupHost(ctx, "devstack-check."+cfg.TLD)
		s.Answers = err == nil && len(addrs) > 0 && (addrs[0] == "127.0.0.1" || addrs[0] == "::1")
	}
	return s, nil
}

// ErrDNSDeclined is returned when the user says no to the administrator prompt.
var ErrDNSDeclined = errors.New("the administrator prompt was declined, so nothing changed")
