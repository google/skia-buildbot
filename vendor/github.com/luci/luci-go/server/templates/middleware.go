// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package templates

import (
	"fmt"
	"net/http"

	"github.com/luci/luci-go/server/router"
)

// WithTemplates is middleware that lazily loads template bundle and injects it
// into the context.
//
// Wrapper reply with HTTP 500 if templates can not be loaded. Inner handler
// receives context with all templates successfully loaded.
func WithTemplates(b *Bundle) router.Middleware {
	return func(c *router.Context, next router.Handler) {
		c.Context = Use(c.Context, b) // calls EnsureLoaded and initializes b.err inside
		if b.err != nil {
			http.Error(c.Writer, fmt.Sprintf("Can't load HTML templates.\n%s", b.err), http.StatusInternalServerError)
			return
		}
		next(c)
	}
}
