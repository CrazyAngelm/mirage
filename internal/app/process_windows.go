//go:build windows

package app

import "syscall"

func hiddenProcessAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: 0x08000000}
}
