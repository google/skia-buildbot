package jsonutils

import (
	"bytes"
	"encoding/json"
	"sort"
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

// MarshalStringMap turns the given string map into a []byte slice that is the JSON encoded version
// of that map. It will produce the same output as json.Marshal. This includes the keys being sorted
// lexicographically. Unlike json.Marshal, it does not return an error because the errors
// json.Marshal could return (e.g. for cyclic data) do not apply.
func MarshalStringMap(m map[string]string) (data []byte) {
	if len(m) == 0 {
		// This behavior matches json.Marshal
		if m == nil {
			return []byte("null")
		}
		return []byte("{}")
	}
	keyValues := make([]string, 0, len(m))
	byteCount := 0
	var buf bytes.Buffer
	for k, v := range m {
		buf.WriteRune('"')
		buf.WriteString(k)
		buf.WriteString(`":"`)
		buf.WriteString(v)
		buf.WriteRune('"')
		keyValues = append(keyValues, buf.String())
		byteCount += buf.Len()
		buf.Reset()
	}
	// sort for determinism and to match the default impl
	// We go with insertion sort unless there are a lot of values.
	if len(keyValues) <= 30 {
		for i := 0; i < len(keyValues); i++ {
			for j := i; j > 0 && keyValues[j] < keyValues[j-1]; j-- {
				keyValues[j], keyValues[j-1] = keyValues[j-1], keyValues[j]
			}
		}
	} else {
		sort.Strings(keyValues)
	}

	var result bytes.Buffer
	// Need to account for an open and closed curly brace, and n-1 commas
	result.Grow(2 + byteCount + len(keyValues) - 1)
	result.WriteRune('{')
	for i := range keyValues {
		if i != 0 {
			result.WriteRune(',')
		}
		result.WriteString(keyValues[i])
	}
	result.WriteRune('}')
	return result.Bytes()
}
