// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package services

import (
	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/config"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	"github.com/luci/luci-go/logdog/appengine/coordinator/endpoints"

	"golang.org/x/net/context"
)

// server is a service supporting privileged support services.
//
// This endpoint is restricted to LogDog support service accounts.
type server struct{}

// New creates a new authenticating ServicesServer instance.
func New() logdog.ServicesServer {
	return &logdog.DecoratedServices{
		Service: &server{},
		Prelude: func(c context.Context, methodName string, req proto.Message) (context.Context, error) {
			// Only service users may access this endpoint.
			if err := coordinator.IsServiceUser(c); err != nil {
				log.WithError(err).Errorf(c, "Failed to authenticate user as a service.")

				if !coordinator.IsMembershipError(err) {
					// Not a membership error. Something went wrong on the server's end.
					return nil, grpcutil.Internal
				}
				return nil, grpcutil.PermissionDenied
			}

			// Enter a datastore namespace based on the message type.
			//
			// We use a type switch here because this is a shared decorator.
			if pbm, ok := req.(endpoints.ProjectBoundMessage); ok {
				project := config.ProjectName(pbm.GetMessageProject())
				log.Fields{
					"project": project,
				}.Debugf(c, "Request is entering project namespace.")
				if err := coordinator.WithProjectNamespace(&c, project, coordinator.NamespaceAccessNoAuth); err != nil {
					return nil, err
				}
			}

			return c, nil
		},
	}
}
