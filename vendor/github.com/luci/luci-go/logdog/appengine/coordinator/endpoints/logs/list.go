// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logs

import (
	ds "github.com/luci/gae/service/datastore"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/logs/v1"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	"github.com/luci/luci-go/logdog/appengine/coordinator/hierarchy"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
)

const (
	// listResultLimit is the maximum number of log streams that will be
	// returned in a single query. If the user requests more, it will be
	// automatically truncated to this value.
	listResultLimit = 500
)

// List returns log stream paths rooted under the hierarchy.
func (s *server) List(c context.Context, req *logdog.ListRequest) (*logdog.ListResponse, error) {
	log.Fields{
		"project":       req.Project,
		"path":          req.PathBase,
		"offset":        req.Offset,
		"streamOnly":    req.StreamOnly,
		"includePurged": req.IncludePurged,
		"maxResults":    req.MaxResults,
	}.Debugf(c, "Received List request.")

	hr := hierarchy.Request{
		Project:    req.Project,
		PathBase:   req.PathBase,
		StreamOnly: req.StreamOnly,
		Limit:      s.limit(int(req.MaxResults), listResultLimit),
		Next:       req.Next,
		Skip:       int(req.Offset),
	}

	// Non-admin users may not request purged results.
	if req.IncludePurged {
		if err := coordinator.IsAdminUser(c); err != nil {
			log.Fields{
				log.ErrorKey: err,
			}.Errorf(c, "Non-superuser requested to see purged paths. Denying.")
			return nil, grpcutil.Errf(codes.PermissionDenied, "non-admin user cannot request purged log paths")
		}

		// TODO(dnj): Apply this to the hierarchy request, when purging is
		// enabled.
	}

	l, err := hierarchy.Get(c, hr)
	if err != nil {
		// Get returns a gRPC error.
		log.WithError(err).Errorf(c, "Failed to get hierarchy listing.")
		return nil, err
	}

	resp := logdog.ListResponse{
		Project:  string(l.Project),
		Next:     l.Next,
		PathBase: string(l.PathBase),
	}

	if len(l.Comp) > 0 {
		resp.Components = make([]*logdog.ListResponse_Component, len(l.Comp))

		for i, c := range l.Comp {
			comp := logdog.ListResponse_Component{
				Name: c.Name,
			}
			switch {
			case l.Project == "":
				comp.Type = logdog.ListResponse_Component_PROJECT
			case c.Stream:
				comp.Type = logdog.ListResponse_Component_STREAM
			default:
				comp.Type = logdog.ListResponse_Component_PATH
			}

			resp.Components[i] = &comp
		}
	}

	// Perform additional stream metadata fetch if state is requested. Collect
	// a list of streams to load.
	if req.State && l.Project != "" {
		c := c
		if err := coordinator.WithProjectNamespace(&c, l.Project, coordinator.NamespaceAccessREAD); err != nil {
			// This should work, since the decorated service would have rejected the
			// namespace if the user was not a member, so a failure here is an
			// internal error.
			log.Fields{
				log.ErrorKey: err,
				"project":    l.Project,
			}.Errorf(c, "Failed to enter namespace for metadata lookup.")
			return nil, grpcutil.Internal
		}

		idxMap := make(map[int]*logdog.ListResponse_Component)
		var streams []coordinator.HashID

		for i, comp := range l.Comp {
			if !comp.Stream {
				continue
			}

			idxMap[len(streams)] = resp.Components[i]
			log.Fields{
				"value": l.Path(comp),
			}.Infof(c, "Loading stream.")
			streams = append(streams, coordinator.LogStreamID(l.Path(comp)))
		}

		if len(streams) > 0 {
			entities := make([]interface{}, 0, 2*len(streams))
			logStreams := make([]coordinator.LogStream, len(streams))
			logStreamStates := make([]coordinator.LogStreamState, len(streams))

			di := ds.Get(c)
			for i, id := range streams {
				logStreams[i].ID = id
				logStreams[i].PopulateState(di, &logStreamStates[i])
				entities = append(entities, &logStreams[i], &logStreamStates[i])
			}

			if err := di.Get(entities); err != nil {
				log.Fields{
					log.ErrorKey: err,
					"count":      len(streams),
				}.Errorf(c, "Failed to load stream descriptors.")
				return nil, grpcutil.Internal
			}

			for sidx, lrs := range idxMap {
				ls := logStreams[sidx]
				lrs.State = buildLogStreamState(&ls, &logStreamStates[sidx])

				lrs.Desc, err = ls.DescriptorValue()
				if err != nil {
					log.Fields{
						log.ErrorKey: err,
						"path":       ls.Path(),
					}.Errorf(c, "Failed to unmarshal descriptor protobuf.")
					return nil, grpcutil.Internal
				}
			}
		}
	}

	log.Fields{
		"count": len(resp.Components),
	}.Infof(c, "List completed successfully.")
	return &resp, nil
}
