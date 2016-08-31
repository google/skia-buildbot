// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package buildbucket

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/milo/appengine/settings"
	"github.com/luci/luci-go/milo/common/miloerror"
	"github.com/luci/luci-go/server/templates"
)

// TODO(nodir): move this value to luci-config.
const defaultServer = "cr-buildbucket.appspot.com"

// Builder displays builder view by fetching builds from buildbucket.
type Builder struct{}

// GetTemplateName for Builder returns the template name for builder pages.
func (b Builder) GetTemplateName(t settings.Theme) string {
	return "builder.html"
}

// Render renders builder view page.
func (b Builder) Render(c context.Context, r *http.Request, p httprouter.Params) (*templates.Args, error) {
	// Parse URL parameters.
	server := r.FormValue("server")
	if server == "" {
		server = defaultServer
	}

	bucket := p.ByName("bucket")
	if bucket == "" {
		return nil, &miloerror.Error{
			Message: "No bucket",
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

	// numbuilds is a name of buildbot's query string parameter for specifying
	// maximum number of builds to show.
	// We are retaining the parameter name for user convenience.
	numBuildsStr := r.FormValue("numbuilds")
	numBuilds := -1
	if numBuildsStr != "" {
		var err error
		numBuilds, err = strconv.Atoi(numBuildsStr)
		if err != nil {
			return nil, &miloerror.Error{
				Message: fmt.Sprintf("numbuilds parameter value %q is not a number: %s", numBuildsStr, err),
				Code:    http.StatusBadRequest,
			}
		}
	}

	result, err := builderImpl(c, server, bucket, builder, numBuilds)
	if err != nil {
		return nil, err
	}

	// Render into the template
	args := &templates.Args{
		"Builder": result,
	}
	return args, nil
}
