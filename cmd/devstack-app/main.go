// Command devstack-app is the desktop app: a tray icon plus a dashboard
// window. It is built as a GUI program (no console):
//
//	go build -ldflags "-H windowsgui" -o dist\devstack-app.exe .\cmd\devstack-app
//
// It also runs the background daemon when started as `devstack-app --root X daemon`,
// so it works without devstack.exe next to it.
package main

//go:generate go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --out rsrc

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/harsha2805/devstack/internal/appconf"
	"github.com/harsha2805/devstack/internal/daemon"
	"github.com/harsha2805/devstack/internal/logfile"
	"github.com/harsha2805/devstack/internal/packages"
	"github.com/harsha2805/devstack/internal/ui"
	"github.com/harsha2805/devstack/internal/winproc"
)

var version = "dev"

func main() {
	packages.UserAgent = "devstack/" + version
	args := os.Args[1:]

	// Daemon mode, used by Start: devstack-app --root <root> daemon
	if len(args) == 3 && args[0] == "--root" && args[2] == "daemon" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		if err := daemon.Run(ctx, daemon.Options{Root: args[1], Version: version}); err != nil {
			os.Exit(1)
		}
		return
	}
	trayOnly := len(args) > 0 && args[0] == "--tray"

	dir, err := appconf.Dir()
	if err != nil {
		fatal(err)
	}
	os.MkdirAll(dir, 0o755)
	if lw, err := logfile.Open(filepath.Join(dir, "app.log")); err == nil {
		log.SetOutput(lw)
	}

	// One app per user. A second launch asks the first to show its window.
	lock, err := winproc.LockFile(filepath.Join(dir, "app.lock"))
	if err != nil {
		if err := showRunning(dir); err != nil {
			log.Printf("second instance: %v", err)
		}
		return
	}
	defer lock.Close()

	exe, _ := os.Executable()
	srv := &ui.Server{Version: version, Exe: exe}
	addr, err := srv.Listen()
	if err != nil {
		fatal(err)
	}
	writeInstance(dir, addr, srv.Token())
	log.Printf("devstack-app %s listening on %s", version, addr)

	win := &window{url: srv.URL(), dataDir: filepath.Join(dir, "webview2")}
	srv.OnShow = win.Show
	app := &trayApp{srv: srv, win: win}
	srv.OnQuit = app.quit

	if !trayOnly || srv.Root() == "" {
		win.Show()
	}
	if srv.Root() != "" && srv.Settings().StartServers {
		srv.StartServers()
	}
	app.run() // blocks until Quit
}

type instance struct {
	Addr  string `json:"addr"`
	Token string `json:"token"`
}

func writeInstance(dir, addr, token string) {
	b, _ := json.Marshal(instance{addr, token})
	os.WriteFile(filepath.Join(dir, "app-instance.json"), b, 0o600)
}

func showRunning(dir string) error {
	b, err := os.ReadFile(filepath.Join(dir, "app-instance.json"))
	if err != nil {
		return err
	}
	var in instance
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	req, _ := http.NewRequest(http.MethodPost, "http://"+in.Addr+"/api/show", bytes.NewReader([]byte("{}")))
	req.Header.Set("X-Token", in.Token)
	req.Header.Set("X-Devstack", "1")
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}
	return nil
}

func fatal(err error) {
	log.Print(err)
	winproc.MessageBox("devstack", "devstack could not start:\n\n"+err.Error())
	os.Exit(1)
}
