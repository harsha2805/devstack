//go:build !windows

package dns

import "errors"

const RuleComment = "devstack"

func RuleInstalled(string) bool { return false }

func SetRule(bool, string) error { return errors.New("only supported on Windows") }
