package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/harsha2805/devstack/internal/config"
	"github.com/harsha2805/devstack/internal/control"
	"github.com/harsha2805/devstack/internal/discover"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/stack"
)

// mailView is Mailpit on the Mail page and in the sidebar.
type mailView struct {
	Enabled bool `json:"enabled"`
	// Installed is the version Turn on would start, when Mailpit is off ("" when none is installed).
	Installed string `json:"installed,omitempty"`
	Version   string `json:"version,omitempty"`
	SMTPPort  int    `json:"smtp_port,omitempty"`
	Inbox     string `json:"inbox,omitempty"`
	// Counted: Mailpit answered, so Unread and Total are real.
	Counted bool `json:"counted"`
	Unread  int  `json:"unread"`
	Total   int  `json:"total"`
}

// siteMailView is how a site's .env sends mail.
type siteMailView struct {
	Mailer string `json:"mailer,omitempty"` // MAIL_MAILER
	Host   string `json:"host,omitempty"`
	Port   string `json:"port,omitempty"`
	// Ours: the .env already sends to devstack's Mailpit.
	Ours bool `json:"ours"`
}

// mailState is what the app shows about Mailpit: the installed version while
// it's off, and the inbox counts while it runs.
func mailState(ctx context.Context, st *model.Stack, running bool) *mailView {
	m := st.Mail
	if m == nil {
		v := &mailView{}
		if cfg, err := config.Load(st.Root); err == nil {
			if b, err := discover.Scan(ctx, stack.BinRoots(cfg)).Pick(discover.Mailpit, ""); err == nil {
				v.Installed = b.Version.String()
			}
		}
		return v
	}
	v := &mailView{Enabled: true, Version: m.Build.Version.String(), SMTPPort: m.SMTPPort, Inbox: m.InboxURL()}
	if running {
		var sum struct {
			Total  int `json:"total"`
			Unread int `json:"unread"`
		}
		if mailpitGet(ctx, m, "/api/v1/messages?limit=1", &sum) == nil {
			v.Counted, v.Total, v.Unread = true, sum.Total, sum.Unread
		}
	}
	return v
}

// mailpitClient talks to Mailpit's API on 127.0.0.1; the state poll can't wait long.
var mailpitClient = &http.Client{Timeout: 700 * time.Millisecond}

func mailpitGet(ctx context.Context, m *model.Mail, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.InboxURL()+path, nil)
	if err != nil {
		return err
	}
	resp, err := mailpitClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mailpit: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type mailAddress struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type mailMessage struct {
	ID      string        `json:"id"`
	Read    bool          `json:"read"`
	From    *mailAddress  `json:"from,omitempty"`
	To      []mailAddress `json:"to"`
	Subject string        `json:"subject"`
	Snippet string        `json:"snippet"`
	Created time.Time     `json:"created"`
	URL     string        `json:"url"` // the message in Mailpit's inbox
}

// handleMailMessages lists the newest caught messages for the Mail page.
func (s *Server) handleMailMessages(w http.ResponseWriter, r *http.Request) {
	l, err := stack.Load(r.Context(), s.root())
	if err != nil || l.Stack == nil || l.Stack.Mail == nil {
		reply(w, []mailMessage{})
		return
	}
	m := l.Stack.Mail
	var list struct {
		Messages []struct {
			ID      string        `json:"ID"`
			Read    bool          `json:"Read"`
			From    *mailAddress  `json:"From"`
			To      []mailAddress `json:"To"`
			Subject string        `json:"Subject"`
			Snippet string        `json:"Snippet"`
			Created time.Time     `json:"Created"`
		} `json:"messages"`
	}
	if err := mailpitGet(r.Context(), m, "/api/v1/messages?limit=8", &list); err != nil {
		replyErr(w, http.StatusBadGateway, errors.New("Mailpit isn't answering; start the servers"))
		return
	}
	out := []mailMessage{}
	for _, x := range list.Messages {
		if x.To == nil {
			x.To = []mailAddress{}
		}
		out = append(out, mailMessage{ID: x.ID, Read: x.Read, From: x.From, To: x.To, Subject: x.Subject,
			Snippet: x.Snippet, Created: x.Created, URL: m.InboxURL() + "/view/" + x.ID})
	}
	reply(w, out)
}

// handleMail turns Mailpit on or off.
func (s *Server) handleMail(w http.ResponseWriter, r *http.Request) {
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
	if err := control.SetMail(r.Context(), root, req.On); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	reply(w, nil)
}

// handleSiteMail points a site's .env at Mailpit.
func (s *Server) handleSiteMail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := control.SiteMail(r.Context(), s.root(), req.Slug)
	if err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	reply(w, res)
}
