package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/harsha2805/devstack/internal/api"
	"github.com/harsha2805/devstack/internal/certs"
	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/winproc"
)

// SetHTTPS turns trusted HTTPS on or off for the stack at root.
//
// On: make the local CA if needed and have Windows trust it (Windows asks
// once; no administrator rights), move ports.https off a port another program
// holds, then services.https = "on" and a reload. Off: services.https = "off"
// and a reload, then remove the CA from Windows and delete it.
func SetHTTPS(ctx context.Context, root string, on bool) error {
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	if !on {
		if err := config.SetService(root, "https", config.Off); err != nil {
			return err
		}
		if err := reloadIfRunning(ctx, root); err != nil {
			return err
		}
		if _, err := certs.Untrust(); err != nil {
			return fmt.Errorf("HTTPS is off, but the devstack certificate is still trusted: %w", err)
		}
		os.RemoveAll(filepath.Join(root, "data", "devstack", "certs"))
		return certs.Delete()
	}

	ca, err := certs.LoadOrCreate(cfg.TLD)
	if err != nil {
		return err
	}
	if !ca.Trusted() {
		if err := ca.Trust(); err != nil {
			return err
		}
	}
	// Only when turning on: once nginx serves HTTPS it holds the port itself.
	if port := freeHTTPSPort(root, cfg.Ports.HTTPS); cfg.Services.HTTPS != "on" && port != cfg.Ports.HTTPS {
		if err := config.SetKey(root, "ports", "https", strconv.Itoa(port)); err != nil {
			return err
		}
	}
	if err := config.SetService(root, "https", "on"); err != nil {
		return err
	}
	return reloadIfRunning(ctx, root)
}

// freeHTTPSPort keeps want unless another program listens on it, then tries
// the usual alternatives. A port this stack's own nginx holds counts as free.
func freeHTTPSPort(root string, want int) int {
	own := map[int]bool{}
	if c, err := api.Connect(root); err == nil {
		if s, err := c.Status(context.Background()); err == nil {
			for _, u := range s.Units {
				own[u.PID] = true
			}
		}
	}
	for _, p := range []int{want, 443, 8443, 9443, 10443, 18443} {
		pid, err := winproc.PortOwner(p)
		if err == nil && (pid == 0 || own[pid]) {
			return p
		}
	}
	return want
}

func reloadIfRunning(ctx context.Context, root string) error {
	c, err := api.Connect(root)
	if errors.Is(err, api.ErrNotRunning) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = c.Reload(ctx)
	return err
}

// HTTPSStatus is what `devstack https status`, doctor and the app show.
type HTTPSStatus struct {
	On      bool   `json:"on"`      // services.https = "on"
	Trusted bool   `json:"trusted"` // the CA is in the user's Root store
	Port    int    `json:"port"`
	CA      string `json:"ca,omitempty"` // ca.crt, for tools with their own trust store
}

// CheckHTTPS reports the setting and whether the CA is trusted.
func CheckHTTPS(root string) (HTTPSStatus, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return HTTPSStatus{}, err
	}
	s := HTTPSStatus{On: cfg.Services.HTTPS == "on", Port: cfg.Ports.HTTPS}
	if ca, err := certs.Load(); err == nil {
		s.Trusted = ca.Trusted()
		s.CA = ca.Path
	}
	return s, nil
}
