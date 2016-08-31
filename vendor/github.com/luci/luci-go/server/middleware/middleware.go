// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package middleware defines base type for context-aware HTTP request handler.
// See appengine/middleware for examples of how to use it in GAE environment.
package middleware

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"
)

// Handler is the type for all request handlers. Of particular note, it's the
// same as httprouter.Handle, except that it also has a context parameter.
type Handler func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params)

// Middleware takes a handler, wraps it with some additional logic, and returns
// resulting handler.
type Middleware func(Handler) Handler

// Base is a start of the middlware chain. It sets up initial context with all
// base services and passes it to the given handler. Return value of Base can
// be plugged in into httprouter directly.
type Base func(Handler) httprouter.Handle

// TestingBase is Base that passes given context to the handler. Useful in
// tests.
func TestingBase(c context.Context) Base {
	return func(h Handler) httprouter.Handle {
		return func(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
			h(c, rw, r, p)
		}
	}
}

// WithContextValue is a middleware that adds a value to the context before
// calling the handler.
func WithContextValue(h Handler, key, val interface{}) Handler {
	return func(c context.Context, rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
		h(context.WithValue(c, key, val), rw, r, p)
	}
}
