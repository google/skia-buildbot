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
	"context"
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	MEASUREMENT_NAME = "event-metrics"

	timestampFormat = "20060102T150405.000000000Z"
)

func encodeKey(ts time.Time) ([]byte, error) {
	if ts.UnixNano() < 0 {
		return nil, fmt.Errorf("Time is invalid: %s", ts)
	}
	return []byte(ts.UTC().Format(timestampFormat)), nil
}

func decodeKey(b []byte) (time.Time, error) {
	return time.Parse(timestampFormat, string(b))
}

// AggregateFn is a function which reduces a number of Events into a single
// data point.
type AggregateFn func([]*Event) (float64, error)

// Event contains some metadata plus whatever data the user chooses.
type Event struct {
	Stream    string
	Timestamp time.Time
	Data      []byte
}

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
	db      EventDB
	metrics map[string]map[time.Duration][]*metric
	mtx     sync.Mutex
}

// NewEventMetrics returns an EventMetrics instance.
func NewEventMetrics(db EventDB) (*EventMetrics, error) {
	return &EventMetrics{
		db:      db,
		metrics: map[string]map[time.Duration][]*metric{},
		mtx:     sync.Mutex{},
	}, nil
}

// Start initiates the EventMetrics goroutines.
func (m *EventMetrics) Start(ctx context.Context) {
	lv := metrics2.NewLiveness("last-successful-event-metrics-update")
	go util.RepeatCtx(time.Minute, ctx, func() {
		if err := m.updateMetrics(time.Now()); err != nil {
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
//      s.AggregateMetric("my-stream", myTags, 24*time.Hour, func(ev []Event) (float64, error) {
//              sum := int64(0)
//              for _, e := range ev {
//                      sum += decodeInt64(e)
//              }
//              return float64(sum), nil
//      })
//
func (m *EventMetrics) AggregateMetric(stream string, tags map[string]string, period time.Duration, agg AggregateFn) error {
	mx := &metric{
		measurement: MEASUREMENT_NAME,
		tags: map[string]string{
			"period": fmt.Sprintf("%s", period),
			"stream": stream,
		},
		stream: stream,
		period: period,
		agg:    agg,
	}
	for k, v := range tags {
		if _, ok := mx.tags[k]; ok {
			return fmt.Errorf("Tag %q is reserved.", k)
		}
		mx.tags[k] = v
	}
	m.mtx.Lock()
	defer m.mtx.Unlock()
	byPeriod, ok := m.metrics[stream]
	if !ok {
		byPeriod = map[time.Duration][]*metric{}
		m.metrics[stream] = byPeriod
	}
	byPeriod[period] = append(byPeriod[period], mx)
	return nil
}

// updateMetric updates the value for a single metric.
func (m *EventMetrics) updateMetric(ev []*Event, mx *metric) error {
	v, err := mx.agg(ev)
	if err != nil {
		return err
	}
	metrics2.GetFloat64Metric(mx.measurement, mx.tags).Update(v)
	return nil
}

// updateMetrics recalculates values for all metrics using the given value for
// the current time.
func (m *EventMetrics) updateMetrics(now time.Time) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	errs := []error{}
	for stream, byPeriod := range m.metrics {
		for period, metrics := range byPeriod {
			ev, err := m.db.Range(stream, now.Add(-period), now)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			for _, mx := range metrics {
				if err := m.updateMetric(ev, mx); err != nil {
					errs = append(errs, err)
					continue
				}
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("UpdateMetrics errors: %v", errs)
	}
	return nil
}

// UpdateMetrics recalculates values for all metrics.
func (m *EventMetrics) UpdateMetrics() error {
	return m.updateMetrics(time.Now())
}

// LogMetrics logs the current values for all metrics.
func (m *EventMetrics) LogMetrics() {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	sklog.Infof("Current event metrics values:")
	for _, byPeriod := range m.metrics {
		for _, metrics := range byPeriod {
			for _, mx := range metrics {
				v := metrics2.GetFloat64Metric(mx.measurement, mx.tags).Get()
				sklog.Infof("  %s %v: %f", mx.measurement, mx.tags, v)
			}
		}
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
