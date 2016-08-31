// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package helloworld

import (
	"github.com/luci/luci-go/grpc/discovery"
	"github.com/luci/luci-go/grpc/prpc"
	"github.com/luci/luci-go/server/router"

	"github.com/luci/luci-go/grpc/prpc/talk/helloworld/proto"
)

func InstallAPIRoutes(r *router.Router, base router.MiddlewareChain) {
	server := &prpc.Server{}
	helloworld.RegisterGreeterServer(server, &greeterService{})
	discovery.Enable(server)
	server.InstallHandlers(r, base)
}
