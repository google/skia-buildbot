package events

/*
	This package provides facilities for event-based metrics.

	The metrics2 package deals with gauges; it doesn't have natural support for
	events, ie. individual samples with timestamps. This package provides that
	support by allowing the user to insert individual data points and provides
	functions for aggregating data points into a gauge format which can then be
	used as a normal metric.
*/

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/net/context"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	TIMESTAMP_FORMAT = "20060102T150405.000000000Z"
)

func encodeKey(ts time.Time) ([]byte, error) {
	if ts.UnixNano() < 0 {
		return nil, fmt.Errorf("Time is invalid: %s", ts)
	}
	return []byte(ts.UTC().Format(TIMESTAMP_FORMAT)), nil
}

func decodeKey(b []byte) (time.Time, error) {
	return time.Parse(TIMESTAMP_FORMAT, string(b))
}

// AggregateFn is a function which reduces a number of Events into a single
// data point. The slice of time.Times are the timestamps which correspond to
// the slice of events.
type AggregateFn func([]time.Time, []Event) (float64, error)

// Event contains whatever data the user chooses.
type Event []byte

// metric is a set of information used to derive metrics.
type metric struct {
	measurement string
	tags        map[string]string
	stream      string
	period      time.Duration
	agg         AggregateFn
}

// EventMetrics is a struct used for creating aggregate metrics on steams of
// events.
type EventMetrics struct {
	db      *eventDB
	metrics []*metric
	mtx     sync.Mutex
}

// NewEventMetrics returns an EventMetrics instance.
func NewEventMetrics(filename string) (*EventMetrics, error) {
	db, err := newEventDB(filename)
	if err != nil {
		return nil, err
	}
	return &EventMetrics{
		db:      db,
		metrics: []*metric{},
		mtx:     sync.Mutex{},
	}, nil
}

// Start initiates the EventMetrics goroutines.
func (m *EventMetrics) Start(ctx context.Context) {
	lv := metrics2.NewLiveness("last-successful-event-metrics-update")
	go util.RepeatCtx(time.Minute, ctx, func() {
		if err := m.UpdateMetrics(time.Now()); err != nil {
			sklog.Errorf("Failed to update event metrics: %s", err)
		} else {
			lv.Reset()
		}
	})
}

// Close cleans up the EventMetrics.
func (m *EventMetrics) Close() error {
	return m.db.Close()
}

// AggregateMetric sets the given aggregation function on the event stream and
// adds a gauge for it. For example, to compute the sum of all int64 events over
// a 24-hour period:
//
//      s.AggregateMetric("my-stream", "sum", myTags, 24*time.Hour, func(ts []time.Time, ev []Event) (float64, error) {
//              sum := int64(0)
//              for _, e := range ev {
//                      sum += decodeInt64(e)
//              }
//              return float64(sum), nil
//      })
//
func (m *EventMetrics) AggregateMetric(stream, measurement string, tags map[string]string, period time.Duration, agg AggregateFn) {
	mx := &metric{
		measurement: measurement,
		tags:        tags,
		stream:      stream,
		period:      period,
		agg:         agg,
	}
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.metrics = append(m.metrics, mx)
}

// updateMetric updates the value for a single metric.
func (m *EventMetrics) updateMetric(now time.Time, mx *metric) error {
	ts, es, err := m.db.GetRange(mx.stream, now.Add(-mx.period), now)
	if err != nil {
		return err
	}
	v, err := mx.agg(ts, es)
	if err != nil {
		return err
	}
	metrics2.GetFloat64Metric(mx.measurement, mx.tags).Update(v)
	return nil
}

// UpdateMetrics recalculates values for all metrics.
func (m *EventMetrics) UpdateMetrics(now time.Time) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	errs := []error{}
	for _, mx := range m.metrics {
		if err := m.updateMetric(now, mx); err != nil {
			errs = append(errs, err)
			continue
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("UpdateMetrics errors: %v", errs)
	}
	return nil
}

// LogMetrics logs the current values for all metrics.
func (m *EventMetrics) LogMetrics() {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	sklog.Infof("Current metrics values:")
	for _, mx := range m.metrics {
		v := metrics2.GetFloat64Metric(mx.measurement, mx.tags).Get()
		sklog.Infof("  %s %v: %f", mx.measurement, mx.tags, v)
	}
}

// GetEventStream returns an EventStream instance.
func (m *EventMetrics) GetEventStream(name string) *EventStream {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	return &EventStream{
		name: name,
		m:    m,
	}
}
