//go:build !windows

package certs

import "errors"

var ErrDeclined = errors.New("declined")

func (ca *CA) Trusted() bool { return false }
func (ca *CA) Trust() error  { return errors.New("only supported on Windows") }
func Untrust() (int, error)  { return 0, errors.New("only supported on Windows") }
