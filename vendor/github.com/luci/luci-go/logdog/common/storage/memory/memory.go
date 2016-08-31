// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package memory

import (
	"errors"
	"sync"
	"time"

	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/logdog/common/storage"
	"github.com/luci/luci-go/logdog/common/types"
)

type logStream struct {
	logs        map[types.MessageIndex][]byte
	latestIndex types.MessageIndex
}

type rec struct {
	index types.MessageIndex
	data  []byte
}

type streamKey struct {
	project config.ProjectName
	path    types.StreamPath
}

// Storage is an implementation of the storage.Storage interface that stores
// data in memory.
//
// This is intended for testing, and not intended to be performant.
type Storage struct {
	// MaxGetCount, if not zero, is the maximum number of records to retrieve from
	// a single Get request.
	MaxGetCount int

	// MaxLogAge is the configured maximum log age.
	MaxLogAge time.Duration

	stateMu sync.Mutex
	streams map[streamKey]*logStream
	closed  bool
	err     error
}

var _ storage.Storage = (*Storage)(nil)

// Close implements storage.Storage.
func (s *Storage) Close() {
	s.run(func() error {
		s.closed = true
		return nil
	})
}

// Config implements storage.Storage.
func (s *Storage) Config(cfg storage.Config) error {
	return s.run(func() error {
		s.MaxLogAge = cfg.MaxLogAge
		return nil
	})
}

// Put implements storage.Storage.
func (s *Storage) Put(req storage.PutRequest) error {
	return s.run(func() error {
		ls := s.getLogStreamLocked(req.Project, req.Path, true)

		for i, v := range req.Values {
			index := req.Index + types.MessageIndex(i)
			if _, ok := ls.logs[index]; ok {
				return storage.ErrExists
			}

			clone := make([]byte, len(v))
			copy(clone, v)
			ls.logs[index] = clone
			if index > ls.latestIndex {
				ls.latestIndex = index
			}
		}
		return nil
	})
}

// Get implements storage.Storage.
func (s *Storage) Get(req storage.GetRequest, cb storage.GetCallback) error {
	recs := []*rec(nil)
	err := s.run(func() error {
		ls := s.getLogStreamLocked(req.Project, req.Path, false)
		if ls == nil {
			return storage.ErrDoesNotExist
		}

		limit := len(ls.logs)
		if req.Limit > 0 && req.Limit < limit {
			limit = req.Limit
		}
		if s.MaxGetCount > 0 && s.MaxGetCount < limit {
			limit = s.MaxGetCount
		}

		// Grab all records starting from our start index.
		for idx := req.Index; idx <= ls.latestIndex; idx++ {
			if le, ok := ls.logs[idx]; ok {
				recs = append(recs, &rec{
					index: idx,
					data:  le,
				})
			}

			if len(recs) >= limit {
				break
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Punt all of the records upstream. We copy the data to prevent the
	// callback from accidentally mutating it. We reuse the data buffer to try
	// and catch errors when the callback retains the data.
	for _, r := range recs {
		var dataCopy []byte
		if !req.KeysOnly {
			dataCopy = make([]byte, len(r.data))
			copy(dataCopy, r.data)
		}
		if !cb(r.index, dataCopy) {
			break
		}
	}

	return nil
}

// Tail implements storage.Storage.
func (s *Storage) Tail(project config.ProjectName, path types.StreamPath) ([]byte, types.MessageIndex, error) {
	var r *rec

	// Find the latest log, then return it.
	err := s.run(func() error {
		ls := s.getLogStreamLocked(project, path, false)
		if ls == nil {
			return storage.ErrDoesNotExist
		}

		r = &rec{
			index: ls.latestIndex,
			data:  ls.logs[ls.latestIndex],
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return r.data, r.index, nil
}

// Count returns the number of log records for the given stream.
func (s *Storage) Count(project config.ProjectName, path types.StreamPath) (c int) {
	s.run(func() error {
		if st := s.getLogStreamLocked(project, path, false); st != nil {
			c = len(st.logs)
		}
		return nil
	})
	return
}

// SetErr sets the storage's error value. If not nil, all operations will fail
// with this error.
func (s *Storage) SetErr(err error) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.err = err
}

func (s *Storage) run(f func() error) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if s.err != nil {
		return s.err
	}
	if s.closed {
		return errors.New("storage is closed")
	}
	return f()
}

func (s *Storage) getLogStreamLocked(project config.ProjectName, path types.StreamPath, create bool) *logStream {
	key := streamKey{
		project: project,
		path:    path,
	}

	ls := s.streams[key]
	if ls == nil && create {
		ls = &logStream{
			logs:        map[types.MessageIndex][]byte{},
			latestIndex: -1,
		}

		if s.streams == nil {
			s.streams = map[streamKey]*logStream{}
		}
		s.streams[key] = ls
	}

	return ls
}
