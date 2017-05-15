package events

import "time"

// EventStream is a struct which deals with a single stream of related events.
type EventStream struct {
	m    *EventMetrics
	name string
}

// Insert inserts the Event into the stream. Overrides any Event at the
// given timestamp.
func (s *EventStream) Insert(e *Event) error {
	e.Stream = s.name
	return s.m.db.Insert(e)
}

// Append adds the given data to the stream at the current time.
func (s *EventStream) Append(data []byte) error {
	return s.m.db.Append(s.name, data)
}

// Range returns all Events in the given range.
func (s *EventStream) Range(start, end time.Time) ([]*Event, error) {
	return s.m.db.Range(s.name, start, end)
}

// AggregateMetric sets the given aggregation function on the event stream and
// adds a gauge for it. For example, to compute the sum of all int64 events over
// a 24-hour period:
//
//      s.AggregateMetric(myTags, 24*time.Hour, func(ev []*Event) (float64, error) {
//              sum := int64(0)
//              for _, e := range ev {
//                      sum += decodeInt64(e)
//              }
//              return float64(sum), nil
//      })
//
func (s *EventStream) AggregateMetric(tags map[string]string, period time.Duration, agg AggregateFn) error {
	return s.m.AggregateMetric(s.name, tags, period, agg)
}

// DynamicMetric sets the given aggregation function on the event stream. Gauges
// will be added and removed dynamically based on the results of the aggregation
// function. Here's a toy example:
//
//      s.DynamicMetric(myTags, 24*time.Hour, func(ev []Event) (map[string]float64, error) {
//              counts := map[int64]int64{}
//              for _, e := range ev {
//                      counts[decodeInt64(e)]++
//              }
//              rv := make(map[string]float64, len(counts))
//              for k, v := range counts {
//                      rv[fmt.Sprintf("%d", k)] = float64(v)
//              }
//              return rv
//      })
//
func (s *EventStream) DynamicMetric(tags map[string]string, period time.Duration, agg DynamicAggregateFn) error {
	return s.m.DynamicMetric(s.name, tags, period, agg)
}
