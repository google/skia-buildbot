// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logdog

import (
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/luci/luci-go/common/config"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/common/types"
	"github.com/luci/luci-go/milo/appengine/settings"
	"github.com/luci/luci-go/milo/common/miloerror"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/templates"

	"golang.org/x/net/context"
)

// AnnotationStream is a ThemedHandler that renders a LogDog Milo annotation
// protobuf stream.
//
// The protobuf stream is fetched live from LogDog and cached locally, either
// temporarily (if incomplete) or indefinitely (if complete).
type AnnotationStream struct {
	// logDogClient is a reusable HTTP client to use for LogDog.
	logDogClient *http.Client
}

// GetTemplateName implements settings.ThemedHandler.
func (s *AnnotationStream) GetTemplateName(t settings.Theme) string {
	return "build.html"
}

// Render implements settings.ThemedHandler.
func (s *AnnotationStream) Render(c context.Context, req *http.Request, p httprouter.Params) (*templates.Args, error) {
	// Initialize the LogDog client authentication.
	// TODO(vadimsh): Use auth.AsUser.
	t, err := auth.GetRPCTransport(c, auth.AsSelf)
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to get transport for LogDog server.")
		return nil, &miloerror.Error{
			Code: http.StatusInternalServerError,
		}
	}

	as := annotationStreamRequest{
		AnnotationStream: s,

		project: config.ProjectName(p.ByName("project")),
		path:    types.StreamPath(strings.Trim(p.ByName("path"), "/")),
		host:    req.FormValue("host"),

		logDogClient: http.Client{
			Transport: t,
		},
	}
	if err := as.normalize(); err != nil {
		return nil, err
	}

	// Load the Milo annotation protobuf from the annotation stream.
	if err := as.load(c); err != nil {
		return nil, err
	}

	// Convert the Milo Annotation protobuf to Milo objects.
	return &templates.Args{
		"Build": as.toMiloBuild(c),
	}, nil
}
