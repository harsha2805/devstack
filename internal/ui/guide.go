package ui

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/harsha2805/devstack/internal/aiguide"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/stack"
)

// handleSiteGuide adds or removes a site's AI guide (see package aiguide).
func (s *Server) handleSiteGuide(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug string `json:"slug"`
		On   bool   `json:"on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	l, err := stack.Load(r.Context(), s.root())
	if err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	var site *model.Site
	for i := range l.Stack.Sites {
		if l.Stack.Sites[i].Slug == req.Slug {
			site = &l.Stack.Sites[i]
		}
	}
	if site == nil {
		replyErr(w, http.StatusNotFound, fmt.Errorf("no site %q", req.Slug))
		return
	}
	if req.On {
		err = aiguide.Install(l.Stack, *site, aiguide.CLI())
	} else {
		err = aiguide.Remove(site.Dir)
	}
	if err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	reply(w, nil)
}

// syncGuides refreshes AI guides after a change made while the servers are
// stopped (the daemon does it on every start and reload).
func (s *Server) syncGuides(r *http.Request) {
	l, err := stack.Load(r.Context(), s.root())
	if err != nil || l.Stack == nil {
		return
	}
	if _, err := aiguide.Sync(l.Stack, aiguide.CLI()); err != nil {
		log.Printf("AI guide: %v", err)
	}
}
