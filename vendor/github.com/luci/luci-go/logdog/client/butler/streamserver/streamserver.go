// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamserver

import (
	"io"

	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"
)

// StreamServer is an interface to a backgound service that allows external
// processes to establish Butler streams.
type StreamServer interface {
	// Performs initial connection and setup, entering a listening state.
	Listen() error
	// Blocks, returning a new Stream when one is available. If the stream server
	// has closed, this will return nil.
	Next() (io.ReadCloser, *streamproto.Properties)
	// Closes the stream server, cleaning up resources.
	Close()
}
