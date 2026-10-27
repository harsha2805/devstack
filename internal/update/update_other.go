//go:build !windows

package update

import "errors"

func Installed(string) bool { return false }

func Launch(Info, string) error { return errors.New("updates are only supported on Windows") }
