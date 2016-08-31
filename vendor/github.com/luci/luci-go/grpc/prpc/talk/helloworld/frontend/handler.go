// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package helloworld

import (
	"net/http"

	"github.com/luci/luci-go/appengine/gaemiddleware"
	"github.com/luci/luci-go/server/router"
)

func init() {
	r := router.New()
	basemw := gaemiddleware.BaseProd()

	gaemiddleware.InstallHandlers(r, basemw)
	InstallAPIRoutes(r, basemw)
	http.DefaultServeMux.Handle("/", r)
}
