// Package api is the contract between the daemon and its clients (the CLI
// now, the tray UI later): the state file that says where the daemon
// listens, the JSON types, and a client.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/harsha2805/devstack/internal/supervisor"
	"github.com/harsha2805/devstack/internal/winproc"
)

// State is data/devstack/daemon.json, written by the daemon while it runs.
type State struct {
	PID     int       `json:"pid"`
	Addr    string    `json:"addr"`
	Token   string    `json:"token"`
	Started time.Time `json:"started"`
	Version string    `json:"version"`
}

func StatePath(root string) string { return filepath.Join(root, "data", "devstack", "daemon.json") }

// ErrNotRunning means there is no live daemon for this root.
var ErrNotRunning = errors.New("devstack is not running")

// ReadState returns the daemon state, cleaning up a file left by a dead daemon.
func ReadState(root string) (State, error) {
	var s State
	b, err := os.ReadFile(StatePath(root))
	if errors.Is(err, os.ErrNotExist) {
		return s, ErrNotRunning
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil || !winproc.Alive(s.PID) {
		os.Remove(StatePath(root))
		return s, ErrNotRunning
	}
	return s, nil
}

// Phase of the daemon as a whole.
const (
	PhaseStarting = "starting"
	PhaseRunning  = "running"
	PhaseStopping = "stopping"
)

type Status struct {
	PID      int                 `json:"pid"`
	Started  time.Time           `json:"started"`
	Version  string              `json:"version"`
	Root     string              `json:"root"`
	Phase    string              `json:"phase"`
	Errors   []string            `json:"errors,omitempty"`
	Warnings []string            `json:"warnings,omitempty"`
	Units    []supervisor.Status `json:"units"`
}

// Result is returned by actions (reload, restart).
type Result struct {
	Actions []string `json:"actions,omitempty"`
	Error   string   `json:"error,omitempty"`
}

type Client struct {
	State State
	http  *http.Client
}

// Connect finds the daemon for root.
func Connect(root string) (*Client, error) {
	s, err := ReadState(root)
	if err != nil {
		return nil, err
	}
	return &Client{State: s, http: &http.Client{Timeout: 5 * time.Minute}}, nil
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	var s Status
	err := c.do(ctx, http.MethodGet, "/v1/status", nil, &s)
	return s, err
}

func (c *Client) Reload(ctx context.Context) (Result, error) {
	var r Result
	err := c.do(ctx, http.MethodPost, "/v1/reload", nil, &r)
	return r, err
}

// Restart restarts a unit or group ("" = everything).
func (c *Client) Restart(ctx context.Context, unit string) (Result, error) {
	var r Result
	err := c.do(ctx, http.MethodPost, "/v1/restart", url.Values{"unit": {unit}}, &r)
	return r, err
}

func (c *Client) Shutdown(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/v1/shutdown", nil, nil)
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, out any) error {
	u := "http://" + c.State.Addr + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.State.Token)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("talk to daemon: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		var r Result
		if json.Unmarshal(body, &r) == nil && r.Error != "" {
			return errors.New(r.Error)
		}
		return fmt.Errorf("daemon: %s: %s", resp.Status, bytes.TrimSpace(body))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}
