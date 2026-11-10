package services

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/supervisor"
)

// MailpitSpec runs Mailpit: it takes mail on the SMTP port and keeps it for
// the inbox page instead of delivering it. Both listen on 127.0.0.1 only.
func MailpitSpec(st *model.Stack, logs *Logs) (supervisor.Spec, error) {
	m := st.Mail
	w, err := logs.Get("mailpit")
	if err != nil {
		return supervisor.Spec{}, err
	}
	args := MailpitArgs(m)
	return supervisor.Spec{
		Name: "mailpit", Group: "mailpit", Version: m.Build.Version.String(), Ports: []int{m.SMTPPort, m.InboxPort},
		Path: m.Build.Exe, Args: args, Dir: m.Build.Dir, Log: w,
		Before: func(context.Context) error { return os.MkdirAll(filepath.Dir(m.DB), 0o755) },
		Ready: func(ctx context.Context) error {
			if err := tcpReady(m.SMTPPort)(ctx); err != nil {
				return err
			}
			return tcpReady(m.InboxPort)(ctx)
		},
		StopTimeout: 5 * time.Second,
		CmdKey:      key(m.Build.Exe, args),
	}, nil
}

// MailpitArgs is Mailpit's command line.
func MailpitArgs(m *model.Mail) []string {
	inbox := "127.0.0.1:" + strconv.Itoa(m.InboxPort)
	return []string{
		"--smtp", "127.0.0.1:" + strconv.Itoa(m.SMTPPort),
		"--listen", inbox,
		"--database", m.DB,
		// Sites keep whatever MAIL_USERNAME and MAIL_PASSWORD they have.
		"--smtp-auth-accept-any", "--smtp-auth-allow-insecure",
		// Only the inbox's own address: a web page can't reach it through DNS rebinding.
		"--allowed-hosts", strings.Join([]string{inbox, "localhost:" + strconv.Itoa(m.InboxPort)}, ","),
		"--disable-version-check",
		"--label", "devstack",
	}
}
