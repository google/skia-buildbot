// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package frontend

import (
	"net/http"

	"github.com/luci/luci-go/appengine/gaemiddleware"
	"github.com/luci/luci-go/milo/appengine/buildbot"
	"github.com/luci/luci-go/milo/appengine/buildbucket"
	"github.com/luci/luci-go/milo/appengine/console"
	"github.com/luci/luci-go/milo/appengine/logdog"
	"github.com/luci/luci-go/milo/appengine/settings"
	"github.com/luci/luci-go/milo/appengine/swarming"
	"github.com/luci/luci-go/server/router"
)

// Where it all begins!!!
func init() {
	// Register plain ol' http handlers.
	r := router.New()
	basemw := settings.Base()
	gaemiddleware.InstallHandlers(r, basemw)
	r.GET("/", basemw, settings.Wrap(frontpage{}))

	// Admin and cron endpoints.
	r.GET("/admin/update", basemw.Extend(gaemiddleware.RequireCron),
		settings.UpdateHandler)
	r.GET("/admin/configs", basemw, settings.Wrap(settings.ViewConfigs{}))

	// Console
	r.GET("/console/:project/:name", basemw, settings.Wrap(console.Console{}))
	r.GET("/console/:project", basemw, console.Main)

	// Swarming
	r.GET("/swarming/task/:id/steps/*logname", basemw, settings.Wrap(swarming.Log{}))
	r.GET("/swarming/task/:id", basemw, settings.Wrap(swarming.Build{}))
	// Backward-compatible URLs:
	r.GET("/swarming/prod/:id/steps/*logname", basemw, settings.Wrap(swarming.Log{}))
	r.GET("/swarming/prod/:id", basemw, settings.Wrap(swarming.Build{}))

	// Buildbucket
	r.GET("/buildbucket/:bucket/:builder", basemw, settings.Wrap(buildbucket.Builder{}))

	// Buildbot
	r.GET("/buildbot/:master/:builder/:build", basemw, settings.Wrap(buildbot.Build{}))
	r.GET("/buildbot/:master/:builder/", basemw, settings.Wrap(buildbot.Builder{}))

	// LogDog Milo Annotation Streams.
	r.GET("/logdog/build/:project/*path", basemw, settings.Wrap(&logdog.AnnotationStream{}))

	// User settings
	r.GET("/settings", basemw, settings.Wrap(settings.Settings{}))
	r.POST("/settings", basemw, settings.ChangeSettings)

	// PubSub subscription endpoints.
	r.POST("/pubsub/buildbot", basemw, buildbot.PubSubHandler)

	http.Handle("/", r)
}
