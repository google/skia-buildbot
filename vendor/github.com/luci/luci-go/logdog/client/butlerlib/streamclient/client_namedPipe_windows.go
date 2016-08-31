// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamclient

import (
	"errors"
	"fmt"
	"io"

	npipe "gopkg.in/natefinch/npipe.v2"
)

// Register POSIX-only protocols.
func init() {
	registerProtocol("net.pipe", newNamedPipeClient)
}

// newNamedPipeClient creates a new Client instance bound to a named pipe stream
// server.
func newNamedPipeClient(path string) (Client, error) {
	if path == "" {
		return nil, errors.New("streamclient: cannot have empty named pipe path")
	}

	return &clientImpl{func() (io.WriteCloser, error) {
		return npipe.Dial(fmt.Sprintf(`\\.\pipe\%s`, path))
	}}, nil
}
