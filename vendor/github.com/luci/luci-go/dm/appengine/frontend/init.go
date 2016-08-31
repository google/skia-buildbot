// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package frontend

import (
	"net/http"

	"github.com/luci/luci-go/appengine/gaemiddleware"
	"github.com/luci/luci-go/dm/appengine/deps"
	"github.com/luci/luci-go/dm/appengine/distributor"
	"github.com/luci/luci-go/dm/appengine/distributor/jobsim"
	"github.com/luci/luci-go/dm/appengine/distributor/swarming/v1"
	"github.com/luci/luci-go/dm/appengine/mutate"
	"github.com/luci/luci-go/grpc/discovery"
	"github.com/luci/luci-go/grpc/prpc"
	"github.com/luci/luci-go/server/router"
	"github.com/luci/luci-go/tumble"
)

func init() {
	tmb := tumble.Service{}

	distributors := distributor.FactoryMap{}
	jobsim.AddFactory(distributors)
	swarming.AddFactory(distributors)

	reg := distributor.NewRegistry(distributors, mutate.FinishExecutionFn)

	basemw := gaemiddleware.BaseProd().Extend(func(c *router.Context, next router.Handler) {
		c.Context = distributor.WithRegistry(c.Context, reg)
		next(c)
	})

	r := router.New()

	svr := prpc.Server{}
	deps.RegisterDepsServer(&svr)
	discovery.Enable(&svr)

	distributor.InstallHandlers(r, basemw)
	svr.InstallHandlers(r, basemw)
	tmb.InstallHandlers(r, basemw)
	gaemiddleware.InstallHandlers(r, basemw)

	http.Handle("/", r)
}
