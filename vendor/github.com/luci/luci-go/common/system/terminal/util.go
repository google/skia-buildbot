// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be that can be found in the LICENSE file.

// Source: https://github.com/golang/crypto/blob/master/ssh/terminal/util.go
// Revision: 30ad74476e5c37a080ab5999ce9ab4465a2ecfac
// License: BSD

// +build darwin dragonfly freebsd linux,!appengine netbsd openbsd

package terminal

import (
	"syscall"
	"unsafe"
)

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd int) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), ioctlReadTermios, uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	return err == 0
}
