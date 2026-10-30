package ui

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/harsha2805/devstack/internal/control"
)

// handlePHPINI returns a PHP version's php.ini for the settings editor.
func (s *Server) handlePHPINI(w http.ResponseWriter, r *http.Request) {
	root := s.root()
	if root == "" {
		replyErr(w, http.StatusConflict, errors.New("devstack is not set up yet"))
		return
	}
	v, err := control.ReadPHPSettings(r.Context(), root, r.URL.Query().Get("php"))
	if err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	reply(w, v)
}

// handlePHPINISave saves the settings editor's changes to a PHP version's
// php.ini and applies them: a running stack restarts that PHP version now,
// instead of in a few seconds when the daemon would notice the file.
func (s *Server) handlePHPINISave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PHP string `json:"php"`
		control.PHPChange
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
	rev, err := control.SavePHPSettings(r.Context(), root, req.PHP, req.PHPChange)
	if errors.Is(err, control.ErrPHPINIChanged) {
		replyErr(w, http.StatusConflict, err)
		return
	}
	if err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	out := struct {
		Rev     string   `json:"rev"`
		Running bool     `json:"running"`
		Actions []string `json:"actions"`
		Error   string   `json:"error,omitempty"` // saved, but applying it failed
	}{Rev: rev}
	out.Running, out.Actions, err = control.ApplyPHPINI(r.Context(), root)
	if err != nil {
		out.Error = err.Error()
	}
	reply(w, out)
}
