// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/logs/v1"
	"github.com/luci/luci-go/logdog/common/types"
)

// StreamGetParams is an accumulating set of Stream Get request parameters.
type StreamGetParams struct {
	// r is the Get request to populate.
	r logdog.GetRequest

	// stateP is the stream state pointer. It is set by State, and, if supplied,
	// will cause the log request to return stream state.
	stateP *LogStream
}

// NewGetParams returns a new StreamGetParams instance.
func NewGetParams() *StreamGetParams {
	return &StreamGetParams{}
}

func (p *StreamGetParams) clone() *StreamGetParams {
	clone := *p
	return &clone
}

// Index returns a stream Get parameter that causes the Get request to
// retrieve logs starting at the requested stream index instead of the default,
// zero.
func (p *StreamGetParams) Index(i types.MessageIndex) *StreamGetParams {
	p = p.clone()
	p.r.Index = int64(i)
	return p
}

// Limit limits the returned logs either by count or by byte count. If either
// limit is <= 0, then no limit will be applied and the server will choose how
// many logs to return.
func (p *StreamGetParams) Limit(bytes, count int) *StreamGetParams {
	p = p.clone()

	if bytes < 0 {
		bytes = 0
	}
	if count < 0 {
		count = 0
	}

	p.r.ByteCount, p.r.LogCount = int32(bytes), int32(count)
	return p
}

// NonContiguous returns a stream Get parameter that causes the Get request
// to allow non-contiguous records to be returned. By default, only contiguous
// records starting from the specific Index will be returned.
//
// By default, a log stream will return only contiguous records starting at the
// requested index. For example, if a stream had: {0, 1, 2, 4, 5} and a request
// was made for index 0, Get will return {0, 1, 2}, for index 3 {}, and for
// index 4 {4, 5}.
//
// If NonContiguous is true, a request for 0 will return {0, 1, 2, 4, 5} and so
// on.
//
// Log entries generally should not be missing, but may be if either the logs
// are still streaming (since they can be ingested out of order) or if a data
// loss or corruption occurs.
func (p *StreamGetParams) NonContiguous() *StreamGetParams {
	p = p.clone()
	p.r.NonContiguous = true
	return p
}

// State returns a stream Get parameter that causes the Get request to return
// its stream state and log stream descriptor.
func (p *StreamGetParams) State(stateP *LogStream) *StreamGetParams {
	p = p.clone()
	p.stateP = stateP
	return p
}
