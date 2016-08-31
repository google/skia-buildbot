// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package registration

import (
	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/config"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/registration/v1"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	"github.com/luci/luci-go/logdog/appengine/coordinator/endpoints"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
)

// server is a service supporting log stream registration.
type server struct{}

// New creates a new authenticating ServicesServer instance.
func New() logdog.RegistrationServer {
	return &logdog.DecoratedRegistration{
		Service: &server{},
		Prelude: func(c context.Context, methodName string, req proto.Message) (context.Context, error) {
			// Enter a datastore namespace based on the message type.
			//
			// We use a type switch here because this is a shared decorator. All user
			// mesages must implement ProjectBoundMessage.
			pbm, ok := req.(endpoints.ProjectBoundMessage)
			if ok {
				// Enter the requested project namespace. This validates that the
				// current user has READ access.
				project := config.ProjectName(pbm.GetMessageProject())
				if project == "" {
					return nil, grpcutil.Errf(codes.InvalidArgument, "project is required")
				}

				log.Fields{
					"project": project,
				}.Debugf(c, "User is accessing project.")
				if err := coordinator.WithProjectNamespace(&c, project, coordinator.NamespaceAccessWRITE); err != nil {
					return nil, getGRPCError(err)
				}
			}

			return c, nil
		},
	}
}

func getGRPCError(err error) error {
	switch {
	case err == nil:
		return nil

	case grpcutil.Code(err) != codes.Unknown:
		// If this is already a gRPC error, return it directly.
		return err

	default:
		// Generic empty internal error.
		return grpcutil.Internal
	}
}
