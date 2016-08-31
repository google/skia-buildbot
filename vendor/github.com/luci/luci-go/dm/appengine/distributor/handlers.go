// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package distributor

import (
	"github.com/luci/luci-go/appengine/gaemiddleware"
	"github.com/luci/luci-go/server/router"
)

// InstallHandlers installs the taskqueue callback handler.
//
// The `base` middleware must have a registry installed with WithRegistry.
func InstallHandlers(r *router.Router, base router.MiddlewareChain) {
	r.POST(handlerPattern, base.Extend(gaemiddleware.RequireTaskQueue("")), TaskQueueHandler)
	r.POST("/_ah/push-handlers/"+notifyTopicSuffix, base, PubsubReceiver)
}
