//go:build windows

package utils

import (
	"syscall"
)

const createNoWindow = 0x08000000

// askpassSysProcAttr hides the helper's console window.
func askpassSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
