// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package main implements HTTP server that handles requests to default
// module.
package main

import (
	"net/http"
	"strings"

	"golang.org/x/net/context"
	"google.golang.org/appengine"

	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/appengine/gaeauth/server"
	"github.com/luci/luci-go/appengine/gaemiddleware"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/router"
	"github.com/luci/luci-go/server/templates"
)

// templateBundle is used to render HTML templates. It provides a base args
// passed to all templates.
var templateBundle = &templates.Bundle{
	Loader:    templates.FileSystemLoader("templates"),
	DebugMode: appengine.IsDevAppServer(),
	DefaultArgs: func(c context.Context) (templates.Args, error) {
		loginURL, err := auth.LoginURL(c, "/")
		if err != nil {
			return nil, err
		}
		logoutURL, err := auth.LogoutURL(c, "/")
		if err != nil {
			return nil, err
		}
		isAdmin, err := auth.IsMember(c, "administrators")
		if err != nil {
			return nil, err
		}
		return templates.Args{
			"AppVersion":  strings.Split(info.Get(c).VersionID(), ".")[0],
			"IsAnonymous": auth.CurrentIdentity(c) == "anonymous:anonymous",
			"IsAdmin":     isAdmin,
			"User":        auth.CurrentUser(c),
			"LoginURL":    loginURL,
			"LogoutURL":   logoutURL,
		}, nil
	},
}

// base returns the root middleware chain.
func base() router.MiddlewareChain {
	methods := auth.Authenticator{
		&server.OAuth2Method{Scopes: []string{server.EmailScope}},
		server.CookieAuth,
		&server.InboundAppIDAuthMethod{},
	}
	return gaemiddleware.BaseProd().Extend(
		templates.WithTemplates(templateBundle),
		auth.Use(methods),
	)
}

//// Routes.

func main() {
	r := router.New()
	basemw := base()
	gaemiddleware.InstallHandlers(r, basemw)
	r.GET("/", basemw.Extend(auth.Authenticate), indexPage)
	http.DefaultServeMux.Handle("/", r)

	appengine.Main()
}

//// Handlers.

func indexPage(c *router.Context) {
	templates.MustRender(c.Context, c.Writer, "pages/index.html", nil)
}
