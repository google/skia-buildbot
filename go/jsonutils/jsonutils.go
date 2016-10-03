package jsonutils

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"

	"go.skia.org/infra/go/util"
)

// Number is an int64 which may be unmarshaled from a JSON string.
type Number int64

// UnmarshalJSON parses data as an integer, whether data is a number or string.
func (n *Number) UnmarshalJSON(data []byte) error {
	data = bytes.Trim(data, `"`)
	num, err := strconv.Atoi(string(data))
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
	ts := (*time.Time)(t).UnixNano() / util.MICROS_TO_NANOS
	return json.Marshal(ts)
}

// UnmarshalJSON parses a time.Time from a JSON number of microseconds.
func (t *Time) UnmarshalJSON(data []byte) error {
	num, err := strconv.ParseInt(string(data), 10, 64)
	if err == nil {
		*t = Time(time.Unix(0, num*util.MICROS_TO_NANOS).UTC())
	}
	return err
}
