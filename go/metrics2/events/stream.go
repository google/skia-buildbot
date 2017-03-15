package events

import "time"

// EventStream is a struct which deals with a single stream of related events.
type EventStream struct {
	m    *EventMetrics
	name string
}

// InsertAt adds a data point to the stream at the given timestamp. Overrides
// any data point at the timestamp.
func (s *EventStream) InsertAt(ts time.Time, e Event) error {
	return s.m.db.InsertAt(s.name, ts, e)
}

// Insert adds a data point to the stream at the current time.
func (s *EventStream) Insert(e Event) error {
	return s.m.db.Insert(s.name, e)
}

// GetRange returns all Events in the given range.
func (s *EventStream) GetRange(start, end time.Time) ([]time.Time, []Event, error) {
	return s.m.db.GetRange(s.name, start, end)
}

// AggregateMetric sets the given aggregation function on the event stream and
// adds a gauge for it. For example, to compute the sum of all int64 events over
// a 24-hour period:
//
//      s.AggregateMetric("sum", myTags, 24*time.Hour, func(ts []time.Time, ev []Event) (float64, error) {
//              sum := int64(0)
//              for _, e := range ev {
//                      sum += decodeInt64(e)
//              }
//              return float64(sum), nil
//      })
//
func (s *EventStream) AggregateMetric(measurement string, tags map[string]string, period time.Duration, agg AggregateFn) {
	s.m.AggregateMetric(s.name, measurement, tags, period, agg)
}
