// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gaemiddleware

import (
	"github.com/luci/luci-go/server/router"
	"github.com/luci/luci-go/server/settings/admin"

	gaeauth "github.com/luci/luci-go/appengine/gaeauth/server"
	"github.com/luci/luci-go/appengine/tsmon"
)

// InstallHandlers installs HTTP handlers for various default routes.
//
// These routes are needed for various services provided in BaseProd context to
// work (e.g. authentication related routes, time series monitoring, etc).
//
// 'base' is expected to be BaseProd or its derivative.
func InstallHandlers(r *router.Router, base router.MiddlewareChain) {
	gaeauth.InstallHandlers(r, base)
	tsmon.InstallHandlers(r, base)
	admin.InstallHandlers(r, base, &gaeauth.UsersAPIAuthMethod{})
}
