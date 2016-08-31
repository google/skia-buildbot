// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// +build darwin dragonfly freebsd linux netbsd openbsd

package streamclient

import (
	"fmt"
	"io"
	"net"
	"os"
)

// Register POSIX-only protocols.
func init() {
	registerProtocol("unix", newUnixSocketClient)
}

// newUnixSocketClient creates a new Client instance bound to a named pipe stream
// server.
//
// newNPipeClient currently only works on POSIX (non-Windows) systems.
func newUnixSocketClient(path string) (Client, error) {
	// Ensure that the supplied path exists and is a named pipe.
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file [%s]: %s", path, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return nil, fmt.Errorf("not a named pipe: [%s]", path)
	}

	return &clientImpl{func() (io.WriteCloser, error) {
		return net.Dial("unix", path)
	}}, nil
}
