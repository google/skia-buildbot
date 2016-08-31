// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package module

import (
	"net/http"

	"github.com/luci/luci-go/grpc/prpc"
	registrationPb "github.com/luci/luci-go/logdog/api/endpoints/coordinator/registration/v1"
	servicesPb "github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	"github.com/luci/luci-go/logdog/appengine/coordinator/endpoints/registration"
	"github.com/luci/luci-go/logdog/appengine/coordinator/endpoints/services"
	"github.com/luci/luci-go/server/router"

	// Include mutations package so its Mutations will register with tumble via
	// init().
	_ "github.com/luci/luci-go/logdog/appengine/coordinator/mutations"
)

// Run installs and executes this site.
func init() {
	r := router.New()

	// Setup Cloud Endpoints.
	svr := prpc.Server{}
	servicesPb.RegisterServicesServer(&svr, services.New())
	registrationPb.RegisterRegistrationServer(&svr, registration.New())

	// Standard HTTP endpoints.
	svr.InstallHandlers(r, coordinator.ProdServices())

	http.Handle("/", r)
}
