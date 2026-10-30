package control

import (
	"context"
	"fmt"
	"strings"

	"github.com/harsha2805/devstack/internal/model"
	"github.com/harsha2805/devstack/internal/phpini"
	"github.com/harsha2805/devstack/internal/render"
	"github.com/harsha2805/devstack/internal/stack"
)

// CustomPHPINI is the path of the php.ini for PHP version key ("8.3"), the
// whole file with every extension listed, created the first time. A running
// stack reloads when it's saved.
func CustomPHPINI(ctx context.Context, root, key string) (string, error) {
	l, err := stack.Load(ctx, root)
	if l == nil || l.Stack == nil {
		return "", err
	}
	var have []string
	for _, p := range l.Stack.PHP {
		if p.Key != key {
			have = append(have, p.Key)
			continue
		}
		return p.CustomINI, writeUserINI(p)
	}
	return "", fmt.Errorf("devstack doesn't run PHP %s (it runs %s)", key, strings.Join(have, ", "))
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
