// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamproto

import (
	"encoding/json"
	"flag"

	"github.com/luci/luci-go/logdog/common/types"
)

// StreamNameFlag is a flag and JSON-compatible type that converts to/from a
// types.StreamName. StreamName validation is part of the conversion.
type StreamNameFlag types.StreamName

var _ interface {
	flag.Value
	json.Marshaler
	json.Unmarshaler
} = (*StreamNameFlag)(nil)

// Set implements flag.Value.
func (f *StreamNameFlag) Set(v string) error {
	if err := types.StreamName(v).Validate(); err != nil {
		return err
	}
	*f = StreamNameFlag(v)
	return nil
}

// String implements flag.Value.
func (f *StreamNameFlag) String() string {
	if f == nil {
		return ""
	}
	return string(*f)
}

// UnmarshalJSON implements json.Unmarshaler.
func (f *StreamNameFlag) UnmarshalJSON(data []byte) error {
	s := ""
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if err := types.StreamName(s).Validate(); err != nil {
		return err
	}
	*f = StreamNameFlag(s)
	return nil
}

// MarshalJSON implements json.Marshaler.
func (f StreamNameFlag) MarshalJSON() ([]byte, error) {
	s := string(f)
	return json.Marshal(&s)
}
