// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"fmt"
	"time"

	"github.com/luci/luci-go/common/data/chunkstream"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"
	"github.com/luci/luci-go/logdog/common/types"
)

// constraints is the set of Constraints to apply when generating a LogEntry.
type constraints struct {
	// limit is the maximum size, in bytes, of the serialized LogEntry protobuf
	// that may be produced.
	limit int

	// allowSplit indicates that bundles should be generated to fill as much of
	// the specified space as possible, splitting them across multiple bundles if
	// necessary.
	//
	// The parser may choose to forego bundling if the result is very suboptimal,
	// but is encouraged to fill the space if it's reasonable.
	allowSplit bool

	// closed means that bundles should be aggressively generated with the
	// expectation that no further data will be buffered. It is only relevant
	// if allowSplit is also true.
	closed bool
}

// parser is a stateful presence bound to a single log stream. A parser yields
// LogEntry messages one at a time and shapes them based on constraints.
//
// parser instances are owned by a single Stream and are not goroutine-safe.
type parser interface {
	// appendData adds a data chunk to this parser's chunk.Buffer.
	appendData(Data)

	// nextEntry returns the next LogEntry in the stream.
	//
	// This method may return nil if there is insuffuicient data to produce a
	// LogEntry given the
	nextEntry(*constraints) (*logpb.LogEntry, error)

	bufferedBytes() int64

	firstChunkTime() (time.Time, bool)
}

func newParser(p *streamproto.Properties, c *counter) (parser, error) {
	base := baseParser{
		counter:  c,
		timeBase: p.Timestamp.Time(),
	}

	switch p.StreamType {
	case logpb.StreamType_TEXT:
		return &textParser{
			baseParser: base,
		}, nil

	case logpb.StreamType_BINARY:
		return &binaryParser{
			baseParser: base,
		}, nil

	case logpb.StreamType_DATAGRAM:
		return &datagramParser{
			baseParser: base,
			maxSize:    int64(types.MaxDatagramSize),
		}, nil

	default:
		return nil, fmt.Errorf("unknown stream type: %v", p.StreamType)
	}
}

// baseParser is a common set of parser capabilities.
type baseParser struct {
	chunkstream.Buffer

	counter *counter

	timeBase  time.Time
	nextIndex uint64
}

func (p *baseParser) baseLogEntry(ts time.Time) *logpb.LogEntry {
	e := logpb.LogEntry{
		TimeOffset:  google.NewDuration(ts.Sub(p.timeBase)),
		PrefixIndex: uint64(p.counter.next()),
		StreamIndex: p.nextIndex,
	}
	p.nextIndex++
	return &e
}

func (p *baseParser) appendData(d Data) {
	p.Append(d)
}

func (p *baseParser) bufferedBytes() int64 {
	return p.Len()
}

func (p *baseParser) firstChunkTime() (time.Time, bool) {
	// Get the first data chunk in our Buffer.
	chunk := p.FirstChunk()
	if chunk == nil {
		return time.Time{}, false
	}

	return chunk.(Data).Timestamp(), true
}

func memoryCorruptionIf(cond bool, err error) {
	if cond {
		memoryCorruption(err)
	}
}

func memoryCorruption(err error) {
	if err != nil {
		panic(fmt.Errorf("bundler: memory corruption: %s", err))
	}
}
