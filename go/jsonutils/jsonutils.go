package jsonutils

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"
)

// Number is an int64 which may be unmarshaled from a JSON string.
type Number int64

// UnmarshalJSON parses data as an integer, whether data is a number or string.
func (n *Number) UnmarshalJSON(data []byte) error {
	data = bytes.Trim(data, `"`)
	num, err := strconv.ParseInt(string(data), 0, 64)
	if err == nil {
		*n = Number(num)
	}
	return err
}

// Time is a convenience type used for unmarshaling a time.Time from a JSON-
// encoded timestamp in microseconds.
type Time time.Time

// MarshalJSON encodes a time.Time as a JSON number of microseconds.
func (t *Time) MarshalJSON() ([]byte, error) {
	ts := (*time.Time)(t).UnixNano() / int64(time.Microsecond)
	return json.Marshal(ts)
}

// UnmarshalJSON parses a time.Time from a JSON number of microseconds.
func (t *Time) UnmarshalJSON(data []byte) error {
	var timeN Number
	if err := timeN.UnmarshalJSON(data); err != nil {
		return err
	}
	*t = Time(time.Unix(0, int64(timeN)*int64(time.Microsecond)).UTC())
	return nil
}
