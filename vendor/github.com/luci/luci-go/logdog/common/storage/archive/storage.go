// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package archive implements a storage.Storage instance that retrieves logs
// from a Google Storage archive.
//
// This is a special implementation of storage.Storage, and does not fully
// conform to the API expecations. Namely:
//	- It is read-only. Mutation methods will return storage.ErrReadOnly.
//	- Storage methods ignore the supplied Path argument, instead opting for
//	  the archive configured in its Options.
package archive

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"sort"
	"sync"

	"golang.org/x/net/context"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/data/recordio"
	"github.com/luci/luci-go/common/gcloud/gs"
	"github.com/luci/luci-go/common/iotools"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/common/storage"
	"github.com/luci/luci-go/logdog/common/types"
)

const (
	// maxStreamRecordSize is the maximum record size we're willing to read from
	// our archived log stream. This will help prevent out-of-memory errors if the
	// arhived log stream is malicious or corrupt.
	maxStreamRecordSize = 16 * 1024 * 1024
)

// Options is the set of configuration options for this Storage instance.
//
// Unlike other Storage instances, this is bound to a single archived stream.
// Project and Path parameters in requests will be ignored in favor of the
// Google Storage URLs.
type Options struct {
	// IndexURL is the Google Storage URL for the stream's index.
	IndexURL string
	// StreamURL is the Google Storage URL for the stream's entries.
	StreamURL string

	// Client is the HTTP client to use for authentication.
	//
	// Closing this Storage instance does not close the underlying Client.
	Client gs.Client

	// MaxBytes, if >0, is the maximum number of bytes to fetch in any given
	// request. This should be set for GAE fetches, as large log streams may
	// exceed the urlfetch system's maximum response size otherwise.
	//
	// This is the number of bytes to request, not the number of bytes of log data
	// to return. The difference is that the former includes the RecordIO frame
	// headers.
	MaxBytes int
}

type storageImpl struct {
	*Options
	context.Context

	streamPath gs.Path
	indexPath  gs.Path

	indexMu     sync.Mutex
	index       *logpb.LogIndex
	closeClient bool
}

// New instantiates a new Storage instance, bound to the supplied Options.
func New(ctx context.Context, o Options) (storage.Storage, error) {
	s := storageImpl{
		Options: &o,
		Context: ctx,

		streamPath: gs.Path(o.StreamURL),
		indexPath:  gs.Path(o.IndexURL),
	}

	if !s.streamPath.IsFullPath() {
		return nil, fmt.Errorf("invalid stream URL: %q", s.streamPath)
	}
	if !s.indexPath.IsFullPath() {
		return nil, fmt.Errorf("invalid index URL: %v", s.indexPath)
	}

	return &s, nil
}

func (s *storageImpl) Close() {
	if s.closeClient {
		_ = s.Client.Close()
	}
}

func (s *storageImpl) Config(storage.Config) error  { return storage.ErrReadOnly }
func (s *storageImpl) Put(storage.PutRequest) error { return storage.ErrReadOnly }

func (s *storageImpl) Get(req storage.GetRequest, cb storage.GetCallback) error {
	idx, err := s.getIndex()
	if err != nil {
		return err
	}

	// Identify the byte offsets that we want to fetch from the entries stream.
	st := s.buildGetStrategy(&req, idx)
	if st.lastIndex == -1 || req.Index > st.lastIndex {
		// No records to read.
		return nil
	}

	offset := int64(st.startOffset)
	log.Fields{
		"offset": offset,
		"length": st.length(),
		"path":   s.streamPath,
	}.Debugf(s, "Creating stream reader for range.")
	r, err := s.Client.NewReader(s.streamPath, offset, st.length())
	if err != nil {
		log.WithError(err).Errorf(s, "Failed to create stream Reader.")
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			log.WithError(err).Warningf(s, "Error closing stream Reader.")
		}
	}()
	cr := iotools.CountingReader{Reader: r}
	rio := recordio.NewReader(&cr, maxStreamRecordSize)

	buf := bytes.Buffer{}
	le := logpb.LogEntry{}
	max := st.count
	for {
		offset += cr.Count()

		sz, r, err := rio.ReadFrame()
		switch err {
		case nil:
			break

		case io.EOF:
			return nil

		default:
			log.Fields{
				log.ErrorKey: err,
				"index":      idx,
				"offset":     offset,
			}.Errorf(s, "Failed to read next frame.")
			return err
		}

		buf.Reset()
		buf.Grow(int(sz))
		if _, err := buf.ReadFrom(r); err != nil {
			log.Fields{
				log.ErrorKey: err,
				"offset":     offset,
				"frameSize":  sz,
			}.Errorf(s, "Failed to read frame data.")
			return err
		}

		if err := proto.Unmarshal(buf.Bytes(), &le); err != nil {
			log.Fields{
				log.ErrorKey: err,
				"offset":     offset,
				"frameSize":  sz,
			}.Errorf(s, "Failed to unmarshal log data.")
			return err
		}

		idx := types.MessageIndex(le.StreamIndex)
		if idx < req.Index {
			// Skip this entry, as it's before the first requested entry.
			continue
		}

		d := make([]byte, buf.Len())
		copy(d, buf.Bytes())
		if !cb(idx, d) {
			break
		}

		// Enforce our limit, if one is supplied.
		if max > 0 {
			max--
			if max == 0 {
				break
			}
		}
	}
	return nil
}

func (s *storageImpl) Tail(project config.ProjectName, path types.StreamPath) ([]byte, types.MessageIndex, error) {
	idx, err := s.getIndex()
	if err != nil {
		return nil, 0, err
	}

	// Get the offset of the last record.
	if len(idx.Entries) == 0 {
		return nil, 0, nil
	}
	lle := idx.Entries[len(idx.Entries)-1]

	// Get a Reader for the Tail entry.
	r, err := s.Client.NewReader(s.streamPath, int64(lle.Offset), -1)
	if err != nil {
		log.Fields{
			log.ErrorKey: err,
			"offset":     lle.Offset,
		}.Errorf(s, "Failed to create reader.")
		return nil, 0, err
	}
	defer func() {
		if err := r.Close(); err != nil {
			log.WithError(err).Warningf(s, "Failed to close Reader.")
		}
	}()

	rio := recordio.NewReader(r, maxStreamRecordSize)
	d, err := rio.ReadFrameAll()
	if err != nil {
		log.WithError(err).Errorf(s, "Failed to read log frame.")
		return nil, 0, err
	}

	return d, types.MessageIndex(lle.StreamIndex), nil
}

// getIndex returns the cached log stream index, fetching it if necessary.
func (s *storageImpl) getIndex() (*logpb.LogIndex, error) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()

	if s.index == nil {
		r, err := s.Client.NewReader(s.indexPath, 0, -1)
		if err != nil {
			log.WithError(err).Errorf(s, "Failed to create index Reader.")
			return nil, err
		}
		defer func() {
			if err := r.Close(); err != nil {
				log.WithError(err).Warningf(s, "Error closing index Reader.")
			}
		}()
		indexData, err := ioutil.ReadAll(r)
		if err != nil {
			log.WithError(err).Errorf(s, "Failed to read index.")
			return nil, err
		}

		index := logpb.LogIndex{}
		if err := proto.Unmarshal(indexData, &index); err != nil {
			log.WithError(err).Errorf(s, "Failed to unmarshal index.")
			return nil, err
		}

		s.index = &index
	}
	return s.index, nil
}

type getStrategy struct {
	// startOffset is the beginning byte offset of the log entry stream.
	startOffset uint64
	// endOffset is the ending byte offset of the log entry stream.
	endOffset uint64

	// count is the number of log entries that will be fetched.
	count int
	// lastIndex is the last log entry index in the stream. This will be -1 if
	// there are no entries in the stream.
	lastIndex types.MessageIndex
}

func (gs *getStrategy) length() int64 {
	if gs.startOffset < gs.endOffset {
		return int64(gs.endOffset - gs.startOffset)
	}
	return -1
}

// setCount sets the `count` field. If called multiple times, the smallest
// assigned value will be retained.
func (gs *getStrategy) setCount(v int) {
	if gs.count <= 0 || gs.count > v {
		gs.count = v
	}
}

// setEndOffset sets the `length` field. If called multiple times, the smallest
// assigned value will be retained.
func (gs *getStrategy) setEndOffset(v uint64) {
	if gs.endOffset == 0 || gs.endOffset > v {
		gs.endOffset = v
	}
}

func (s *storageImpl) buildGetStrategy(req *storage.GetRequest, idx *logpb.LogIndex) *getStrategy {
	st := getStrategy{}

	if len(idx.Entries) == 0 {
		st.lastIndex = -1
		return &st
	}

	st.lastIndex = types.MessageIndex(idx.Entries[len(idx.Entries)-1].StreamIndex)
	startIdx := indexEntryFor(idx.Entries, req.Index)
	if startIdx < 0 {
		startIdx = 0
	}
	le := idx.Entries[startIdx]
	st.startOffset = le.Offset

	// Determine an upper bound based on our limits.
	//
	// If we have a count limit, and we have enough index entries to upper-bound
	// our stream based on that limit, use that. Note that this may overshoot if
	// the index and/or stream is sparse. We know for sure that we have one
	// LogEntry per index entry, so that's the best we can do.
	if req.Limit > 0 {
		if ub := startIdx + req.Limit; ub < len(idx.Entries) {
			st.setEndOffset(idx.Entries[ub].Offset)
		}
		st.setCount(req.Limit)
	}

	// If we have a byte limit, count the entry sizes until we reach that limit.
	if mb := int64(s.MaxBytes); mb > 0 {
		mb := uint64(mb)

		for i, e := range idx.Entries[startIdx:] {
			if e.Offset < st.startOffset {
				// This shouldn't really happen, but it could happen if there is a
				// corrupt index.
				continue
			}

			// Calculate the request offset and truncate if we've exceeded our maximum
			// request bytes.
			if size := (e.Offset - st.startOffset); size > mb {
				st.setEndOffset(e.Offset)
				st.setCount(i)
				break
			}
		}
	}
	return &st
}

// indexEntryFor identifies the log index entry closest (<=) to the specified
// index.
//
// If the first index entry is greater than our search index, -1 will be
// returned. This should never happen in practice, though, since our index
// construction always indexes log entry #0.
//
// It does this by performing a binary search over the index entries.
func indexEntryFor(entries []*logpb.LogIndex_Entry, i types.MessageIndex) int {
	ui := uint64(i)
	s := sort.Search(len(entries), func(i int) bool {
		return entries[i].StreamIndex > ui
	})

	// The returned index is the one immediately after the index that we want. If
	// our search returned 0, the first index entry is > our search entry, and we
	// will return nil.
	return s - 1
}
