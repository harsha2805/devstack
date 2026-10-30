package control

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/harsha2805/devstack/internal/stack"
)

// CustomPHPINI is the path of the user's own php.ini for PHP version key
// ("8.3"), created with a short how-to the first time. devstack adds it to the
// end of the generated php.ini, and a running stack reloads when it's saved.
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
		if _, err := os.Stat(p.CustomINI); err == nil {
			return p.CustomINI, nil
		}
		if err := os.MkdirAll(filepath.Dir(p.CustomINI), 0o755); err != nil {
			return "", err
		}
		return p.CustomINI, os.WriteFile(p.CustomINI, []byte(strings.ReplaceAll(customINITemplate(key), "\n", "\r\n")), 0o644)
	}
	return "", fmt.Errorf("devstack doesn't run PHP %s (it runs %s)", key, strings.Join(have, ", "))
}

func customINITemplate(key string) string {
	return `; Your php.ini settings for PHP ` + key + `.
;
; devstack adds this file to the end of the php.ini it generates, so a setting
; here wins over the default. Save it and PHP ` + key + ` restarts with it within
; a few seconds while the servers run. Every site on PHP ` + key + ` gets it, and so
; do php and composer in their terminals.
;
; For one site only, put a .user.ini file in that site's docroot (public\ for
; Laravel) instead. For every PHP version, use [php.ini] in stack.toml.
;
; Remove the ; in front of a line to use it.

;memory_limit = 1G
;upload_max_filesize = 512M
;post_max_size = 512M
;max_execution_time = 300
;date.timezone = "Asia/Kolkata"
`
}
