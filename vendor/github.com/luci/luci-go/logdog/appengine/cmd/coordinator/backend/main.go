// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package module

import (
	"net/http"

	"github.com/luci/luci-go/logdog/appengine/coordinator"
	"github.com/luci/luci-go/server/router"
	"github.com/luci/luci-go/tumble"

	// Include mutations package so its Mutations will register with tumble via
	// init().
	_ "github.com/luci/luci-go/logdog/appengine/coordinator/mutations"
)

func init() {
	tmb := tumble.Service{}

	r := router.New()
	tmb.InstallHandlers(r, coordinator.ProdServices())

	http.Handle("/", r)
}
