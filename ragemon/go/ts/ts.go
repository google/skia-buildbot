// Package ts provides TimeSeries, for storing time series data in a compressed format that's fast to read.
package ts

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"sync"

	"github.com/skia-dev/glog"
)

// Point represents a single sample point.
type Point struct {
	Timestamp int64 // Seconds since the Unix epoch.
	Value     int64
}

// TimeSeries is a series of Points.
//
// TimeSeries must be created via New or NewFromData, as the zero value is invalid.
type TimeSeries struct {
	data []Point
	lock sync.Mutex
}

// TimeSeriesSet is a map from a structured key to TimeSeries.
type TimeSeriesSet map[string]*TimeSeries

// encode a singe Point to a byte slice.
func encode(p Point) []byte {
	buf := make([]byte, 20)
	n := binary.PutVarint(buf, p.Timestamp)
	m := binary.PutVarint(buf[n:], p.Value)
	return buf[:m+n]
}

// decode a single Point from byte slice, returns the Point, the number of
// bytes actually read from 'b', and a non-nil error if an error occurred.
func decode(b []byte) (Point, int, error) {
	ret := Point{}
	ts, n := binary.Varint(b)
	if n <= 0 {
		return ret, 0, fmt.Errorf("Failed to decode timestamp, Varint returned: %d", n)
	}
	ret.Timestamp = ts
	value, m := binary.Varint(b[n:])
	if m <= 0 {
		return ret, 0, fmt.Errorf("Failed to decode value, Varint returned: %d", m)
	}
	ret.Value = value
	return ret, n + m, nil
}

// New creates a new TimeSeries with the given first sample point.
func New(pt Point) *TimeSeries {
	ret := &TimeSeries{
		data: []Point{pt},
	}
	return ret
}

// NewFromData returns a TimeSeries reconstituted from the given byte slice,
// ready to add new data.
func NewFromData(b []byte) (*TimeSeries, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("An empty byte slice is not a valid serialized TimeSeries.")
	}
	// The first point is stored directly.
	prevPoint, n, err := decode(b)
	if err != nil {
		return nil, err
	}
	b = b[n:]
	t := &TimeSeries{
		data: []Point{prevPoint},
	}
	// Process the rest of the samples.
	for len(b) != 0 {
		pair, m, err := decode(b)
		if err != nil {
			return nil, err
		}
		// All points besides the first are stored as deltas from the previous value.
		pair.Value += prevPoint.Value
		pair.Timestamp += prevPoint.Timestamp
		t.data = append(t.data, pair)
		prevPoint = pair
		b = b[m:]
	}
	return t, nil
}

// Points returns the Points that the timeseries contains.
func (t *TimeSeries) Points() []Point {
	return t.data
}

// Returns true if p.Timestamp is in [begin, end).
func inRange(begin, end int64, p Point) bool {
	return begin <= p.Timestamp && end > p.Timestamp
}

// PointsInRange returns all the points that have a timestamp that fall in the
// range [begin, end).
func (t *TimeSeries) PointsInRange(begin, end int64) []Point {
	t.lock.Lock()
	defer t.lock.Unlock()

	ret := []Point{}
	first := sort.Search(len(t.data), func(i int) bool {
		return begin <= t.data[i].Timestamp
	})

	if first == len(t.data) {
		return ret
	}

	for _, p := range t.data[first:] {
		if p.Timestamp < end {
			ret = append(ret, p)
		} else {
			break
		}
	}

	return ret
}

// PointsInRange returns all the points that have a timestamp that fall in the
// range [begin, end) when 'b' is deserialized as a TimeSeries. A non-nil error
// is returned if 'b' is not a valid serialize TimeSeries.
func PointsInRange(begin, end int64, b []byte) ([]Point, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("An empty byte slice is not a valid serialized TimeSeries.")
	}

	// The first point is stored directly.
	prevPoint, n, err := decode(b)
	if err != nil {
		return nil, err
	}
	b = b[n:]
	ret := []Point{}
	if inRange(begin, end, prevPoint) {
		ret = append(ret, prevPoint)
	}
	// Process the rest of the samples.
	for len(b) != 0 {
		pair, m, err := decode(b)
		if err != nil {
			return nil, err
		}
		// All points besides the first are stored as deltas from the previous value.
		pair.Value += prevPoint.Value
		pair.Timestamp += prevPoint.Timestamp
		if inRange(begin, end, pair) {
			ret = append(ret, pair)
		} else if pair.Timestamp > end {
			break
		}
		prevPoint = pair
		b = b[m:]
	}
	return ret, nil
}

// Add a new value to the timeseries.
func (t *TimeSeries) Add(pt Point) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if pt.Timestamp <= t.data[len(t.data)-1].Timestamp {
		glog.Errorf("Received an out of order point %v", pt)
		return
	}
	t.data = append(t.data, pt)
}

// Bytes encodes the TimeSeries series of timestamps and int64 values quickly
// and efficiently.
//
// The values stored in data are encoded via binary.PutVarint, which uses
// zigzag encoding. The format of data is a series of varints, where the first
// timestamp and value are stored directly as varint. The remaining timestamps
// and values are stored as deltas from the previous value.
//
//   [first timestamp][first value]
//     [delta encoded second timestamp][delta encoded second value]
//     [delta encoded third timestamp][delta encoded third value]
//
func (t *TimeSeries) Bytes() ([]byte, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	buf := &bytes.Buffer{}
	prevPoint := t.data[0]
	// The first point is written directly.
	if _, err := buf.Write(encode(prevPoint)); err != nil {
		return nil, fmt.Errorf("Failed to encode Point %#v: %s", prevPoint, err)
	}
	currentPoint := Point{}
	for _, p := range t.data[1:] {
		// All points besides the first as written as deltas to their previous
		// value.
		currentPoint.Timestamp = p.Timestamp - prevPoint.Timestamp
		currentPoint.Value = p.Value - prevPoint.Value
		if _, err := buf.Write(encode(currentPoint)); err != nil {
			return nil, fmt.Errorf("Failed to encode Point %#v: %s", currentPoint, err)
		}
		prevPoint = p
	}
	return buf.Bytes(), nil
}
