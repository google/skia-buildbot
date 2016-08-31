// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// +build appengine

package terminal

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd int) bool {
	return false
}
