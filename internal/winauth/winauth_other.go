//go:build !windows

package winauth

import "errors"

// ErrCanceled means the user closed the prompt.
var ErrCanceled = errors.New("verification canceled")

// Verify is Windows-only; elsewhere it refuses.
func Verify(owner uintptr, message string) error {
	return errors.New("verifying the user needs Windows")
}
