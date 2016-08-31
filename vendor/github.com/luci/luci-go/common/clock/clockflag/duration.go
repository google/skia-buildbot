// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package clockflag

import (
	"encoding/json"
	"flag"
	"fmt"
	"time"
)

// Duration is a Flag- and JSON-compatible Duration value.
type Duration time.Duration

var _ flag.Value = (*Duration)(nil)

// Set implements flag.Value.
func (d *Duration) Set(value string) (err error) {
	*d, err = ParseDuration(value)
	return
}

func (d *Duration) String() string {
	return FormatDuration(time.Duration(*d))
}

// IsZero tests if this Duration is the zero value.
func (d Duration) IsZero() bool {
	return time.Duration(d) == time.Duration(0)
}

// UnmarshalJSON implements json.Unmarshaler.
//
// Unmarshals a JSON entry into the underlying type. The entry is expected to
// contain a string corresponding to one of the enum's keys.
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	return d.Set(s)
}

// MarshalJSON implements json.Marshaler.
//
// Marshals a Duration into a duration string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, time.Duration(d).String())), nil
}

// ParseDuration parses a clockflag Duration from a string. This is basically
// a typed fall-through to time.ParseDuration.
func ParseDuration(v string) (Duration, error) {
	duration, err := time.ParseDuration(v)
	if err != nil {
		return 0, err
	}
	return Duration(duration), nil
}

// FormatDuration formats a time.Duration into a string that can be parsed with
// ParseDuration.
func FormatDuration(d time.Duration) string {
	return d.String()
}
