// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"errors"
	"sync"
	"time"

	"github.com/luci/luci-go/common/clock"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/client/coordinator"
	"github.com/luci/luci-go/logdog/common/fetcher"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"
)

const (
	// defaultBytes is the default maximum number of bytes to request per fetch
	// round.
	defaultBytes = 1024 * 1024 * 1 // 1 MB

	noStreamDelay = 5 * time.Second
)

// coordinatorSource is a fetcher.Source implementation that uses the
// Coordiantor API.
type coordinatorSource struct {
	sync.Mutex

	stream *coordinator.Stream
	tidx   types.MessageIndex

	state coordinator.LogStream
}

func (s *coordinatorSource) LogEntries(c context.Context, req *fetcher.LogRequest) (
	[]*logpb.LogEntry, types.MessageIndex, error) {
	s.Lock()
	defer s.Unlock()

	p := coordinator.NewGetParams().Limit(int(req.Bytes), req.Count).Index(req.Index)

	// If we haven't terminated, use this opportunity to fetch/update our stream
	// state.
	if s.tidx < 0 {
		p = p.State(&s.state)
	}

	for {
		logs, err := s.stream.Get(c, p)
		switch err {
		case nil:
			if s.state.State != nil && s.tidx < 0 {
				s.tidx = s.state.State.TerminalIndex
			}
			return logs, s.tidx, nil

		case coordinator.ErrNoSuchStream:
			log.WithError(err).Warningf(c, "Stream does not exist. Sleeping pending registration.")

			// Delay, interrupting if our Context is interrupted.
			if tr := <-clock.After(c, noStreamDelay); tr.Incomplete() {
				return nil, 0, tr.Err
			}

		default:
			if err != nil {
				return nil, 0, err
			}
		}
	}
}

func (s *coordinatorSource) getState() coordinator.LogStream {
	s.Lock()
	defer s.Unlock()
	return s.state
}

func (s *coordinatorSource) descriptor() (*logpb.LogStreamDescriptor, error) {
	if d := s.state.Desc; d != nil {
		return d, nil
	}
	return nil, errors.New("no descriptor loaded")
}
