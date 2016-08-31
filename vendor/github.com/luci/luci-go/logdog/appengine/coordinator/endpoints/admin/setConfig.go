// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package admin

import (
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/admin/v1"
	"github.com/luci/luci-go/logdog/appengine/coordinator/config"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
)

// SetConfig loads the supplied configuration into a config.GlobalConfig
// instance.
func (s *server) SetConfig(c context.Context, req *logdog.SetConfigRequest) (*google.Empty, error) {
	se := config.Settings{
		BigTableServiceAccountJSON: req.StorageServiceAccountJson,
	}
	if err := se.Validate(); err != nil {
		log.Fields{
			log.ErrorKey: err,
		}.Errorf(c, "New configuration did not validate.")
		return nil, grpcutil.Errf(codes.InvalidArgument, "config did not validate: %v", err)
	}

	if err := se.Store(c, "setConfig endpoint"); err != nil {
		log.Fields{
			log.ErrorKey: err,
		}.Errorf(c, "Failed to store new configuration.")
		return nil, grpcutil.Internal
	}
	return &google.Empty{}, nil
}
