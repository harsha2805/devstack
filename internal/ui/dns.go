package ui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/harsha2805/devstack/internal/appconf"
	"github.com/harsha2805/devstack/internal/control"
)

type dnsView struct {
	control.DNSStatus
	Asked bool `json:"asked"` // the one-time question was answered
}

// handleDNS turns <site>.<tld> addresses on or off. It runs as a job because
// it waits for the user to answer the administrator prompt.
func (s *Server) handleDNS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		On bool `json:"on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	root := s.root()
	if root == "" {
		replyErr(w, http.StatusConflict, errors.New("devstack is not set up yet"))
		return
	}
	title := "Turning off .test addresses"
	if req.On {
		title = "Turning on .test addresses"
	}
	s.startJob(w, title, func(ctx context.Context, j *Job) error {
		s.step(j, "Waiting for administrator approval", "running", 0, 0)
		if err := control.SetDNS(ctx, root, req.On); err != nil {
			return err
		}
		s.step(j, "Waiting for administrator approval", "done", 0, 0)
		s.markDNSAsked()
		return nil
	})
}

func (s *Server) handleDNSDismiss(w http.ResponseWriter, r *http.Request) {
	s.markDNSAsked()
	reply(w, nil)
}

func (s *Server) markDNSAsked() {
	s.mu.Lock()
	s.settings.DNSAsked = true
	set := s.settings
	s.mu.Unlock()
	appconf.Save(set)
}
