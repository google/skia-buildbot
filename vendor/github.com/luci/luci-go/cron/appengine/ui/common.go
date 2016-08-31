// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package ui implements request handlers that serve user facing HTML pages.
package ui

import (
	"strings"

	"golang.org/x/net/context"
	"google.golang.org/appengine"

	"github.com/luci/gae/service/info"

	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/auth/xsrf"
	"github.com/luci/luci-go/server/router"
	"github.com/luci/luci-go/server/templates"

	"github.com/luci/luci-go/cron/appengine/engine"
)

// Config is global configuration of UI handlers.
type Config struct {
	Engine        engine.Engine
	TemplatesPath string // path to templates directory deployed to GAE
}

// InstallHandlers adds HTTP handlers that render HTML pages.
func InstallHandlers(r *router.Router, base router.MiddlewareChain, cfg Config) {
	tmpl := prepareTemplates(cfg.TemplatesPath)

	m := base.Extend(func(c *router.Context, next router.Handler) {
		c.Context = context.WithValue(c.Context, configContextKey(0), &cfg)
		next(c)
	}, templates.WithTemplates(tmpl), auth.Authenticate)

	r.GET("/", m, indexPage)
	r.GET("/jobs/:ProjectID", m, projectPage)
	r.GET("/jobs/:ProjectID/:JobID", m, jobPage)
	r.GET("/jobs/:ProjectID/:JobID/:InvID", m, invocationPage)

	// All POST forms must be protected with XSRF token.
	mxsrf := m.Extend(xsrf.WithTokenCheck)
	r.POST("/actions/runJob/:ProjectID/:JobID", mxsrf, runJobAction)
	r.POST("/actions/pauseJob/:ProjectID/:JobID", mxsrf, pauseJobAction)
	r.POST("/actions/resumeJob/:ProjectID/:JobID", mxsrf, resumeJobAction)
	r.POST("/actions/abortJob/:ProjectID/:JobID", mxsrf, abortJobAction)
	r.POST("/actions/abortInvocation/:ProjectID/:JobID/:InvID", mxsrf, abortInvocationAction)
}

type configContextKey int

// config returns Config passed to InstallHandlers.
func config(c context.Context) *Config {
	cfg, _ := c.Value(configContextKey(0)).(*Config)
	if cfg == nil {
		panic("impossible, configContextKey is not set")
	}
	return cfg
}

// prepareTemplates configures templates.Bundle used by all UI handlers.
//
// In particular it includes a set of default arguments passed to all templates.
func prepareTemplates(templatesPath string) *templates.Bundle {
	return &templates.Bundle{
		Loader:          templates.FileSystemLoader(templatesPath),
		DebugMode:       appengine.IsDevAppServer(),
		DefaultTemplate: "base",
		DefaultArgs: func(c context.Context) (templates.Args, error) {
			loginURL, err := auth.LoginURL(c, "/")
			if err != nil {
				return nil, err
			}
			logoutURL, err := auth.LogoutURL(c, "/")
			if err != nil {
				return nil, err
			}
			token, err := xsrf.Token(c)
			if err != nil {
				return nil, err
			}
			return templates.Args{
				"AppVersion":  strings.Split(info.Get(c).VersionID(), ".")[0],
				"IsAnonymous": auth.CurrentIdentity(c) == "anonymous:anonymous",
				"User":        auth.CurrentUser(c),
				"LoginURL":    loginURL,
				"LogoutURL":   logoutURL,
				"XsrfToken":   token,
			}, nil
		},
	}
}
