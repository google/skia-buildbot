// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamproto

import (
	"encoding/json"
	"flag"

	"github.com/luci/luci-go/common/flag/flagenum"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/common/types"
)

// StreamType is a flag- and JSON-compatible wrapper around the StreamType
// protobuf field.
type StreamType logpb.StreamType

// DefaultContentType returns the default ContentType for a given stream type.
func (t StreamType) DefaultContentType() types.ContentType {
	switch logpb.StreamType(t) {
	case logpb.StreamType_TEXT:
		return types.ContentTypeText

	case logpb.StreamType_DATAGRAM:
		return types.ContentTypeLogdogDatagram

	case logpb.StreamType_BINARY:
		fallthrough
	default:
		return types.ContentTypeBinary
	}
}

var _ interface {
	json.Marshaler
	json.Unmarshaler
	flag.Value
} = (*StreamType)(nil)

var (
	// StreamTypeFlagEnum maps configuration strings to their underlying StreamTypes.
	StreamTypeFlagEnum = flagenum.Enum{
		"text":     StreamType(logpb.StreamType_TEXT),
		"binary":   StreamType(logpb.StreamType_BINARY),
		"datagram": StreamType(logpb.StreamType_DATAGRAM),
	}
)

// Set implements flag.Value.
func (t *StreamType) Set(v string) error {
	return StreamTypeFlagEnum.FlagSet(t, v)
}

// String implements flag.Value.
func (t *StreamType) String() string {
	return StreamTypeFlagEnum.FlagString(t)
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *StreamType) UnmarshalJSON(data []byte) error {
	return StreamTypeFlagEnum.JSONUnmarshal(t, data)
}

// MarshalJSON implements json.Marshaler.
func (t StreamType) MarshalJSON() ([]byte, error) {
	return StreamTypeFlagEnum.JSONMarshal(t)
}
