// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package clockflag

import (
	"encoding/json"
	"flag"
	"time"
)

// Time is a flag- and JSON-compatible Time which parses from RFC3339 strings.
type Time time.Time

var _ interface {
	flag.Value
	json.Unmarshaler
	json.Marshaler
} = (*Time)(nil)

// Time returns the Time value associated with this Time.
func (t Time) Time() time.Time {
	return time.Time(t)
}

// Set implements flag.Value.
func (t *Time) Set(value string) error {
	timeValue, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return err
	}
	*t = Time(timeValue.UTC())
	return nil
}

func (t *Time) String() string {
	return time.Time(*t).String()
}

// UnmarshalJSON implements json.Unmarshaler.
//
// Unmarshals a JSON entry into the underlying type. The entry is expected to
// contain a string corresponding to one of the enum's keys.
func (t *Time) UnmarshalJSON(data []byte) error {
	var value time.Time
	if err := value.UnmarshalJSON(data); err != nil {
		return err
	}
	*t = Time(value.UTC())
	return nil
}

// MarshalJSON implements json.Marshaler.
//
// Marshals a Time into an RFC3339 time string.
func (t Time) MarshalJSON() ([]byte, error) {
	return t.Time().UTC().MarshalJSON()
}
