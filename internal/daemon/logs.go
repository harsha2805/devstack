package daemon

import (
	"log"
	"os"
	"path/filepath"

	"github.com/harsha2805/devstack/internal/logfile"
	"github.com/harsha2805/devstack/internal/model"
)

// rotateLogs moves the logs nginx and PHP write themselves (site access logs,
// nginx-error.log, php-X.Y-error.log) to .1 once they pass the size the
// supervisor rotates its own logs at. It runs before the servers start, while
// nothing has the files open; the logs devstack writes rotate as they grow.
func rotateLogs(st *model.Stack, l *log.Logger) {
	paths := []string{filepath.Join(st.Logs, "nginx-error.log")}
	for _, p := range st.PHP {
		paths = append(paths, filepath.Join(st.Logs, p.Name+"-error.log"))
	}
	for _, s := range st.Sites {
		paths = append(paths, s.AccessLog)
	}
	for _, p := range paths {
		if err := rotateIfLarge(p, logfile.DefaultMaxSize); err != nil {
			l.Printf("rotating %s: %v", p, err)
		}
	}
}

func rotateIfLarge(path string, max int64) error {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() <= max {
		return nil
	}
	backup := path + ".1"
	os.Remove(backup)
	return os.Rename(path, backup)
}
