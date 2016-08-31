// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package settings

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/server/auth/xsrf"
	"github.com/luci/luci-go/server/templates"
)

// Settings - Container for html methods for settings.
type Settings struct{}

// GetTemplateName - Implements a Theme, template is constant.
func (s Settings) GetTemplateName(t Theme) string {
	return "settings.html"
}

// Render renders both the build page and the log.
func (s Settings) Render(c context.Context, r *http.Request, p httprouter.Params) (*templates.Args, error) {
	result, err := getSettings(c, r)
	if err != nil {
		return nil, err
	}

	token, err := xsrf.Token(c)
	if err != nil {
		return nil, err
	}

	args := &templates.Args{
		"Settings":  result,
		"XsrfToken": token,
	}
	return args, nil
}

// ViewConfigs - Container for viewing the current set of luci-configs.
type ViewConfigs struct{}

func (s ViewConfigs) GetTemplateName(t Theme) string {
	return "configs.html"
}

func (s ViewConfigs) Render(c context.Context, r *http.Request, p httprouter.Params) (*templates.Args, error) {
	projects, err := GetAllProjects(c)
	if err != nil {
		return nil, err
	}

	return &templates.Args{
		"Projects": projects,
	}, nil
}
