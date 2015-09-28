// metrics is a package for helper functions for working go-metrics.
package metrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
)

// DEFAULT_WINDOW is the default window size for SlidingWindow.
const DEFAULT_WINDOW = time.Minute

// Liveness keeps a time-since-last-successful-update metric.
//
// The unit of the metrics is in seconds.
//
// It is used to keep track of periodic processes to make sure that they are running
// successfully. Every liveness metric should have a corresponding alert set up that
// will fire of the time-since-last-successful-update metric gets too large.
type Liveness struct {
	lastSuccessfulUpdate           time.Time
	timeSinceLastSucceessfulUpdate metrics.Gauge
}

// Update should be called when some work has been successfully completed.
func (l *Liveness) Update() {
	l.lastSuccessfulUpdate = time.Now()
}

// NewLiveness creates a new Liveness metric helper.
func NewLiveness(name string) *Liveness {
	l := &Liveness{
		lastSuccessfulUpdate:           time.Now(),
		timeSinceLastSucceessfulUpdate: metrics.NewRegisteredGauge(name+".time-since-last-successful-update", metrics.DefaultRegistry),
	}
	l.timeSinceLastSucceessfulUpdate.Update(int64(time.Since(l.lastSuccessfulUpdate).Seconds()))
	go func() {
		for _ = range time.Tick(time.Minute) {
			l.timeSinceLastSucceessfulUpdate.Update(int64(time.Since(l.lastSuccessfulUpdate).Seconds()))
		}
	}()
	return l
}

var registry = map[string]*SlidingWindow{}
var registryMutex = sync.Mutex{}

// SlidingWindow stores data points over a time period and reports statistics on
// the data to go-metrics every minute.
type SlidingWindow struct {
	data   []int64
	mutex  sync.Mutex
	name   string
	period time.Duration
	times  []time.Time
}

// GetOrRegisterSlidingWindow returns the SlidingWindow with the given name,
// creating it if necessary.
func GetOrRegisterSlidingWindow(name string, period time.Duration) *SlidingWindow {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	if _, ok := registry[name]; !ok {
		r := SlidingWindow{
			data:   []int64{},
			mutex:  sync.Mutex{},
			name:   name,
			period: period,
			times:  []time.Time{},
		}
		go func() {
			for _ = range time.Tick(time.Minute) {
				r.report()
			}
		}()
		registry[name] = &r
	}
	return registry[name]
}

// Update inserts a data point.
func (r *SlidingWindow) Update(v int64) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.data = append(r.data, v)
	r.times = append(r.times, time.Now())
}

// report sends statistics to go-metrics.
func (r *SlidingWindow) report() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	limit := time.Now().Add(-r.period)

	// Evict old data points.
	idx := 0
	for _, t := range r.times {
		if t.After(limit) {
			break
		}
		idx++
	}
	r.data = r.data[idx:]
	r.times = r.times[idx:]
	if len(r.data) == 0 {
		return
	}

	// Calculate stats.
	max := metrics.SampleMax(r.data)
	mean := metrics.SampleMean(r.data)
	min := metrics.SampleMin(r.data)
	percentiles := []float64{0.5, 0.75, 0.95, 0.99}
	percentileVals := metrics.SamplePercentiles(r.data, percentiles)
	stddev := metrics.SampleStdDev(r.data)
	sum := metrics.SampleSum(r.data)
	variance := metrics.SampleVariance(r.data)

	// Log stats into Gauges.
	reg := metrics.DefaultRegistry
	metrics.GetOrRegisterGauge(r.name+".max", reg).Update(max)
	metrics.GetOrRegisterGaugeFloat64(r.name+".mean", reg).Update(mean)
	metrics.GetOrRegisterGauge(r.name+".min", reg).Update(min)
	for i, p := range percentiles {
		metric := r.name + "." + fmt.Sprintf("%0.2f", p)[2:]
		metrics.GetOrRegisterGaugeFloat64(metric, reg).Update(percentileVals[i])
	}
	metrics.GetOrRegisterGaugeFloat64(r.name+".std-dev", reg).Update(stddev)
	metrics.GetOrRegisterGauge(r.name+".sum", reg).Update(sum)
	metrics.GetOrRegisterGaugeFloat64(r.name+".variance", reg).Update(variance)
	metrics.GetOrRegisterGauge(r.name+".count", reg).Update(int64(len(r.data)))
}

// Timer is a timer which reports its result to go-metrics.
//
// The standard way to use Timer is at the top of the func you
// want to measure:
//
//    defer metrics.NewTimer("myapp.myFunction").Stop()
//
type Timer struct {
	Begin  time.Time
	Metric string
}

func NewTimer(metric string) *Timer {
	return &Timer{
		Begin:  time.Now(),
		Metric: metric,
	}
}

func (t Timer) Stop() {
	v := int64(time.Now().Sub(t.Begin))
	GetOrRegisterSlidingWindow(t.Metric, DEFAULT_WINDOW).Update(v)
}
