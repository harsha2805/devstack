package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/systray"

	"github.com/harsha2805/devstack/internal/api"
	"github.com/harsha2805/devstack/internal/control"
	"github.com/harsha2805/devstack/internal/stack"
	"github.com/harsha2805/devstack/internal/supervisor"
	"github.com/harsha2805/devstack/internal/ui"
	"github.com/harsha2805/devstack/internal/winproc"
)

var (
	//go:embed icons/tray.ico
	iconStopped []byte
	//go:embed icons/tray-running.ico
	iconRunning []byte
)

// trayApp is the tray icon and its menu. Left click opens the window; the
// menu has the everyday actions.
type trayApp struct {
	srv *ui.Server
	win *window

	toggle *systray.MenuItem
	sites  *systray.MenuItem

	// mu guards the fields below: the ticker and a menu click both refresh.
	mu       sync.Mutex
	siteItem map[string]*systray.MenuItem
	siteURL  map[string]string
	ticks    int // refreshes so far
	shown    int // sites in the submenu
	quitting atomic.Bool
}

func (a *trayApp) run() {
	systray.Run(a.ready, func() { os.Exit(0) })
}

func (a *trayApp) ready() {
	systray.SetIcon(iconStopped)
	systray.SetTooltip("devstack")
	systray.SetOnTapped(a.win.Show)

	open := systray.AddMenuItem("Open devstack", "")
	systray.AddSeparator()
	a.toggle = systray.AddMenuItem("Start servers", "")
	a.sites = systray.AddMenuItem("Sites", "")
	a.siteItem = map[string]*systray.MenuItem{}
	a.siteURL = map[string]string{}
	www := systray.AddMenuItem("Open www folder", "")
	systray.AddSeparator()
	quit := systray.AddMenuItem("Stop servers and quit", "")

	go func() {
		for {
			select {
			case <-open.ClickedCh:
				a.win.Show()
			case <-a.toggle.ClickedCh:
				if a.running() {
					a.srv.StopServersAsync()
				} else {
					a.srv.StartServers()
				}
				a.refresh()
			case <-www.ClickedCh:
				if root := a.srv.Root(); root != "" {
					if l, err := stack.Load(context.Background(), root); err == nil {
						winproc.ShellOpen(l.Stack.WWW)
					}
				}
			case <-quit.ClickedCh:
				a.quit()
			}
		}
	}()
	go func() {
		for {
			a.refresh()
			time.Sleep(2 * time.Second)
		}
	}()
}

func (a *trayApp) running() bool {
	root := a.srv.Root()
	if root == "" {
		return false
	}
	_, err := api.Connect(root)
	return err == nil
}

// refresh updates the icon, tooltip, toggle label and the Sites submenu.
func (a *trayApp) refresh() {
	if a.quitting.Load() {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	root := a.srv.Root()
	busy := a.srv.Busy()
	state, tip := "stopped", "devstack: stopped"
	if root == "" {
		tip = "devstack: not set up yet"
	} else if c, err := api.Connect(root); err == nil {
		state, tip = "running", "devstack: running"
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if s, err := c.Status(ctx); err == nil {
			for _, g := range control.Groups(s.Units) {
				if g.State != supervisor.Running {
					state, tip = "attention", fmt.Sprintf("devstack: %s is %s", g.Name, g.State)
				}
			}
		}
		cancel()
	}
	if busy != "" {
		tip = "devstack: " + busy + "…"
	}
	if state == "stopped" {
		systray.SetIcon(iconStopped)
	} else {
		systray.SetIcon(iconRunning)
	}
	systray.SetTooltip(tip)

	switch {
	case busy != "":
		a.toggle.SetTitle(busy + "…")
		a.toggle.Disable()
	case root == "":
		a.toggle.SetTitle("Start servers")
		a.toggle.Disable()
	case state == "stopped":
		a.toggle.SetTitle("Start servers")
		a.toggle.Enable()
	default:
		a.toggle.SetTitle("Stop servers")
		a.toggle.Enable()
	}
	a.refreshSites(root, state != "stopped")
	a.ticks++
}

// refreshSites runs with a.mu held. The site list is re-read every tenth
// second; the rest of the menu follows the servers every tick.
func (a *trayApp) refreshSites(root string, running bool) {
	if root == "" {
		a.sites.Disable()
		return
	}
	if a.ticks%5 != 0 {
		if running && a.shown > 0 {
			a.sites.Enable()
		} else {
			a.sites.Disable()
		}
		return
	}
	l, err := stack.Load(context.Background(), root)
	if err != nil || l.Stack == nil {
		return
	}
	seen := map[string]bool{}
	for _, s := range l.Stack.Sites {
		seen[s.Slug] = true
		url := l.Stack.URL(s)
		a.siteURL[s.Slug] = url
		if _, ok := a.siteItem[s.Slug]; ok {
			continue
		}
		item := a.sites.AddSubMenuItem(s.Slug, url)
		a.siteItem[s.Slug] = item
		slug := s.Slug
		go func() {
			for range item.ClickedCh {
				a.mu.Lock()
				url := a.siteURL[slug]
				a.mu.Unlock()
				winproc.ShellOpen(url)
			}
		}()
	}
	for slug, item := range a.siteItem {
		if seen[slug] {
			item.Show()
		} else {
			item.Hide()
		}
	}
	a.shown = len(seen)
	if running && len(seen) > 0 {
		a.sites.Enable()
	} else {
		a.sites.Disable()
	}
}

// quit stops the servers, then exits.
func (a *trayApp) quit() {
	if a.quitting.Swap(true) {
		return
	}
	systray.SetTooltip("devstack: stopping…")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	a.srv.StopServers(ctx)
	systray.Quit()
}
