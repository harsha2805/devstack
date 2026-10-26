// Package logfile is an append-only log writer that keeps one backup
// (<name>.1) once the file passes a size limit.
package logfile

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DefaultMaxSize is the rotate threshold used by Open.
const DefaultMaxSize = 10 << 20

type Writer struct {
	mu      sync.Mutex
	path    string
	maxSize int64
	f       *os.File
	size    int64
}

var _ io.WriteCloser = (*Writer)(nil)

// Open appends to path, creating it and its folder if needed.
func Open(path string) (*Writer, error) {
	w := &Writer{path: path, maxSize: DefaultMaxSize}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.f, w.size = f, st.Size()
	return nil
}

func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return 0, os.ErrClosed
	}
	if w.size+int64(len(p)) > w.maxSize {
		w.rotate()
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

// rotate moves the file to .1. If that fails (another program has it open
// without delete sharing), it keeps writing to the current file.
func (w *Writer) rotate() {
	w.f.Close()
	backup := w.path + ".1"
	os.Remove(backup)
	os.Rename(w.path, backup)
	if err := w.open(); err != nil {
		w.f = nil
	}
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

// Tail returns the last n lines of a file, or "" if it can't be read.
// It reads at most the final 256 KB, so it is cheap on large logs.
func Tail(path string, n int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return ""
	}
	const window = 256 << 10
	off := max(st.Size()-window, 0)
	buf := make([]byte, st.Size()-off)
	if _, err := f.ReadAt(buf, off); err != nil && err != io.EOF {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(string(buf), "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if off > 0 && len(lines) > 0 {
		lines = lines[1:] // the first line is probably partial
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
