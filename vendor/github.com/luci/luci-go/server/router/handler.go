// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package router

// Handler is the type for all request handlers.
type Handler func(*Context)

// Middleware is a function that accepts a shared context and the next
// function. Since Middleware is typically part of a chain of functions
// that handles an HTTP request, it must obey the following rules.
//
//     - Middleware must call next if it has not written to the Context
//       by the end of the function.
//     - Middleware must not call next if it has written to the Context.
//     - Middleware must not write to the Context after next is called and
//       the Context has been written to.
//     - Middleware may modify the embedded context before calling next.
type Middleware func(c *Context, next Handler)

// MiddlewareChain is an ordered collection of Middleware.
//
// MiddlewareChain's zero value is a middleware chain with no Middleware.
// Allocating a MiddlewareChain with Middleware can be done with
// NewMiddlewareChain.
type MiddlewareChain struct {
	middleware []Middleware
}

// RunMiddleware executes the middleware chain and handlers with the given
// initial context. Useful to execute a chain of functions in tests.
func RunMiddleware(c *Context, mc MiddlewareChain, h Handler) {
	runChains(c, mc, MiddlewareChain{}, h)
}

// NewMiddlewareChain creates a new MiddlewareChain with the supplied Middleware
// entries.
func NewMiddlewareChain(mw ...Middleware) (mc MiddlewareChain) {
	if len(mw) > 0 {
		mc = mc.Extend(mw...)
	}
	return
}

// Extend returns a new MiddlewareChain with the supplied Middleware appended to
// the end.
func (mc MiddlewareChain) Extend(mw ...Middleware) MiddlewareChain {
	if len(mw) == 0 {
		return mc
	}

	ext := make([]Middleware, 0, len(mc.middleware)+len(mw))
	return MiddlewareChain{append(append(ext, mc.middleware...), mw...)}
}

// ExtendFrom returns a new MiddlewareChain with the supplied MiddlewareChain
// appended to the end.
func (mc MiddlewareChain) ExtendFrom(other MiddlewareChain) MiddlewareChain {
	return mc.Extend(other.middleware...)
}

func runChains(c *Context, mc, nc MiddlewareChain, h Handler) {
	run(c, mc.middleware, nc.middleware, h)
}

// run executes the middleware chains m and n and the handler h using
// c as the initial context. If a middleware or handler is nil, run
// simply advances to the next middleware or handler.
func run(c *Context, m, n []Middleware, h Handler) {
	switch {
	case len(m) > 0:
		if m[0] != nil {
			m[0](c, func(ctx *Context) { run(ctx, m[1:], n, h) })
		} else {
			run(c, m[1:], n, h)
		}
	case len(n) > 0:
		if n[0] != nil {
			n[0](c, func(ctx *Context) { run(ctx, nil, n[1:], h) })
		} else {
			run(c, nil, n[1:], h)
		}
	case h != nil:
		h(c)
	}
}
