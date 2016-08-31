// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package middleware

import (
	"net/http"

	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/runtime/paniccatcher"
	"github.com/luci/luci-go/server/router"
)

// WithPanicCatcher is a middleware that catches panics, dumps stack trace to
// logging and returns HTTP 500.
func WithPanicCatcher(c *router.Context, next router.Handler) {
	ctx := c.Context
	w := c.Writer
	req := c.Request
	defer paniccatcher.Catch(func(p *paniccatcher.Panic) {
		log.Fields{
			"panic.error": p.Reason,
		}.Errorf(ctx, "Caught panic during handling of %q:\n%s", req.RequestURI, p.Stack)

		// Note: it may be too late to send HTTP 500 if `next` already sent
		// headers. But there's nothing else we can do at this point anyway.
		http.Error(w, "Internal Server Error. See logs.", http.StatusInternalServerError)
	})
	next(c)
}
