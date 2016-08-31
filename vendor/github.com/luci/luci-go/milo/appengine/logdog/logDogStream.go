// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logdog

import (
	miloProto "github.com/luci/luci-go/common/proto/milo"
)

// Streams represents a group of LogDog Streams with a single entry point.
// Generally all of the streams are referenced by the entry point.
type Streams struct {
	// MainStream is a pointer to the primary stream for this group of streams.
	MainStream *Stream
	// Streams is the full map streamName->stream referenced by MainStream.
	// It includes MainStream.
	Streams map[string]*Stream
}

// Stream represents a single LogDog style stream, which can contain either
// annotations (assumed to be MiloProtos) or text.  Other types of annotations are
// not supported.
type Stream struct {
	// Server is the LogDog server this stream originated from.
	Server string
	// Prefix is the LogDog prefix for the Stream.
	Prefix string
	// Path is the final part of the LogDog path of the Stream.
	Path string
	// IsDatagram is true if this is a MiloProto. False implies that this is a text log.
	IsDatagram bool
	// Data is the miloProto.Step of the Stream, if IsDatagram is true.  Otherwise
	// this is nil.
	Data *miloProto.Step
	// Text is the text of the Stream, if IsDatagram is false.  Otherwise
	// this is an empty string.
	Text string

	// Closed specifies whether Text or Data may change in the future.
	// If Closed, they may not.
	Closed bool
}
