// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package router provides an HTTP router with support for middleware and
// subrouters. It wraps around julienschmidt/httprouter.
package router

import (
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"
)

// Router is the main type for the package. To create a Router, use New.
type Router struct {
	hrouter    *httprouter.Router
	middleware MiddlewareChain
	BasePath   string
}

// Context contains the context, response writer, request, and params shared
// across Middleware and Handler functions.
type Context struct {
	Context context.Context
	Writer  http.ResponseWriter
	Request *http.Request
	Params  httprouter.Params
}

var _ http.Handler = (*Router)(nil)

// New creates a Router.
func New() *Router {
	return &Router{
		hrouter:    httprouter.New(),
		middleware: NewMiddlewareChain(),
		BasePath:   "/",
	}
}

// Use adds middleware chains to the group. The added middleware applies to
// all handlers registered on the router and to all handlers registered on
// routers that may be derived from the router (using Subrouter).
func (r *Router) Use(mc MiddlewareChain) {
	r.middleware = r.middleware.ExtendFrom(mc)
}

// Subrouter creates a new router with an updated base path.
// The new router copies middleware and configuration from the
// router it derives from.
func (r *Router) Subrouter(relativePath string) *Router {
	newRouter := &Router{
		hrouter:    r.hrouter,
		BasePath:   makeBasePath(r.BasePath, relativePath),
		middleware: r.middleware,
	}
	return newRouter
}

// GET is a shortcut for router.Handle("GET", path, mc, h)
func (r *Router) GET(path string, mc MiddlewareChain, h Handler) {
	r.Handle("GET", path, mc, h)
}

// HEAD is a shortcut for router.Handle("HEAD", path, mc, h)
func (r *Router) HEAD(path string, mc MiddlewareChain, h Handler) {
	r.Handle("HEAD", path, mc, h)
}

// OPTIONS is a shortcut for router.Handle("OPTIONS", path, mc, h)
func (r *Router) OPTIONS(path string, mc MiddlewareChain, h Handler) {
	r.Handle("OPTIONS", path, mc, h)
}

// POST is a shortcut for router.Handle("POST", path, mc, h)
func (r *Router) POST(path string, mc MiddlewareChain, h Handler) {
	r.Handle("POST", path, mc, h)
}

// PUT is a shortcut for router.Handle("PUT", path, mc, h)
func (r *Router) PUT(path string, mc MiddlewareChain, h Handler) {
	r.Handle("PUT", path, mc, h)
}

// PATCH is a shortcut for router.Handle("PATCH", path, mc, h)
func (r *Router) PATCH(path string, mc MiddlewareChain, h Handler) {
	r.Handle("PATCH", path, mc, h)
}

// DELETE is a shortcut for router.Handle("DELETE", path, mc, h)
func (r *Router) DELETE(path string, mc MiddlewareChain, h Handler) {
	r.Handle("DELETE", path, mc, h)
}

// Handle registers a middleware chain and a handler for the given method and
// path. len(mc)==0 is allowed. See https://godoc.org/github.com/julienschmidt/httprouter
// for documentation on how the path may be formatted.
func (r *Router) Handle(method, path string, mc MiddlewareChain, h Handler) {
	handle := r.adapt(mc, h)
	r.hrouter.Handle(method, makeBasePath(r.BasePath, path), handle)
}

// ServeHTTP makes Router implement the http.Handler interface.
func (r *Router) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	r.hrouter.ServeHTTP(rw, req)
}

// adapt adapts given middleware chain and handler into a httprouter-style handle.
func (r *Router) adapt(mc MiddlewareChain, h Handler) httprouter.Handle {
	return httprouter.Handle(func(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
		runChains(&Context{
			Context: context.Background(),
			Writer:  rw,
			Request: req,
			Params:  p,
		}, r.middleware, mc, h)
	})
}

// makeBasePath combines the given base and relative path using "/".
// The result is: "/"+base+"/"+relative. Consecutive "/" are collapsed
// into a single "/". In addition, the following rules apply:
//         - The "/" between base and relative exists only if either base has a
//           trailing "/" or relative is not the empty string.
//         - A trailing "/" is added to the result if relative has a trailing
//           "/".
func makeBasePath(base, relative string) string {
	if !strings.HasSuffix(base, "/") && relative != "" {
		base += "/"
	}
	return httprouter.CleanPath(base + relative)
}
