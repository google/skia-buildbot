// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package swarming

import (
	"net/http"
	"os"

	"google.golang.org/api/googleapi"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/milo/appengine/settings"
	"github.com/luci/luci-go/milo/common/miloerror"
	"github.com/luci/luci-go/server/templates"
)

func getServer(r *http.Request) string {
	server := r.FormValue("server")
	// TODO(hinoka): configure this mapping in luci-config
	switch server {
	case "":
		return "chromium-swarm.appspot.com"
	case "dev":
		return "chromium-swarm-dev.appspot.com"
	default:
		return server
	}
}

// Log is for fetching logs from swarming.
type Log struct{}

// Build is for deciphering recipe builds from swarming based off of logs.
type Build struct{}

// GetTemplateName for Log returns the template name for log pages.
func (l Log) GetTemplateName(t settings.Theme) string {
	return "log.html"
}

// Render writes the build log to the given response writer.
func (l Log) Render(c context.Context, r *http.Request, p httprouter.Params) (*templates.Args, error) {
	id := p.ByName("id")
	if id == "" {
		return nil, &miloerror.Error{
			Message: "No id",
			Code:    http.StatusBadRequest,
		}
	}
	logname := p.ByName("logname")
	if logname == "" {
		return nil, &miloerror.Error{
			Message: "No log name",
			Code:    http.StatusBadRequest,
		}
	}

	log, closed, err := swarmingBuildLogImpl(c, getServer(r), id, logname)
	if err != nil {
		return nil, convertErr(err)
	}

	args := &templates.Args{
		"Log":    log,
		"Closed": closed,
	}
	return args, nil
}

// GetTemplateName for Build returns the template name for build pages.
func (b Build) GetTemplateName(t settings.Theme) string {
	return "build.html"
}

// Render renders both the build page and the log.
func (b Build) Render(c context.Context, r *http.Request, p httprouter.Params) (*templates.Args, error) {
	// Get the swarming ID
	id := p.ByName("id")
	if id == "" {
		return nil, &miloerror.Error{
			Message: "No id",
			Code:    http.StatusBadRequest,
		}
	}

	result, err := swarmingBuildImpl(c, r.URL.String(), getServer(r), id)
	if err != nil {
		return nil, convertErr(err)
	}

	// Render into the template
	args := &templates.Args{
		"Build": result,
	}
	return args, nil
}

func convertErr(err error) error {
	if isAPINotFound(err) || os.IsNotExist(err) {
		return &miloerror.Error{
			Message: err.Error(),
			Code:    http.StatusNotFound,
		}
	}
	return err
}

// isAPINotFound returns true if err is a HTTP 404 API response.
func isAPINotFound(err error) bool {
	if apiErr, ok := err.(*googleapi.Error); ok && apiErr.Code == http.StatusNotFound {
		return true
	}

	return false
}
