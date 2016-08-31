// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamserver

import (
	"net"

	log "github.com/luci/luci-go/common/logging"
	"golang.org/x/net/context"
	npipe "gopkg.in/natefinch/npipe.v2"
)

// NewNamedPipeServer instantiates a new Windows named pipe server instance.
func NewNamedPipeServer(ctx context.Context, address string) StreamServer {
	ctx = log.SetField(ctx, "address", address)
	return &listenerStreamServer{
		Context: ctx,
		gen: func() (net.Listener, error) {
			log.Debugf(ctx, "Creating Windows server socket Listener.")
			return npipe.Listen(address)
		},
	}
}
