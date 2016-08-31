// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package services

import (
	"crypto/subtle"

	"github.com/golang/protobuf/proto"
	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/logdog/api/config/svcconfig"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	"github.com/luci/luci-go/logdog/appengine/coordinator/config"
	"github.com/luci/luci-go/logdog/appengine/coordinator/endpoints"
	"github.com/luci/luci-go/logdog/appengine/coordinator/hierarchy"
	"github.com/luci/luci-go/logdog/appengine/coordinator/mutations"
	"github.com/luci/luci-go/logdog/common/types"
	"github.com/luci/luci-go/tumble"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
)

func buildLogStreamState(ls *coordinator.LogStream, lst *coordinator.LogStreamState) *logdog.LogStreamState {
	st := logdog.LogStreamState{
		ProtoVersion:  ls.ProtoVersion,
		Secret:        lst.Secret,
		TerminalIndex: lst.TerminalIndex,
		Archived:      lst.ArchivalState().Archived(),
		Purged:        ls.Purged,
	}
	if !lst.Terminated() {
		st.TerminalIndex = -1
	}
	return &st
}

// RegisterStream is an idempotent stream state register operation.
//
// Successive operations will succeed if they have the correct secret for their
// registered stream, regardless of whether the contents of their request match
// the currently registered state.
func (s *server) RegisterStream(c context.Context, req *logdog.RegisterStreamRequest) (*logdog.RegisterStreamResponse, error) {
	var path types.StreamPath

	// Unmarshal the serialized protobuf.
	var desc logpb.LogStreamDescriptor
	switch req.ProtoVersion {
	case logpb.Version:
		if err := proto.Unmarshal(req.Desc, &desc); err != nil {
			log.Fields{
				log.ErrorKey:   err,
				"protoVersion": req.ProtoVersion,
			}.Errorf(c, "Failed to unmarshal descriptor protobuf.")
			return nil, grpcutil.Errf(codes.InvalidArgument, "Failed to unmarshal protobuf.")
		}

	default:
		log.Fields{
			"protoVersion": req.ProtoVersion,
		}.Errorf(c, "Unrecognized protobuf version.")
		return nil, grpcutil.Errf(codes.InvalidArgument, "Unrecognized protobuf version: %q", req.ProtoVersion)
	}

	path = desc.Path()
	logStreamID := coordinator.LogStreamID(path)
	log.Fields{
		"project":       req.Project,
		"path":          path,
		"prospectiveID": logStreamID,
		"terminalIndex": req.TerminalIndex,
	}.Infof(c, "Registration request for log stream.")

	if err := desc.Validate(true); err != nil {
		return nil, grpcutil.Errf(codes.InvalidArgument, "Invalid log stream descriptor: %s", err)
	}
	prefix, _ := path.Split()

	// Load our service and project configs.
	cfg, err := coordinator.GetServices(c).Config(c)
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to load configuration.")
		return nil, grpcutil.Internal
	}

	pcfg, err := coordinator.CurrentProjectConfig(c)
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to load current project configuration.")
		return nil, grpcutil.Internal
	}

	// Load our Prefix. It must be registered.
	di := ds.Get(c)

	pfx := &coordinator.LogPrefix{ID: coordinator.LogPrefixID(prefix)}
	if err := di.Get(pfx); err != nil {
		log.Fields{
			log.ErrorKey: err,
			"id":         pfx.ID,
			"prefix":     prefix,
		}.Errorf(c, "Failed to load log stream prefix.")
		if err == ds.ErrNoSuchEntity {
			return nil, grpcutil.Errf(codes.FailedPrecondition, "prefix is not registered")
		}
		return nil, grpcutil.Internal
	}

	// If we're past prefix's expiration, reject this stream.
	//
	// If the prefix doesn't have an expiration, use its creation time and apply
	// the maximum expiration.
	expirationTime := pfx.Expiration
	if expirationTime.IsZero() {
		expiration := endpoints.MinDuration(cfg.Coordinator.PrefixExpiration, pcfg.PrefixExpiration)
		if expiration > 0 {
			expirationTime = pfx.Created.Add(expiration)
		}
	}
	if now := clock.Now(c); expirationTime.IsZero() || !now.Before(expirationTime) {
		log.Fields{
			"prefix":     pfx.Prefix,
			"expiration": expirationTime,
		}.Errorf(c, "The log stream Prefix has expired.")
		return nil, grpcutil.Errf(codes.FailedPrecondition, "prefix has expired")
	}

	// The prefix secret must match the request secret. If it does, we know this
	// is a legitimate registration attempt.
	if subtle.ConstantTimeCompare(pfx.Secret, req.Secret) != 1 {
		log.Errorf(c, "Request secret does not match prefix secret.")
		return nil, grpcutil.Errf(codes.InvalidArgument, "invalid secret")
	}

	// Check for registration, and that the prefix did not expire
	// (non-transactional).
	ls := &coordinator.LogStream{ID: logStreamID}
	lst := ls.State(di)

	if err := di.Get(ls, lst); err != nil {
		if !anyNoSuchEntity(err) {
			log.WithError(err).Errorf(c, "Failed to check for log stream.")
			return nil, err
		}

		// The stream is not registered. Perform a transactional registration via
		// mutation.
		//
		// Determine which hierarchy components we need to add.
		comps := hierarchy.Components(path, true)
		if comps, err = hierarchy.Missing(di, comps); err != nil {
			log.WithError(err).Warningf(c, "Failed to probe for missing hierarchy components.")
		}

		// Before we go into transaction, try and put these entries. This should not
		// be contested, since components don't share an entity root.
		if err := hierarchy.PutMulti(di, comps); err != nil {
			log.WithError(err).Errorf(c, "Failed to add missing hierarchy components.")
			return nil, grpcutil.Internal
		}

		// The stream does not exist. Proceed with transactional registration.
		err = tumble.RunMutation(c, &registerStreamMutation{
			RegisterStreamRequest: req,
			cfg:  cfg,
			pcfg: pcfg,
			desc: &desc,
			pfx:  pfx,
			lst:  lst,
			ls:   ls,
		})
		if err != nil {
			log.Fields{
				log.ErrorKey: err,
			}.Errorf(c, "Failed to register LogStream.")
			return nil, err
		}
	}

	return &logdog.RegisterStreamResponse{
		Id:    string(ls.ID),
		State: buildLogStreamState(ls, lst),
	}, nil
}

type registerStreamMutation struct {
	*logdog.RegisterStreamRequest

	cfg  *config.Config
	pcfg *svcconfig.ProjectConfig

	desc *logpb.LogStreamDescriptor
	pfx  *coordinator.LogPrefix
	ls   *coordinator.LogStream
	lst  *coordinator.LogStreamState
}

func (m *registerStreamMutation) RollForward(c context.Context) ([]tumble.Mutation, error) {
	di := ds.Get(c)

	// Load our state and stream (transactional).
	switch err := di.Get(m.ls, m.lst); {
	case err == nil:
		// The stream is already registered.
		return nil, nil

	case !anyNoSuchEntity(err):
		log.WithError(err).Errorf(c, "Failed to check for stream registration (transactional).")
		return nil, err
	}

	// The stream is not yet registered.
	log.Infof(c, "Registering new log stream.")

	// Construct our LogStreamState.
	now := clock.Now(c).UTC()
	m.lst.Created = now
	m.lst.Updated = now
	m.lst.Secret = m.pfx.Secret // Copy Prefix Secret to reduce datastore Gets.

	// Construct our LogStream.
	m.ls.Created = now
	m.ls.ProtoVersion = m.ProtoVersion

	if err := m.ls.LoadDescriptor(m.desc); err != nil {
		log.Fields{
			log.ErrorKey: err,
		}.Errorf(c, "Failed to load descriptor into LogStream.")
		return nil, grpcutil.Errf(codes.InvalidArgument, "Failed to load descriptor.")
	}

	// If our registration request included a terminal index, terminate the
	// log stream state as well.
	if m.TerminalIndex >= 0 {
		log.Fields{
			"terminalIndex": m.TerminalIndex,
		}.Debugf(c, "Registration request included terminal index.")

		m.lst.TerminalIndex = m.TerminalIndex
		m.lst.TerminatedTime = now
	} else {
		m.lst.TerminalIndex = -1
	}

	if err := di.Put(m.ls, m.lst); err != nil {
		log.Fields{
			log.ErrorKey: err,
		}.Errorf(c, "Failed to Put LogStream.")
		return nil, grpcutil.Internal
	}

	// Add a named delayed mutation to archive this stream if it's not archived
	// yet.
	//
	// If the registration did not include a terminal index, this will be our
	// pessimistic archival request, scheduled on registration to catch streams
	// that don't expire. This mutation will be replaced by the optimistic
	// archival mutation when/if the stream is terminated via TerminateStream.
	//
	// If the registration included a terminal index, apply our standard
	// parameters to the archival. Since TerminateStream will not be called,
	// this will be our formal optimistic archival task.
	params := standardArchivalParams(m.cfg, m.pcfg)
	cat := mutations.CreateArchiveTask{
		ID: m.ls.ID,
	}
	if m.TerminalIndex < 0 {
		// No terminal index, schedule pessimistic cleanup archival.
		cat.Expiration = now.Add(params.CompletePeriod)

		log.Fields{
			"deadline": cat.Expiration,
		}.Debugf(c, "Scheduling cleanup archival mutation.")
	} else {
		// Terminal index, schedule optimistic archival (mirrors TerminateStream).
		cat.SettleDelay = params.SettleDelay
		cat.CompletePeriod = params.CompletePeriod

		// Schedule this mutation to execute after our settle delay.
		cat.Expiration = now.Add(params.SettleDelay)

		log.Fields{
			"settleDelay":    cat.SettleDelay,
			"completePeriod": cat.CompletePeriod,
			"scheduledAt":    cat.Expiration,
		}.Debugf(c, "Scheduling archival mutation.")
	}

	aeParent, aeName := cat.TaskName(di)
	if err := tumble.PutNamedMutations(c, aeParent, map[string]tumble.Mutation{aeName: &cat}); err != nil {
		log.WithError(err).Errorf(c, "Failed to write named mutations.")
		return nil, grpcutil.Internal
	}

	return nil, nil
}

func (m *registerStreamMutation) Root(c context.Context) *ds.Key {
	return ds.Get(c).KeyForObj(m.ls)
}
