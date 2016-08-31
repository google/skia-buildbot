// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be that can be found in the LICENSE file.

// Source: https://github.com/golang/crypto/blob/master/ssh/terminal/util_linux.go
// Revision: 30ad74476e5c37a080ab5999ce9ab4465a2ecfac
// License: BSD

package terminal

// These constants are declared here, rather than importing
// them from the syscall package as some syscall packages, even
// on linux, for example gccgo, do not declare them.
const ioctlReadTermios = 0x5401  // syscall.TCGETS
const ioctlWriteTermios = 0x5402 // syscall.TCSETS
