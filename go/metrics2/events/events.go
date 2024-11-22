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
	"math"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// Tag key indicating what aggregation function was used to derive the
	// metric.
	tagAggregation = "aggregation"

	// Tag key indicating the time period over which a metric is calculated.
	tagPeriod = "period"

	// Tag key indicating which event stream a metric is calculated from.
	tagStream = "stream"

	// The timestamp format used for encoding/decoding keys for the BoltDB
	// database of events.
	timestampFormat = "20060102T150405.000000000Z"
)

var (
	// Callers may not use these tag keys.
	RESERVED_TAGS = []string{tagPeriod, tagStream}
)

// encodeKey encodes a key for an entry in the BoltDB database of events.
func encodeKey(ts time.Time) ([]byte, error) {
	if ts.UnixNano() < 0 {
		return nil, fmt.Errorf("Time is invalid: %s", ts)
	}
	return []byte(ts.UTC().Format(timestampFormat)), nil
}

// decodeKey decodes a key for an entry in the BoltDB database of events.
func decodeKey(b []byte) (time.Time, error) {
	return time.Parse(timestampFormat, string(b))
}

// AggregateFn is a function which reduces a number of Events into a single
// data point.
type AggregateFn func([]*Event) (float64, error)

// DynamicAggregateFn is a function which reduces a number of Events into a
// several data points, each with its own set of tags. The aggregation function
// may return multiple data points, each with its own set of tags. Each tag set
// comprises its own metric, and the aggregation function may return different
// tag sets at each iteration, eg. due to events with different properties
// occurring within each time period.
type DynamicAggregateFn func([]*Event) ([]map[string]string, []float64, error)

// ObservationsFn is a function which derives a number of float64 observations
// from a set of Events. The number of observations does not need to equal the
// number of Events.
type ObservationsFn func([]*Event) ([]float64, error)

// Event contains some metadata plus whatever data the user chooses.
type Event struct {
	Stream    string
	Timestamp time.Time
	Data      []byte
}

// metric is a set of information used to derive a metric.
type metric struct {
	measurement string
	tags        map[string]string
	stream      string
	period      time.Duration
	agg         AggregateFn
}

// dynamicMetric is a set of information used to derive a set of changing
// metrics. The aggregation function may return multiple data points, each with
// its own set of tags. Each tag set comprises its own metric, and the
// aggregation function may return different tag sets at each iteration, eg. due
// to events with different properties occurring within each time period.
type dynamicMetric struct {
	measurement string
	tags        map[string]string
	stream      string
	period      time.Duration
	agg         DynamicAggregateFn
}

// EventMetrics is a struct used for creating aggregate metrics on streams of
// events.
type EventMetrics struct {
	db                    EventDB
	measurement           string
	dynamicMetrics        map[string]map[time.Duration][]*dynamicMetric
	currentDynamicMetrics map[string]metrics2.Float64Metric
	metrics               map[string]map[time.Duration][]*metric
	mtx                   sync.Mutex
}

// NewEventMetrics returns an EventMetrics instance.
func NewEventMetrics(db EventDB, measurement string) (*EventMetrics, error) {
	return &EventMetrics{
		db:                    db,
		measurement:           measurement,
		dynamicMetrics:        map[string]map[time.Duration][]*dynamicMetric{},
		currentDynamicMetrics: map[string]metrics2.Float64Metric{},
		metrics:               map[string]map[time.Duration][]*metric{},
		mtx:                   sync.Mutex{},
	}, nil
}

// Start initiates the EventMetrics goroutines.
func (m *EventMetrics) Start(ctx context.Context) {
	lv := metrics2.NewLiveness("last_successful_event_metrics_update", map[string]string{
		"measurement": m.measurement,
	})
	go util.RepeatCtx(ctx, time.Minute, func(ctx context.Context) {
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

// checkTags returns an error if the given map contains reserved tags.
func checkTags(tags map[string]string) error {
	for _, tag := range RESERVED_TAGS {
		if _, ok := tags[tag]; ok {
			return fmt.Errorf("Tag %q is reserved.", tag)
		}
	}
	return nil
}

// AggregateMetric sets the given aggregation function on the event stream and
// adds a gauge for it. For example, to compute the sum of all int64 events over
// a 24-hour period:
//
//	s.AggregateMetric("my-stream", myTags, 24*time.Hour, func(ev []Event) (float64, error) {
//	        sum := int64(0)
//	        for _, e := range ev {
//	                sum += decodeInt64(e)
//	        }
//	        return float64(sum), nil
//	})
func (m *EventMetrics) AggregateMetric(stream string, tags map[string]string, period time.Duration, agg AggregateFn) error {
	mx := &metric{
		measurement: m.measurement,
		tags: map[string]string{
			tagPeriod:      fmt.Sprintf("%s", period),
			tagStream:      stream,
			tagAggregation: "",
		},
		stream: stream,
		period: period,
		agg:    agg,
	}
	if err := checkTags(tags); err != nil {
		return err
	}
	for k, v := range tags {
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

// DynamicMetric sets the given aggregation function on the event stream. Gauges
// will be added and removed dynamically based on the results of the aggregation
// function. Here's a toy example:
//
//	s.DynamicMetric("my-stream", myTags, 24*time.Hour, func(ev []Event) (map[string]float64, error) {
//		counts := map[string]int64{}
//		for _, e := range ev {
//			counts[fmt.Sprintf("%d", decodeInt64(e))]++
//		}
//		rv := make(map[string]float64, len(counts))
//		for k, v := range counts {
//			rv[k] = float64(v)
//		}
//		return rv
//	})
func (m *EventMetrics) DynamicMetric(stream string, tags map[string]string, period time.Duration, agg DynamicAggregateFn) error {
	mx := &dynamicMetric{
		measurement: m.measurement,
		tags: map[string]string{
			tagPeriod:      fmt.Sprintf("%s", period),
			tagStream:      stream,
			tagAggregation: "",
		},
		stream: stream,
		period: period,
		agg:    agg,
	}
	for k, v := range tags {
		if err := checkTags(tags); err != nil {
			return err
		}
		mx.tags[k] = v
	}
	m.mtx.Lock()
	defer m.mtx.Unlock()
	byPeriod, ok := m.dynamicMetrics[stream]
	if !ok {
		byPeriod = map[time.Duration][]*dynamicMetric{}
		m.dynamicMetrics[stream] = byPeriod
	}
	byPeriod[period] = append(byPeriod[period], mx)
	return nil
}

// ComputeStatsMetric sets the given observation function on the event stream.
// Gauges will be added for various aggregation types on the observations
// generated by the function, eg. mean, standard deviation, quantiles, etc. For
// example, to compute statistics for duration of all events over a 24-hour
// period:
//
//	s.ComputeStatsMetric("my-stream", myTags, 24*time.Hour, func(ev []Event) (map[string]float64, error) {
//		vals := make([]float64, 0, len(ev))
//		for _, e := range ev {
//			data := decodeEvent()
//			vals = append(vals, float64(data.End.Sub(data.Start)))
//		}
//		return vals
//	})
func (m *EventMetrics) ComputeStatsMetric(stream string, tags map[string]string, period time.Duration, obs ObservationsFn) error {
	if err := checkTags(tags); err != nil {
		return skerr.Wrap(err)
	}
	metrics := []*metric{}
	for aggregationName, aggregationFn := range computeStatsAggregationFuncs {
		aggregationFn := aggregationFn // https://golang.org/doc/faq#closures_and_goroutines
		agg := func(ev []*Event) (float64, error) {
			observations, err := obs(ev)
			if err != nil {
				return 0.0, skerr.Wrap(err)
			}
			return aggregationFn(observations), nil
		}
		mx := &metric{
			measurement: m.measurement,
			tags: map[string]string{
				tagPeriod:      period.String(),
				tagStream:      stream,
				tagAggregation: aggregationName,
			},
			stream: stream,
			period: period,
			agg:    agg,
		}
		for k, v := range tags {
			mx.tags[k] = v
		}
		metrics = append(metrics, mx)
	}
	m.mtx.Lock()
	defer m.mtx.Unlock()
	byPeriod, ok := m.metrics[stream]
	if !ok {
		byPeriod = map[time.Duration][]*metric{}
		m.metrics[stream] = byPeriod
	}
	for _, mx := range metrics {
		byPeriod[period] = append(byPeriod[period], mx)
	}
	return nil
}

type aggregationFn func([]float64) float64

var computeStatsAggregationFuncs = map[string]aggregationFn{
	"mean":     mean,
	"variance": variance,
	"50th-percentile": func(v []float64) float64 {
		return percentile(v, 0.5)
	},
	"90th-percentile": func(v []float64) float64 {
		return percentile(v, 0.9)
	},
	"99th-percentile": func(v []float64) float64 {
		return percentile(v, 0.99)
	},
}

func mean(v []float64) float64 {
	// Avoid dividing by zero.
	if len(v) == 0 {
		return 0.0
	}
	sum := float64(0.0)
	for _, val := range v {
		sum += val
	}
	return sum / float64(len(v))
}

func variance(v []float64) float64 {
	// Avoid dividing by zero.
	if len(v) == 0 {
		return 0.0
	}
	meanV := mean(v)
	sum := float64(0.0)
	for _, val := range v {
		dist := val - meanV
		sum += dist * dist
	}
	return sum / float64(len(v))
}

func percentile(v []float64, p float64) float64 {
	// Avoid index errors.
	if len(v) == 0 {
		return 0.0
	} else if len(v) == 1 {
		return v[0]
	}
	sort.Float64s(v)
	rank := p * float64(len(v)-1)
	floor := math.Floor(rank)
	ceil := math.Ceil(rank)
	if floor == ceil {
		return v[int(floor)]
	}
	lower := v[int(floor)]
	upper := v[int(ceil)]
	fraction := rank - floor
	return lower + fraction*(upper-lower)
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

// updateDynamicMetric updates the values for a dynamic metric. Returns the
// metrics which were updated in a map by ID, or an error if applicable.
func (m *EventMetrics) updateDynamicMetric(ev []*Event, mx *dynamicMetric) (map[string]metrics2.Float64Metric, error) {
	tagSets, values, err := mx.agg(ev)
	if err != nil {
		return nil, err
	}
	if len(tagSets) != len(values) {
		return nil, fmt.Errorf("DynamicAggregateFn must return slices of tags and values of equal length (got %d vs %d).", len(tagSets), len(values))
	}
	gotMetrics := map[string]metrics2.Float64Metric{}
	for i, dynamicTags := range tagSets {
		tags := util.CopyStringMap(mx.tags)
		if err := checkTags(dynamicTags); err != nil {
			return nil, err
		}
		for k, v := range dynamicTags {
			tags[k] = v
		}
		m := metrics2.GetFloat64Metric(mx.measurement, tags)
		m.Update(values[i])
		// TODO(borenet): Should we include the measurement name?
		id, err := util.MD5Sum(tags)
		if err != nil {
			return nil, err
		}
		gotMetrics[id] = m
	}
	return gotMetrics, nil
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
				errs = append(errs, fmt.Errorf("Failed to retrieve %q events from range %s - %s: %s", stream, now.Add(-period), now, err))
				continue
			}
			for _, mx := range metrics {
				if err := m.updateMetric(ev, mx); err != nil {
					errs = append(errs, fmt.Errorf("Failed to update metric: %+v\n%s", mx, err))
					continue
				}
			}
		}
	}
	gotDynamicMetrics := map[string]metrics2.Float64Metric{}
	for stream, byPeriod := range m.dynamicMetrics {
		for period, metrics := range byPeriod {
			ev, err := m.db.Range(stream, now.Add(-period), now)
			if err != nil {
				errs = append(errs, fmt.Errorf("Failed to retrieve %q events from range %s - %s (2): %s", stream, now.Add(-period), now, err))
				continue
			}
			for _, mx := range metrics {
				got, err := m.updateDynamicMetric(ev, mx)
				if err != nil {
					errs = append(errs, fmt.Errorf("Failed to update dynamic metric: %+v\n%s", mx, err))
					continue
				}
				for k, v := range got {
					gotDynamicMetrics[k] = v
				}
			}
		}
	}
	// Delete any no-longer-generated metrics.
	for k, v := range m.currentDynamicMetrics {
		if _, ok := gotDynamicMetrics[k]; !ok {
			if err := v.Delete(); err != nil {
				errs = append(errs, fmt.Errorf("Failed to delete old metric %q: %s", k, err))
			}
		}
	}
	m.currentDynamicMetrics = gotDynamicMetrics
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
