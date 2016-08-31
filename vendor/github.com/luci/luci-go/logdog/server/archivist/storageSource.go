// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package archivist

import (
	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/config"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/common/archive"
	"github.com/luci/luci-go/logdog/common/storage"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"
)

// storageSource is an archive.LogEntrySource that pulls log entries from
// intermediate storage via its storage.Storage instance.
type storageSource struct {
	context.Context

	st            storage.Storage    // the storage instance to read from
	project       config.ProjectName // the path of the log stream
	path          types.StreamPath   // the path of the log stream
	terminalIndex types.MessageIndex // if >= 0, discard logs beyond this

	buf           []*logpb.LogEntry
	lastIndex     types.MessageIndex
	logEntryCount int64
}

func (s *storageSource) bufferEntries(start types.MessageIndex) error {
	bytes := 0

	req := storage.GetRequest{
		Project: s.project,
		Path:    s.path,
		Index:   start,
	}
	return s.st.Get(req, func(idx types.MessageIndex, d []byte) bool {
		le := logpb.LogEntry{}
		if err := proto.Unmarshal(d, &le); err != nil {
			log.Fields{
				log.ErrorKey:  err,
				"streamIndex": idx,
			}.Errorf(s, "Failed to unmarshal LogEntry.")
			return false
		}
		s.buf = append(s.buf, &le)

		// Stop loading if we've reached or exceeded our buffer size.
		bytes += len(d)
		return bytes < storageBufferSize
	})
}

func (s *storageSource) NextLogEntry() (*logpb.LogEntry, error) {
	if len(s.buf) == 0 {
		s.buf = s.buf[:0]
		if err := s.bufferEntries(s.lastIndex + 1); err != nil {
			if err == storage.ErrDoesNotExist {
				log.Warningf(s, "Archive target stream does not exist in intermediate storage.")
				return nil, archive.ErrEndOfStream
			}

			log.WithError(err).Errorf(s, "Failed to retrieve log stream from storage.")
			return nil, err
		}
	}

	// If we have no more buffered entries, we have exhausted our log stream.
	if len(s.buf) == 0 {
		// If we have a terminal index, but we didn't actually emit that index,
		// mark that we have missing entries.
		if s.terminalIndex >= 0 && s.lastIndex != s.terminalIndex {
			log.Fields{
				"terminalIndex": s.terminalIndex,
				"lastIndex":     s.lastIndex,
			}.Warningf(s, "Log stream stopped before terminal index.")
		} else {
			log.Fields{
				"lastIndex": s.lastIndex,
			}.Debugf(s, "Encountered end of stream.")
		}

		return nil, archive.ErrEndOfStream
	}

	// Pop the next log entry and advance the stream.
	var le *logpb.LogEntry
	le, s.buf = s.buf[0], s.buf[1:]

	// If we're enforcing a maximum terminal index, return end of stream if this
	// LogEntry exceeds that index.
	sidx := types.MessageIndex(le.StreamIndex)
	if s.terminalIndex >= 0 && sidx > s.terminalIndex {
		log.Fields{
			"index":         sidx,
			"terminalIndex": s.terminalIndex,
		}.Warningf(s, "Discarding log entries beyond expected terminal index.")
		return nil, archive.ErrEndOfStream
	}

	s.lastIndex = sidx
	s.logEntryCount++
	return le, nil
}
