// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"fmt"

	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"
)

// Coordinator is an interface to a remote LogDog Coordinator service. This is
// a simplified version of the Coordinator's Service API tailored specifically
// to the Collector's usage.
//
// All Coordiantor methods will return transient-wrapped errors if appropriate.
type Coordinator interface {
	// RegisterStream registers a log stream state.
	RegisterStream(c context.Context, s *LogStreamState, desc []byte) (*LogStreamState, error)
	// TerminateStream registers the terminal index of a log stream state.
	TerminateStream(c context.Context, s *TerminateRequest) error
}

// LogStreamState is a local representation of a remote stream's state. It is a
// subset of the remote state with the necessary elements for the Collector to
// operate and update.
type LogStreamState struct {
	// Project is the log stream project.
	Project config.ProjectName
	// Path is the log stream path.
	Path types.StreamPath

	// ID is the stream's Coordinator ID. This is returned by the Coordiantor.
	ID string
	// ProtoVersion is the stream protocol version string.
	ProtoVersion string
	// Secret is the log stream's prefix secret.
	Secret types.PrefixSecret
	// TerminalIndex is an optional terminal index to register alongside the
	// stream. If this is <0, the stream will be registered without a terminal
	// index.
	TerminalIndex types.MessageIndex

	// Archived is true if the log stream has been archived. This is returned by
	// the remote service.
	Archived bool
	// Purged is true if the log stream has been archived. This is returned by
	// the remote service.
	Purged bool
}

// TerminateRequest is a local representation of a Coordinator stream
// termination request.
type TerminateRequest struct {
	Project config.ProjectName // Project name.
	Path    types.StreamPath   // Stream path. Needed for cache lookup.

	// ID is the stream's Coordinator ID, as indicated by the Coordinator.
	ID string
	// TerminalIndex is the terminal index to register.
	//
	// This must be >= 0, else there is no point in sending the TerminateStream
	// request.
	TerminalIndex types.MessageIndex
	// Secret is the log stream's prefix secret.
	Secret types.PrefixSecret
}

type coordinatorImpl struct {
	c logdog.ServicesClient
}

// NewCoordinator returns a Coordinator implementation that uses a
// logdog.ServicesClient.
func NewCoordinator(s logdog.ServicesClient) Coordinator {
	return &coordinatorImpl{s}
}

func (c *coordinatorImpl) RegisterStream(ctx context.Context, s *LogStreamState, desc []byte) (*LogStreamState, error) {
	// Client-side validate our parameters.
	if err := s.Project.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate project: %s", err)
	}
	if err := s.Path.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate path: %s", err)
	}
	if err := s.Secret.Validate(); err != nil {
		return nil, fmt.Errorf("invalid secret: %s", err)
	}

	req := logdog.RegisterStreamRequest{
		Project:       string(s.Project),
		Secret:        []byte(s.Secret),
		ProtoVersion:  s.ProtoVersion,
		Desc:          desc,
		TerminalIndex: int64(s.TerminalIndex),
	}

	resp, err := c.c.RegisterStream(ctx, &req)
	switch {
	case err != nil:
		return nil, err
	case resp.State == nil:
		return nil, errors.New("missing stream state")
	}

	return &LogStreamState{
		ID:            resp.Id,
		Project:       s.Project,
		Path:          s.Path,
		ProtoVersion:  resp.State.ProtoVersion,
		Secret:        types.PrefixSecret(resp.State.Secret),
		TerminalIndex: types.MessageIndex(resp.State.TerminalIndex),
		Archived:      resp.State.Archived,
		Purged:        resp.State.Purged,
	}, nil
}

func (c *coordinatorImpl) TerminateStream(ctx context.Context, r *TerminateRequest) error {
	// Client-side validate our parameters.
	if err := r.Project.Validate(); err != nil {
		return fmt.Errorf("failed to validate project: %s", err)
	}
	if r.ID == "" {
		return errors.New("missing stream ID")
	}
	if r.TerminalIndex < 0 {
		return errors.New("refusing to terminate with non-terminal state")
	}

	req := logdog.TerminateStreamRequest{
		Project:       string(r.Project),
		Id:            r.ID,
		Secret:        []byte(r.Secret),
		TerminalIndex: int64(r.TerminalIndex),
	}

	if _, err := c.c.TerminateStream(ctx, &req); err != nil {
		return err
	}
	return nil
}
