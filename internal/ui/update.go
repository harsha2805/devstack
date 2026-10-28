package ui

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/harsha2805/devstack/internal/api"
	"github.com/harsha2805/devstack/internal/appconf"
	"github.com/harsha2805/devstack/internal/control"
	"github.com/harsha2805/devstack/internal/update"
	"github.com/harsha2805/devstack/internal/winproc"
)

// updateJob is the job title for an in-app update.
const updateJob = "Updating devstack"

// updateState is what the app knows about newer releases. Guarded by Server.mu.
type updateState struct {
	available *update.Info
	checked   time.Time
	err       string
	checking  bool
}

type updateView struct {
	Release   bool   `json:"release"`   // false for development builds, which never update
	Installed bool   `json:"installed"` // installed copy; portable copies are sent to the download page
	Checking  bool   `json:"checking"`
	Checked   string `json:"checked,omitempty"`
	Error     string `json:"error,omitempty"`
	Version   string `json:"version,omitempty"` // the newer release, if any
	Notes     string `json:"notes,omitempty"`
	Size      int64  `json:"size,omitempty"` // the newer release's download size, if the server says
}

// updateViewLocked builds the dashboard's view; the caller holds s.mu.
func (s *Server) updateViewLocked() *updateView {
	v := &updateView{Release: update.Release(s.Version), Installed: update.Installed(s.Exe),
		Checking: s.upd.checking, Error: s.upd.err}
	if !s.upd.checked.IsZero() {
		v.Checked = s.upd.checked.Format(time.RFC3339)
	}
	if a := s.upd.available; a != nil {
		v.Version, v.Notes, v.Size = a.Version, a.Notes, a.Size
	}
	return v
}

// CheckForUpdates asks the release feed whether a newer version exists.
func (s *Server) CheckForUpdates(ctx context.Context) error {
	if !update.Release(s.Version) {
		return errors.New("this is a development build, which doesn't update")
	}
	s.mu.Lock()
	if s.upd.checking {
		s.mu.Unlock()
		return nil
	}
	s.upd.checking = true
	s.mu.Unlock()

	info, err := update.Check(ctx, s.Version)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.upd.checking = false
	s.upd.checked = time.Now()
	if err != nil {
		s.upd.err = err.Error()
		return err
	}
	s.upd.err = ""
	s.upd.available = info
	return nil
}

// AutoCheckUpdates checks shortly after launch, then once a day, while the
// "check for updates" setting is on. Failures only show in Settings.
func (s *Server) AutoCheckUpdates() {
	if !update.Release(s.Version) {
		return
	}
	go func() {
		time.Sleep(30 * time.Second)
		for {
			if s.Settings().CheckUpdates {
				if err := s.CheckForUpdates(context.Background()); err != nil {
					log.Printf("update check: %v", err)
				}
			}
			time.Sleep(24 * time.Hour)
		}
	}()
}

// ResumeAfterUpdate reports, once, whether an in-app update stopped the
// servers, so the new version starts them again.
func (s *Server) ResumeAfterUpdate() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.settings.ResumeServers {
		return false
	}
	s.settings.ResumeServers = false
	appconf.Save(s.settings)
	return true
}

func (s *Server) setResume(on bool) {
	s.mu.Lock()
	s.settings.ResumeServers = on
	set := s.settings
	s.mu.Unlock()
	appconf.Save(set)
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if err := s.CheckForUpdates(r.Context()); err != nil {
		replyErr(w, http.StatusBadGateway, err)
		return
	}
	s.mu.Lock()
	v := s.updateViewLocked()
	s.mu.Unlock()
	reply(w, v)
}

// handleUpdateInstall downloads and runs the new installer. A portable copy
// opens the release page instead, since there is no install to upgrade.
func (s *Server) handleUpdateInstall(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	info := s.upd.available
	s.mu.Unlock()
	if info == nil {
		replyErr(w, http.StatusBadRequest, errors.New("no update is available"))
		return
	}
	if !update.Installed(s.Exe) {
		page := info.Page
		if page == "" {
			page = info.URL
		}
		if err := winproc.ShellOpen(page); err != nil {
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
		reply(w, map[string]bool{"opened": true})
		return
	}
	i := *info
	s.startJob(w, updateJob, func(ctx context.Context, j *Job) error { return s.runUpdate(ctx, j, i) })
}

// runUpdate downloads the installer, stops the servers (the daemon runs from
// this exe, which the installer replaces), starts the installer and quits.
// The new version starts the servers again.
func (s *Server) runUpdate(ctx context.Context, j *Job, i update.Info) error {
	label := "Downloading devstack " + i.Version
	s.step(j, label, "running", 0, 0)
	path, err := update.Download(ctx, i, filepath.Join(os.TempDir(), "devstack-update"), func(done, total int64) {
		s.step(j, label, "running", done, total)
	})
	if err != nil {
		return err
	}
	s.step(j, label, "done", 0, 0)

	stopped := false
	if root := s.root(); root != "" {
		if _, err := api.Connect(root); err == nil {
			s.step(j, "Stopping servers", "running", 0, 0)
			if err := control.Down(ctx, root, nil); err != nil {
				return err
			}
			s.step(j, "Stopping servers", "done", 0, 0)
			stopped = true
			s.setResume(true)
		}
	}

	s.step(j, "Starting the installer", "running", 0, 0)
	if err := update.Launch(i, path); err != nil {
		if stopped {
			s.setResume(false)
			s.runStart(ctx, j)
		}
		return err
	}
	s.step(j, "Starting the installer", "done", 0, 0)
	if s.OnQuit != nil {
		go s.OnQuit()
	}
	return nil
}
