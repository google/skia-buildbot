// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package main implements HTTP server that handles requests to backend
// module.
package main

import (
	"net/http"

	"google.golang.org/appengine"

	"github.com/luci/luci-go/appengine/gaemiddleware"
	"github.com/luci/luci-go/server/router"
)

//// Routes.

func main() {
	r := router.New()
	basemw := gaemiddleware.BaseProd()

	gaemiddleware.InstallHandlers(r, basemw)
	r.GET("/hi", basemw, sayHi)
	http.DefaultServeMux.Handle("/", r)

	appengine.Main()
}

//// Handlers.

func sayHi(c *router.Context) {
	c.Writer.Write([]byte("Hi, I'm backend"))
}
