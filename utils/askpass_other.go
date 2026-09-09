//go:build !windows

package utils

import (
	"syscall"
)

// askpassSysProcAttr has no extra attributes outside Windows.
func askpassSysProcAttr() *syscall.SysProcAttr {
	return nil
}
