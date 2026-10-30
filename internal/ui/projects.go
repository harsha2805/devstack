package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/harsha2805/devstack/internal/control"
	"github.com/harsha2805/devstack/internal/sites"
	"github.com/harsha2805/devstack/internal/winproc"
)

// projectView is a folder listed in stack.toml's [projects] that isn't being
// served, so the Sites tab can say why and offer to take it off the list.
type projectView struct {
	Name    string `json:"name"`
	Dir     string `json:"dir"`
	Missing bool   `json:"missing"` // the folder is gone (moved, or on a drive that's unplugged)
}

// handlePickFolder shows the Windows folder picker. An empty dir means the
// user cancelled.
func (s *Server) handlePickFolder(w http.ResponseWriter, r *http.Request) {
	dir, err := winproc.PickFolder("Choose a project folder to serve")
	if err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	reply(w, map[string]string{"dir": dir})
}

func (s *Server) handleProjectAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dir  string `json:"dir"`
		Name string `json:"name"`
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
	res, err := control.AddProject(r.Context(), root, req.Dir, req.Name)
	var nameErr *control.NameError
	if errors.As(err, &nameErr) {
		// The only case that needs the user: the dashboard asks for another name.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": nameErr.Msg, "suggestion": nameErr.Suggestion})
		return
	}
	if err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	reply(w, res)
}

func (s *Server) handleProjectRemove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
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
	if err := control.RemoveProject(r.Context(), root, req.Name); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	reply(w, nil)
}

// unserved lists the projects in [projects] that no site came from.
func unserved(projects []sites.Project, served []siteView) []projectView {
	out := []projectView{}
	for _, p := range projects {
		found := false
		for _, s := range served {
			found = found || (s.Project && s.Slug == p.Name)
		}
		if !found {
			st, err := os.Stat(p.Dir)
			out = append(out, projectView{Name: p.Name, Dir: p.Dir, Missing: err != nil || !st.IsDir()})
		}
	}
	return out
}
