// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package buildbot

import (
	"github.com/luci/luci-go/grpc/discovery"
	"github.com/luci/luci-go/grpc/prpc"
	"github.com/luci/luci-go/server/router"

	"github.com/luci/luci-go/grpc/prpc/talk/buildbot/proto"
)

func InstallAPIRoutes(r *router.Router, base router.MiddlewareChain) {
	server := &prpc.Server{}
	buildbot.RegisterBuildbotServer(server, &buildbotService{})
	discovery.Enable(server)
	server.InstallHandlers(r, base)
}
