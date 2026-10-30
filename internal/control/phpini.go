package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/harsha2805/devstack/internal/api"
	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/phpini"
	"github.com/harsha2805/devstack/internal/render"
	"github.com/harsha2805/devstack/internal/stack"
)

// CustomPHPINI is the path of the php.ini for PHP version key ("8.3"), the
// whole file with every extension listed, created the first time. A running
// stack reloads when it's saved.
func CustomPHPINI(ctx context.Context, root, key string) (string, error) {
	p, err := phpPool(ctx, root, key)
	if err != nil {
		return "", err
	}
	return p.CustomINI, writeUserINI(p)
}

// phpPool is the PHP version key that root runs.
func phpPool(ctx context.Context, root, key string) (*model.PHPPool, error) {
	l, err := stack.Load(ctx, root)
	if l == nil || l.Stack == nil {
		return nil, err
	}
	var have []string
	for _, p := range l.Stack.PHP {
		if p.Key == key {
			return p, nil
		}
		have = append(have, p.Key)
	}
	return nil, fmt.Errorf("devstack doesn't run PHP %s (it runs %s)", key, strings.Join(have, ", "))
}

func writeUserINI(p *model.PHPPool) error {
	text, fresh := render.UserINI(p)
	if !fresh {
		return nil
	}
	return render.WriteUserINI(p, text)
}

// setExtensionINIs turns the named extensions on or off in the php.ini of
// every PHP version root runs, so the app's switches and `devstack ext` do
// what an edit by hand would. A version whose ext folder lacks the DLL is
// left alone when turning on.
func setExtensionINIs(ctx context.Context, root string, names []string, on bool) error {
	l, err := stack.Load(ctx, root)
	if l == nil || l.Stack == nil {
		return err
	}
	for _, p := range l.Stack.PHP {
		text, fresh := render.UserINI(p)
		was := text
		for _, n := range names {
			if !on {
				text = phpini.Disable(text, func(e string) bool { return model.ExtensionBase(e) == n })
				continue
			}
			if f, ok := model.ExtensionFile(p.Build, n); ok {
				text = phpini.Disable(text, func(e string) bool { return e != f && model.ExtensionBase(e) == n }) // one DLL per extension
				text = phpini.Enable(text, f, false)
			}
		}
		if fresh || text != was {
			if err := render.WriteUserINI(p, text); err != nil {
				return err
			}
		}
	}
	return nil
}

// PHPSettings is a PHP version's php.ini as the app's settings editor shows it.
type PHPSettings struct {
	PHP        string            `json:"php"`
	Version    string            `json:"version"`
	Path       string            `json:"path"`
	Rev        string            `json:"rev"`        // changes whenever the file does; Save refuses a stale one
	Settings   map[string]string `json:"settings"`   // the active key = value lines, the last one per key
	Order      []string          `json:"order"`      // Settings' keys in file order
	Stack      map[string]string `json:"stack"`      // stack.toml [php.ini], which wins over the file
	Owned      []string          `json:"owned"`      // set by devstack after the file
	Extensions []phpini.Ext      `json:"extensions"` // every extension= line, on or off
}

// ReadPHPSettings reads the php.ini of PHP version key, creating it first.
func ReadPHPSettings(ctx context.Context, root, key string) (PHPSettings, error) {
	p, err := phpPool(ctx, root, key)
	if err != nil {
		return PHPSettings{}, err
	}
	if err := writeUserINI(p); err != nil {
		return PHPSettings{}, err
	}
	text := model.UserINI(p)
	v := PHPSettings{PHP: p.Key, Version: p.Build.Version.String(), Path: p.CustomINI, Rev: rev(text),
		Settings: map[string]string{}, Order: []string{}, Stack: p.INI, Owned: render.Owned,
		Extensions: phpini.ExtensionStates(text)}
	if v.Stack == nil {
		v.Stack = map[string]string{}
	}
	if v.Extensions == nil {
		v.Extensions = []phpini.Ext{}
	}
	for _, kv := range phpini.Settings(text) {
		if _, ok := v.Settings[kv[0]]; !ok {
			v.Order = append(v.Order, kv[0])
		}
		v.Settings[kv[0]] = kv[1]
	}
	return v, nil
}

func rev(text string) string {
	sum := sha256.Sum256([]byte(strings.ReplaceAll(text, "\r\n", "\n")))
	return hex.EncodeToString(sum[:8])
}

// PHPChange is an edit from the settings editor.
type PHPChange struct {
	Rev        string            `json:"rev"`        // the file as the editor read it
	Set        map[string]string `json:"set"`        // key = value lines to write
	Unset      []string          `json:"unset"`      // keys to comment out, back to PHP's default
	Extensions map[string]bool   `json:"extensions"` // listed extensions to turn on or off
}

// ErrPHPINIChanged means the file changed on disk since the editor read it,
// most likely in a text editor.
var ErrPHPINIChanged = errors.New("php.ini was changed somewhere else since it was opened here")

var iniKey = regexp.MustCompile(`^[A-Za-z0-9_.\-]+$`)

// SavePHPSettings applies ch to the php.ini of PHP version key, editing its
// lines in place so comments and the rest of the file stay as they are, and
// returns the new rev. It doesn't reload; see ApplyPHPINI.
func SavePHPSettings(ctx context.Context, root, key string, ch PHPChange) (string, error) {
	p, err := phpPool(ctx, root, key)
	if err != nil {
		return "", err
	}
	if err := writeUserINI(p); err != nil {
		return "", err
	}
	text := model.UserINI(p)
	if ch.Rev != rev(text) {
		return "", ErrPHPINIChanged
	}
	check := func(k string) error {
		switch {
		case !iniKey.MatchString(k):
			return fmt.Errorf("%q isn't a php.ini setting name (letters, digits, _ . and - only)", k)
		case strings.EqualFold(k, "extension") || strings.EqualFold(k, "zend_extension"):
			return errors.New("turn extensions on and off in the Extensions list")
		case slices.ContainsFunc(render.Owned, func(o string) bool { return strings.EqualFold(o, k) }):
			return fmt.Errorf("devstack sets %s itself, because it points into devstack's folders", k)
		}
		return nil
	}
	for _, k := range ch.Unset {
		if err := check(k); err != nil {
			return "", err
		}
		text = phpini.Unset(text, k)
	}
	keys := make([]string, 0, len(ch.Set))
	for k := range ch.Set {
		keys = append(keys, k)
	}
	slices.Sort(keys) // settings new to the file are added in a stable order
	for _, k := range keys {
		v := strings.TrimSpace(ch.Set[k])
		if err := check(k); err != nil {
			return "", err
		}
		if strings.ContainsAny(v, "\r\n") {
			return "", fmt.Errorf("%s: a value must fit on one line", k)
		}
		text = phpini.Set(text, k, v)
	}
	listed := phpini.ExtensionStates(text)
	for name, on := range ch.Extensions {
		if !slices.ContainsFunc(listed, func(e phpini.Ext) bool { return e.Name == name }) {
			return "", fmt.Errorf("PHP %s's php.ini doesn't list the %s extension", key, name)
		}
		if on {
			text = phpini.Enable(text, name, false)
		} else {
			text = phpini.Disable(text, func(n string) bool { return n == name })
		}
	}
	if err := render.WriteUserINI(p, text); err != nil {
		return "", err
	}
	return rev(model.UserINI(p)), nil
}

// ApplyPHPINI makes a saved php.ini take effect. A running stack reloads,
// which restarts the PHP versions whose php.ini changed; a stopped one only
// regenerates etc/, so php in terminals uses it now and the servers get it
// when they start.
func ApplyPHPINI(ctx context.Context, root string) (running bool, actions []string, err error) {
	c, err := api.Connect(root)
	if errors.Is(err, api.ErrNotRunning) {
		l, err := stack.Load(ctx, root)
		if err != nil {
			return false, nil, err
		}
		_, err = render.Write(l.Stack)
		return false, nil, err
	}
	if err != nil {
		return false, nil, err
	}
	res, err := c.Reload(ctx)
	return true, res.Actions, err
}
