// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package frontend implements HTTP server that handles requests to default
// module.
package frontend

import (
	"net/http"
	"strings"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/appengine"

	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/appengine/gaeauth/server"
	"github.com/luci/luci-go/appengine/gaemiddleware"
	"github.com/luci/luci-go/examples/appengine/helloworld_standard/proto"
	"github.com/luci/luci-go/grpc/discovery"
	"github.com/luci/luci-go/grpc/prpc"
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

// pageBase returns the middleware chain for page handlers.
func pageBase() router.MiddlewareChain {
	return gaemiddleware.BaseProd().Extend(
		templates.WithTemplates(templateBundle),
		auth.Use(auth.Authenticator{server.CookieAuth}),
	)
}

// prpcBase returns the middleware chain for pRPC API handlers.
func prpcBase() router.MiddlewareChain {
	// OAuth 2.0 with email scope is registered as a default authenticator
	// by importing "github.com/luci/luci-go/appengine/gaeauth/server".
	// No need to setup an authenticator here.
	//
	// For authorization checks, we use per-service decorators; see
	// service registration code.
	return gaemiddleware.BaseProd()
}

//// Routes.

func checkAPIAccess(c context.Context, methodName string, req proto.Message) (context.Context, error) {
	// Implement authorization check here, for example:
	//
	// import "github.com/golang/protobuf/proto"
	// import "google.golang.org/grpc/codes"
	// import "github.com/luci/luci-go/grpc/grpcutil"
	//
	// hasAccess, err := auth.IsMember(c, "my-users")
	// if err != nil {
	//   return nil, grpcutil.Errf(codes.Internal, "%s", err)
	// }
	// if !hasAccess {
	//   return nil, grpcutil.Errf(codes.PermissionDenied, "%s is not allowed to call APIs", auth.CurrentIdentity(c))
	// }

	return c, nil
}

func init() {
	r := router.New()
	gaemiddleware.InstallHandlers(r, pageBase())
	r.GET("/", pageBase().Extend(auth.Authenticate), indexPage)

	var api prpc.Server
	helloworld.RegisterGreeterServer(&api, &helloworld.DecoratedGreeter{
		Service: &greeterService{},
		Prelude: checkAPIAccess,
	})
	discovery.Enable(&api)
	api.InstallHandlers(r, prpcBase())

	http.DefaultServeMux.Handle("/", r)
}

//// Handlers.

func indexPage(c *router.Context) {
	templates.MustRender(c.Context, c.Writer, "pages/index.html", nil)
}
