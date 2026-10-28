package ui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/harsha2805/devstack/internal/control"
	"github.com/harsha2805/devstack/internal/shim"
)

// shimsView is whether php and composer in terminals run each site's PHP.
type shimsView struct {
	shim.PathStatus
	OK      bool   `json:"ok"`
	Problem string `json:"problem,omitempty"`
}

// handleTerminal opens a terminal in a site's folder, where php and
// composer run the site's PHP.
func (s *Server) handleTerminal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Site string `json:"site"`
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
	if err := control.OpenTerminal(root, req.Site); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	reply(w, nil)
}

// handleShimsFix puts devstack's php and composer first on PATH. It runs as a
// job because a PHP on the system PATH means an administrator prompt.
func (s *Server) handleShimsFix(w http.ResponseWriter, r *http.Request) {
	s.startJob(w, "Putting devstack's php first on PATH", func(ctx context.Context, j *Job) error {
		label := "Updating your PATH"
		if st, ok := control.ShimStatus(); ok && st.BlockerSystem {
			label = "Waiting for administrator approval"
		}
		s.step(j, label, "running", 0, 0)
		if err := control.FixShimPath(); err != nil {
			return err
		}
		s.step(j, label, "done", 0, 0)
		return nil
	})
}
