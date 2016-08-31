// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package annotee

import (
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/clockflag"
	"github.com/luci/luci-go/common/proto/milo"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"
	"github.com/luci/luci-go/logdog/common/types"

	"golang.org/x/net/context"
)

var (
	textStreamArchetype = streamproto.Flags{
		ContentType: string(types.ContentTypeText),
		Type:        streamproto.StreamType(logpb.StreamType_TEXT),
	}

	metadataStreamArchetype = streamproto.Flags{
		ContentType: string(milo.ContentTypeAnnotations),
		Type:        streamproto.StreamType(logpb.StreamType_DATAGRAM),
	}
)

// TextStreamFlags returns the streamproto.Flags for a text stream using
// Annotee's text stream archetype.
func TextStreamFlags(ctx context.Context, name types.StreamName) streamproto.Flags {
	return streamFlagsFromArchetype(ctx, name, &textStreamArchetype)
}

func streamFlagsFromArchetype(ctx context.Context, name types.StreamName, archetype *streamproto.Flags) streamproto.Flags {
	// Clone the properties archetype and customize.
	f := *archetype
	f.Timestamp = clockflag.Time(clock.Now(ctx))
	f.Name = streamproto.StreamNameFlag(name)
	return f
}
