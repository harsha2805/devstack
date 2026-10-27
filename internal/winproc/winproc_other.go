//go:build !windows

// Stubs so the rest of the code builds and unit-tests on other platforms.
// devstack itself only runs on Windows.
package winproc

import (
	"errors"
	"os"
	"os/exec"
)

var errUnsupported = errors.New("only supported on Windows")

func HideWindow(*exec.Cmd) {}

func StartDetached(cmd *exec.Cmd) (int, error) {
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	return pid, cmd.Process.Release()
}

type Job struct{}

func NewJob() (*Job, error)    { return nil, errUnsupported }
func (*Job) Assign(int) error  { return errUnsupported }
func (*Job) Terminate() error  { return errUnsupported }
func (*Job) Close() error      { return nil }
func Alive(pid int) bool       { return pid > 0 }
func ProcessName(int) string   { return "" }
func PortOwner(int) (int, error) { return 0, nil }

func LongPath(p string) string { return p }

func ShellOpen(string) error                  { return errUnsupported }
func MessageBox(string, string)               {}
func AllowForeground(int)                     {}
func Autostart(string) bool                    { return false }
func SetAutostart(string, string, bool) error  { return errUnsupported }

func FileVersion(string) ([4]int, error) { return [4]int{}, errUnsupported }
func VCRuntime() (string, bool)           { return "", true }

func LockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
}
