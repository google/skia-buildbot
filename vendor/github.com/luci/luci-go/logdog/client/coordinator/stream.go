// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"errors"
	"fmt"
	"time"

	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/logs/v1"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"
)

// StreamState represents the client-side state of the log stream.
//
// It is a type-promoted version of logdog.LogStreamState.
type StreamState struct {
	// Created is the time, represented as a UTC RFC3339 string, when the log
	// stream was created.
	Created time.Time
	// Updated is the time, represented as a UTC RFC3339 string, when the log
	// stream was last updated.
	Updated time.Time

	// TerminalIndex is the stream index of the log stream's terminal message. If
	// its value is <0, then the log stream has not terminated yet.
	// In this case, FinishedIndex is the index of that terminal message.
	TerminalIndex types.MessageIndex

	// Archived is true if the stream is marked as archived.
	Archived bool
	// ArchiveIndexURL is the Google Storage URL where the log stream's index is
	// archived.
	ArchiveIndexURL string
	// ArchiveStreamURL is the Google Storage URL where the log stream's raw
	// stream data is archived. If this is not empty, the log stream is considered
	// archived.
	ArchiveStreamURL string
	// ArchiveDataURL is the Google Storage URL where the log stream's assembled
	// data is archived. If this is not empty, the log stream is considered
	// archived.
	ArchiveDataURL string

	// Purged indicates the purged state of a log. A log that has been purged is
	// only acknowledged to administrative clients.
	Purged bool
}

// LogStream is returned metadata about a log stream.
type LogStream struct {
	// Project is the log stream's project.
	Project config.ProjectName
	// Path is the path of the log stream.
	Path types.StreamPath

	// Desc is the log stream's descriptor.
	Desc *logpb.LogStreamDescriptor

	// State is the stream's current state.
	State *StreamState
}

func loadLogStream(proj string, path types.StreamPath, s *logdog.LogStreamState, d *logpb.LogStreamDescriptor) *LogStream {
	ls := LogStream{
		Project: config.ProjectName(proj),
		Path:    path,
		Desc:    d,
	}
	if s != nil {
		st := StreamState{
			Created:       s.Created.Time(),
			TerminalIndex: types.MessageIndex(s.TerminalIndex),
			Purged:        s.Purged,
		}
		if a := s.Archive; a != nil {
			st.Archived = true
			st.ArchiveIndexURL = a.IndexUrl
			st.ArchiveStreamURL = a.StreamUrl
			st.ArchiveDataURL = a.DataUrl
		}

		ls.State = &st
	}
	return &ls
}

// Stream is an interface to Coordinator stream-level commands. It is bound to
// and operates on a single log stream path.
type Stream struct {
	// c is the Coordinator instance that this Stream is bound to.
	c *Client

	// project is this stream's project.
	project config.ProjectName
	// path is the log stream's prefix.
	path types.StreamPath
}

// State fetches the LogStreamDescriptor for a given log stream.
func (s *Stream) State(ctx context.Context) (*LogStream, error) {
	req := logdog.GetRequest{
		Project:  string(s.project),
		Path:     string(s.path),
		State:    true,
		LogCount: -1, // Don't fetch any logs.
	}

	resp, err := s.c.C.Get(ctx, &req)
	if err != nil {
		return nil, normalizeError(err)
	}

	path := types.StreamPath(req.Path)
	if desc := resp.Desc; desc != nil {
		path = desc.Path()
	}
	return loadLogStream(resp.Project, path, resp.State, resp.Desc), nil
}

// Get retrieves log stream entries from the Coordinator. The supplied
// parameters shape which entries are requested and what information is
// returned.
func (s *Stream) Get(ctx context.Context, p *StreamGetParams) ([]*logpb.LogEntry, error) {
	if p == nil {
		p = &StreamGetParams{}
	}

	req := p.r
	req.Project = string(s.project)
	req.Path = string(s.path)
	if p.stateP != nil {
		req.State = true
	}

	resp, err := s.c.C.Get(ctx, &req)
	if err != nil {
		return nil, normalizeError(err)
	}
	if err := loadStatePointer(p.stateP, resp); err != nil {
		return nil, err
	}
	return resp.Logs, nil
}

// Tail performs a tail call, returning the last log entry in the stream. If
// stateP is not nil, the stream's state will be requested and loaded into the
// variable.
func (s *Stream) Tail(ctx context.Context, stateP *LogStream) (*logpb.LogEntry, error) {
	req := logdog.TailRequest{
		Project: string(s.project),
		Path:    string(s.path),
	}
	if stateP != nil {
		req.State = true
	}

	resp, err := s.c.C.Tail(ctx, &req)
	if err != nil {
		return nil, normalizeError(err)
	}
	if err := loadStatePointer(stateP, resp); err != nil {
		return nil, err
	}

	switch len(resp.Logs) {
	case 0:
		return nil, nil

	case 1:
		return resp.Logs[0], nil

	default:
		return nil, fmt.Errorf("tail call returned %d logs", len(resp.Logs))
	}
}

func loadStatePointer(stateP *LogStream, resp *logdog.GetResponse) error {
	if stateP == nil {
		return nil
	}

	if resp.Desc == nil {
		return errors.New("Requested descriptor was not returned")
	}
	if resp.State == nil {
		return errors.New("Requested state was not returned")
	}
	ls := loadLogStream(resp.Project, resp.Desc.Path(), resp.State, resp.Desc)
	*stateP = *ls
	return nil
}
