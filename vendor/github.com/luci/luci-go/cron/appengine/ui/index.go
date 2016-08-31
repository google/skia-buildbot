// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package ui

import (
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/server/router"
	"github.com/luci/luci-go/server/templates"
)

func indexPage(c *router.Context) {
	jobs, err := config(c.Context).Engine.GetAllCronJobs(c.Context)
	if err != nil {
		panic(err)
	}
	templates.MustRender(c.Context, c.Writer, "pages/index.html", map[string]interface{}{
		"Jobs": convertToSortedCronJobs(jobs, clock.Now(c.Context).UTC()),
	})
}
