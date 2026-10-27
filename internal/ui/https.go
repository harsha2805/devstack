package ui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/harsha2805/devstack/internal/control"
)

// handleHTTPS turns trusted HTTPS on or off. It runs as a job because it
// waits for the user to answer Windows' question about trusting the CA.
func (s *Server) handleHTTPS(w http.ResponseWriter, r *http.Request) {
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
	title, step := "Turning off HTTPS", "Removing the devstack certificate"
	if req.On {
		title, step = "Turning on HTTPS", "Waiting for you to trust the devstack certificate"
	}
	s.startJob(w, title, func(ctx context.Context, j *Job) error {
		s.step(j, step, "running", 0, 0)
		if err := control.SetHTTPS(ctx, root, req.On); err != nil {
			return err
		}
		s.step(j, step, "done", 0, 0)
		return nil
	})
}
