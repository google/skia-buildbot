// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be that can be found in the LICENSE file.

// Source: https://github.com/golang/crypto/blob/master/ssh/terminal/util_windows.go
// Revision: 30ad74476e5c37a080ab5999ce9ab4465a2ecfac
// License: BSD

// +build windows

package terminal

import (
	"syscall"
	"unsafe"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var procGetConsoleMode = kernel32.NewProc("GetConsoleMode")

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd int) bool {
	var st uint32
	r, _, e := syscall.Syscall(procGetConsoleMode.Addr(), 2, uintptr(fd), uintptr(unsafe.Pointer(&st)), 0)
	return r != 0 && e == 0
}
