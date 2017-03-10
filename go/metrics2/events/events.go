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
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"

	"golang.org/x/net/context"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db/local_db"

	"github.com/boltdb/bolt"
)

func encodeKey(ts time.Time) ([]byte, error) {
	if ts.UnixNano() < 0 {
		return nil, fmt.Errorf("Time is invalid: %s", ts)
	}
	return []byte(ts.UTC().Format(local_db.TIMESTAMP_FORMAT)), nil
}

func decodeKey(b []byte) (time.Time, error) {
	return time.Parse(local_db.TIMESTAMP_FORMAT, string(b))
}

func encodeValue(v float64) []byte {
	rv := make([]byte, 8)
	binary.BigEndian.PutUint64(rv, math.Float64bits(v))
	return rv
}

func decodeValue(b []byte) float64 {
	return math.Float64frombits(binary.BigEndian.Uint64(b))
}

// EventMetrics is a struct used for aggregating event-based metrics.
type EventMetrics struct {
	db      *bolt.DB
	metrics []*metric
	mtx     sync.Mutex
	streams map[string]*eventStream
}

// NewEventMetrics returns an EventMetrics instance.
func NewEventMetrics(filename string) (*EventMetrics, error) {
	db, err := bolt.Open(filename, 0600, nil)
	if err != nil {
		return nil, err
	}

	rv := &EventMetrics{
		db:      db,
		metrics: []*metric{},
		streams: map[string]*eventStream{},
	}

	// Load all streams from the DB.
	if err := db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			rv.streams[string(name)] = &eventStream{
				bucket: name,
				m:      rv,
			}
			return nil
		})
	}); err != nil {
		return nil, err
	}

	return rv, nil
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

// metric is a set of information used to derive metrics.
type metric struct {
	name   string
	tags   map[string]string
	events *eventStream
	period time.Duration
	agg    func([]time.Time, []float64) (float64, error)
}

// calc computes the current aggregated value of the metric.
func (m *metric) calc(now time.Time) (float64, error) {
	ts, vs, err := m.events.GetRange(now.Add(-m.period), now)
	if err != nil {
		return 0.0, err
	}
	return m.agg(ts, vs)
}

// addMetric adds a metric.
func (m *EventMetrics) addMetric(mx *metric) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.metrics = append(m.metrics, mx)
}

// updateMetrics recalculates values for all metrics.
func (m *EventMetrics) updateMetrics(now time.Time) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	errs := []error{}
	for _, m := range m.metrics {
		v, err := m.calc(now)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		metrics2.GetFloat64Metric(m.name, m.tags).Update(v)
	}
	if len(errs) > 0 {
		return fmt.Errorf("updateMetrics errors: %v", errs)
	}
	return nil
}

// GetEventStream returns an EventStream instance.
func (m *EventMetrics) GetEventStream(name string) *eventStream {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	rv, ok := m.streams[name]
	if !ok {
		rv = &eventStream{
			bucket: []byte(name),
			m:      m,
		}
		m.streams[name] = rv
	}
	return rv
}

// eventStream is a struct which deals with a single stream of related events.
type eventStream struct {
	bucket []byte
	m      *EventMetrics
}

// InsertAt adds a data point to the stream at the given timestamp. Overrides
// any data point at the timestamp.
func (s *eventStream) InsertAt(ts time.Time, v float64) error {
	k, err := encodeKey(ts)
	if err != nil {
		return err
	}
	return s.m.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(s.bucket)
		if err != nil {
			return err
		}
		return b.Put(k, encodeValue(v))
	})
}

// Insert adds a data point to the stream at the current time.
func (s *eventStream) Insert(v float64) error {
	return s.InsertAt(time.Now(), v)
}

// GetRange returns all data points in the given range.
func (s *eventStream) GetRange(start, end time.Time) ([]time.Time, []float64, error) {
	min, err := encodeKey(start)
	if err != nil {
		return nil, nil, err
	}
	max, err := encodeKey(end)
	if err != nil {
		return nil, nil, err
	}

	ts := []time.Time{}
	vs := []float64{}
	if err := s.m.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(s.bucket).Cursor()
		for k, v := c.Seek(min); k != nil && bytes.Compare(k, max) <= 0; k, v = c.Next() {
			t, err := decodeKey(k)
			if err != nil {
				return err
			}
			ts = append(ts, t)
			vs = append(vs, decodeValue(v))
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	return ts, vs, nil
}

// AggregateMetric sets the given aggregation function on the event stream and adds
// a gauge for it.
func (s *eventStream) AggregateMetric(measurement string, tags map[string]string, period time.Duration, agg func([]time.Time, []float64) (float64, error)) {
	s.m.addMetric(&metric{
		name:   measurement,
		tags:   tags,
		events: s,
		period: period,
		agg:    agg,
	})
}

// MeanMetric adds a gauge which takes a moving average over the given period.
func (s *eventStream) MeanMetric(measurement string, tags map[string]string, period time.Duration) {
	// TODO(borenet): Use a more stable algorithm.
	s.AggregateMetric(measurement, tags, period, func(ts []time.Time, vs []float64) (float64, error) {
		return util.Float64StableSum(vs) / float64(len(vs)), nil
	})
}
