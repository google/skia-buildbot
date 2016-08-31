// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package services

import (
	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
)

// LoadStream loads the log stream state.
func (s *server) LoadStream(c context.Context, req *logdog.LoadStreamRequest) (*logdog.LoadStreamResponse, error) {
	log.Fields{
		"project": req.Project,
		"id":      req.Id,
	}.Infof(c, "Loading log stream state.")

	id := coordinator.HashID(req.Id)
	if err := id.Normalize(); err != nil {
		log.WithError(err).Errorf(c, "Invalid stream ID.")
		return nil, grpcutil.Errf(codes.InvalidArgument, "Invalid ID (%s): %s", id, err)
	}

	di := ds.Get(c)
	ls := &coordinator.LogStream{ID: coordinator.HashID(req.Id)}
	lst := ls.State(di)

	if err := di.Get(lst, ls); err != nil {
		if anyNoSuchEntity(err) {
			log.WithError(err).Errorf(c, "No such entity in datastore.")

			// The state isn't registered, so this stream does not exist.
			return nil, grpcutil.Errf(codes.NotFound, "Log stream was not found.")
		}

		log.WithError(err).Errorf(c, "Failed to load log stream.")
		return nil, grpcutil.Internal
	}

	// The log stream and state loaded successfully.
	resp := logdog.LoadStreamResponse{
		State: buildLogStreamState(ls, lst),
	}
	if req.Desc {
		resp.Desc = ls.Descriptor
	}
	resp.ArchivalKey = lst.ArchivalKey
	resp.Age = google.NewDuration(ds.RoundTime(clock.Now(c)).Sub(lst.Updated))

	log.Fields{
		"id":              lst.ID(),
		"terminalIndex":   resp.State.TerminalIndex,
		"archived":        resp.State.Archived,
		"purged":          resp.State.Purged,
		"age":             resp.Age.Duration(),
		"archivalKeySize": len(resp.ArchivalKey),
	}.Infof(c, "Successfully loaded log stream state.")
	return &resp, nil
}

func anyNoSuchEntity(err error) bool {
	return errors.Any(err, func(err error) bool {
		return err == ds.ErrNoSuchEntity
	})
}
