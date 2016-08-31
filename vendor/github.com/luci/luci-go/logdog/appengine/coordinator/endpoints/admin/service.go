// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package admin

import (
	"github.com/golang/protobuf/proto"
	"github.com/luci/gae/service/info"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/admin/v1"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	"github.com/luci/luci-go/server/auth"
	"golang.org/x/net/context"
)

// server is the service implementation for the administrator endpoint.
type server struct{}

// New instantiates a new AdminServer instance.
func New() logdog.AdminServer {
	return &logdog.DecoratedAdmin{
		Service: &server{},
		Prelude: func(c context.Context, methodName string, req proto.Message) (context.Context, error) {
			if err := coordinator.IsAdminUser(c); err != nil {
				log.WithError(err).Warningf(c, "User is not an administrator.")

				// If we're on development server, any user can access this endpoint.
				if info.Get(c).IsDevAppServer() {
					log.Infof(c, "On development server, allowing admin access.")
					return c, nil
				}

				u := auth.CurrentUser(c)
				if u == nil || !u.Superuser {
					return nil, grpcutil.PermissionDenied
				}

				log.Fields{
					"email":    u.Email,
					"clientID": u.ClientID,
					"name":     u.Name,
				}.Infof(c, "User is an AppEngine superuser. Granting access.")
			}

			return c, nil
		},
	}
}
