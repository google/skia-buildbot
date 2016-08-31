// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package archive

import (
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/logdog/api/logpb"
)

// indexBuilder is a stateful engine that constructs an archival index.
type indexBuilder struct {
	*Manifest
	index logpb.LogIndex

	lastPrefixIndex uint64
	lastStreamIndex uint64
	lastBytes       uint64

	sizeFunc func(proto.Message) int
}

func (i *indexBuilder) addLogEntry(le *logpb.LogEntry, offset int64) {
	// Only calculate the size if we actually use it.
	if i.ByteRange > 0 {
		i.lastBytes += uint64(i.size(le))
	}

	// Do we index this LogEntry?
	if len(i.index.Entries) > 0 {
		if !((i.StreamIndexRange > 0 && (le.StreamIndex-i.lastStreamIndex) >= uint64(i.StreamIndexRange)) ||
			(i.PrefixIndexRange > 0 && (le.PrefixIndex-i.lastPrefixIndex) >= uint64(i.PrefixIndexRange)) ||
			(i.ByteRange > 0 && i.lastBytes >= uint64(i.ByteRange))) {
			// Not going to index this entry.
			return
		}

		i.lastBytes = 0
	}

	i.index.Entries = append(i.index.Entries, &logpb.LogIndex_Entry{
		Sequence:    le.Sequence,
		PrefixIndex: le.PrefixIndex,
		StreamIndex: le.StreamIndex,
		Offset:      uint64(offset),
		TimeOffset:  le.TimeOffset,
	})

	// Update our counters.
	i.lastStreamIndex = le.StreamIndex
	i.lastPrefixIndex = le.PrefixIndex
}

func (i *indexBuilder) emit(w io.Writer) error {
	d, err := proto.Marshal(&i.index)
	if err != nil {
		return err
	}

	if _, err := w.Write(d); err != nil {
		return err
	}
	return nil
}

func (i *indexBuilder) size(pb proto.Message) int {
	if f := i.sizeFunc; f != nil {
		return f(pb)
	}
	return proto.Size(pb)
}
