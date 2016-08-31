// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package buildbot

import (
	"fmt"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/milo/appengine/settings"
	"github.com/luci/luci-go/milo/common/miloerror"
	"github.com/luci/luci-go/server/templates"
)

// Build is the container struct for methods related to buildbot build pages.
type Build struct{}

// Builder is the container struct for methods related to buildbot builder pages.
type Builder struct{}

// GetTemplateName returns the template name for build pages.
func (b Build) GetTemplateName(t settings.Theme) string {
	return "build.html"
}

// Render Render the buildbot build page.
func (b Build) Render(c context.Context, r *http.Request, p httprouter.Params) (*templates.Args, error) {
	master := p.ByName("master")
	if master == "" {
		return nil, &miloerror.Error{
			Message: "No master",
			Code:    http.StatusBadRequest,
		}
	}
	builder := p.ByName("builder")
	if builder == "" {
		return nil, &miloerror.Error{
			Message: "No builder",
			Code:    http.StatusBadRequest,
		}
	}
	buildNum := p.ByName("build")
	if buildNum == "" {
		return nil, &miloerror.Error{
			Message: "No build num",
			Code:    http.StatusBadRequest,
		}
	}

	result, err := build(c, master, builder, buildNum)
	if err != nil {
		return nil, err
	}

	// Render into the template
	fmt.Fprintf(os.Stderr, "Result: %#v\n\n", result)
	args := &templates.Args{
		"Build": result,
	}
	return args, nil
}

// GetTemplateName returns the template name for builder pages.
func (b Builder) GetTemplateName(t settings.Theme) string {
	return "builder.html"
}

// Render renders the buildbot builder page.
func (b Builder) Render(c context.Context, r *http.Request, p httprouter.Params) (*templates.Args, error) {
	master := p.ByName("master")
	if master == "" {
		return nil, &miloerror.Error{
			Message: "No master",
			Code:    http.StatusBadRequest,
		}
	}
	builder := p.ByName("builder")
	if builder == "" {
		return nil, &miloerror.Error{
			Message: "No builder",
			Code:    http.StatusBadRequest,
		}
	}

	result, err := builderImpl(c, master, builder)
	if err != nil {
		return nil, err
	}

	// Render into the template
	args := &templates.Args{
		"Builder": result,
	}
	return args, nil
}
