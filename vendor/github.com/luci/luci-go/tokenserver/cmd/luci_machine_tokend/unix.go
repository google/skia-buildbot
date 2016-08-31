// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// +build !windows

package main

import (
	"os"
	"syscall"
)

func interruptSignals() []os.Signal {
	return []os.Signal{
		os.Interrupt,
		syscall.SIGTERM,
	}
}

func atomicRename(source, target string) error {
	return os.Rename(source, target)
}
