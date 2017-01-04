package metrics2

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// invalidChar is used to force metric and tag names to conform to Prometheus's restrictions.
	invalidChar = regexp.MustCompile("([^a-zA-Z0-9_:])")
)

func clean(s string) string {
	return invalidChar.ReplaceAllLiteralString(s, "_")
}

// promInt64 implements the Int64Metric interface.
type promInt64 struct {
	// i tracks the value of the gauge, because prometheus client lib doesn't
	// support get on Gauge values.
	i     int64
	gauge prometheus.Gauge
}

func (m *promInt64) Get() int64 {
	return atomic.LoadInt64(&(m.i))
}

func (m *promInt64) Update(v int64) {
	atomic.StoreInt64(&(m.i), v)
	m.gauge.Set(float64(v))
}

func (m *promInt64) Delete() error {
	// The delete is a lie.
	return nil
}

// promFloat64 implements the Float64Metric interface.
type promFloat64 struct {
	// i tracks the value of the gauge, because prometheus client lib doesn't
	// support get on Gauge values.
	mutex sync.Mutex
	i     float64
	gauge prometheus.Gauge
}

func (m *promFloat64) Get() float64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.i
}

func (m *promFloat64) Update(v float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.i = v
	m.gauge.Set(float64(v))
}

func (m *promFloat64) Delete() error {
	// The delete is a lie.
	return nil
}

// promFloat64Summary implements the Float64Metric interface.
type promFloat64Summary struct {
	summary prometheus.Summary
}

func (m *promFloat64Summary) Observe(v float64) {
	m.summary.Observe(v)
}

// promCounter implements the Counter interface.
type promCounter struct {
	promInt64
}

func (pc *promCounter) Inc(i int64) {
	pc.Update(pc.Get() + i)
}

func (pc *promCounter) Dec(i int64) {
	pc.Update(pc.Get() - i)
}

func (pc *promCounter) Reset() {
	pc.Update(0)
}

// promClient implements the Client interface.
type promClient struct {
	int64GaugeVecs map[string]*prometheus.GaugeVec
	int64Gauges    map[string]*promInt64
	int64Mutex     sync.Mutex

	float64GaugeVecs map[string]*prometheus.GaugeVec
	float64Gauges    map[string]*promFloat64
	float64Mutex     sync.Mutex

	float64SummaryVecs  map[string]*prometheus.SummaryVec
	float64Summaries    map[string]*promFloat64Summary
	float64SummaryMutex sync.Mutex
}

func newPromClient() *promClient {
	return &promClient{
		int64GaugeVecs:     map[string]*prometheus.GaugeVec{},
		int64Gauges:        map[string]*promInt64{},
		float64GaugeVecs:   map[string]*prometheus.GaugeVec{},
		float64Gauges:      map[string]*promFloat64{},
		float64SummaryVecs: map[string]*prometheus.SummaryVec{},
		float64Summaries:   map[string]*promFloat64Summary{},
	}
}

// commonGet does a lot of the common work for each of the Get* funcs.
//
// It returns:
//   measurement - A clean measurement name.
//   cleanTags   - A clean set of tags.
//   keys        - A slice of the keys of cleanTags, sorted.
//   gaugeKey    - A name to uniquely identify the metric.
//   gaugeVecKey - A name to uniquely identify the collection of metrics. See the Prometheus
//                 docs about Collections.
func (p *promClient) commonGet(measurement string, tags ...map[string]string) (string, map[string]string, []string, string, string) {
	// Convert measurement to a safe name.
	measurement = clean(measurement)

	// Merge all tags.
	rawTags := util.AddParams(map[string]string{}, tags...)

	// Make all label keys safe.
	cleanTags := map[string]string{}
	keys := []string{}
	for k, v := range rawTags {
		key := clean(k)
		cleanTags[key] = v
		keys = append(keys, key)
	}

	// Sort tag keys.
	sort.Strings(keys)

	// Create a key to look up the gauge.
	gaugeKeySrc := []string{measurement}
	for _, key := range keys {
		gaugeKeySrc = append(gaugeKeySrc, key, cleanTags[key])
	}
	gaugeKey := strings.Join(gaugeKeySrc, "-")
	gaugeVecKey := fmt.Sprintf("%s %v", measurement, keys)

	return measurement, cleanTags, keys, gaugeKey, gaugeVecKey
}

func (p *promClient) GetInt64Metric(name string, tags ...map[string]string) Int64Metric {
	measurement, cleanTags, keys, gaugeKey, gaugeVecKey := p.commonGet(name, tags...)
	sklog.Debugf("GetInt64Metric: %s %s", gaugeKey, gaugeVecKey)

	p.int64Mutex.Lock()
	ret, ok := p.int64Gauges[gaugeKey]
	p.int64Mutex.Unlock()

	if ok {
		return ret
	}

	// Didn't find the metric, so we need to look for a GaugeVec to create it under.
	p.int64Mutex.Lock()
	gaugeVec, ok := p.int64GaugeVecs[gaugeVecKey]
	p.int64Mutex.Unlock()

	if !ok {
		// Register a new gauge vec.
		gaugeVec = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: measurement,
				Help: measurement,
			},
			keys,
		)
		err := prometheus.Register(gaugeVec)
		if err != nil {
			sklog.Fatalf("Failed to register %q: %s", measurement, err)
		}
		p.int64GaugeVecs[gaugeVecKey] = gaugeVec
	}
	gauge, err := gaugeVec.GetMetricWith(prometheus.Labels(cleanTags))
	if err != nil {
		sklog.Fatalf("Failed to get gauge: %s", err)
	}
	ret = &promInt64{
		gauge: gauge,
	}
	p.int64Gauges[gaugeKey] = ret
	return ret
}

func (p *promClient) GetCounter(name string, tags ...map[string]string) Counter {
	i64 := p.GetInt64Metric(name, tags...)
	return &promCounter{
		promInt64: *(i64.(*promInt64)),
	}
}

func (p *promClient) GetFloat64Metric(name string, tags ...map[string]string) Float64Metric {
	measurement, cleanTags, keys, gaugeKey, gaugeVecKey := p.commonGet(name, tags...)
	sklog.Debugf("GetFloat64Metric: %s %s", gaugeKey, gaugeVecKey)

	p.float64Mutex.Lock()
	ret, ok := p.float64Gauges[gaugeKey]
	p.float64Mutex.Unlock()

	if ok {
		return ret
	}

	// Didn't find the metric, so we need to look for a GaugeVec to create it under.
	p.float64Mutex.Lock()
	gaugeVec, ok := p.float64GaugeVecs[gaugeVecKey]
	p.float64Mutex.Unlock()

	if !ok {
		// Register a new gauge vec.
		gaugeVec = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: measurement,
				Help: measurement,
			},
			keys,
		)
		err := prometheus.Register(gaugeVec)
		if err != nil {
			sklog.Fatalf("Failed to register %q: %s", measurement, err)
		}
		p.float64GaugeVecs[gaugeVecKey] = gaugeVec
	}
	gauge, err := gaugeVec.GetMetricWith(prometheus.Labels(cleanTags))
	if err != nil {
		sklog.Fatalf("Failed to get gauge: %s", err)
	}
	ret = &promFloat64{
		gauge: gauge,
	}
	p.float64Gauges[gaugeKey] = ret
	return ret
}

func (p *promClient) GetFloat64SummaryMetric(name string, tags ...map[string]string) Float64SummaryMetric {
	measurement, cleanTags, keys, summaryKey, summaryVecKey := p.commonGet(name, tags...)
	sklog.Debugf("GetFloat64SummaryMetric: %s %s", summaryKey, summaryVecKey)

	p.float64SummaryMutex.Lock()
	ret, ok := p.float64Summaries[summaryKey]
	p.float64SummaryMutex.Unlock()

	if ok {
		return ret
	}

	// Didn't find the metric, so we need to look for a SummaryVec to create it under.
	p.float64SummaryMutex.Lock()
	summaryVec, ok := p.float64SummaryVecs[summaryVecKey]
	p.float64SummaryMutex.Unlock()

	if !ok {
		// Register a new summary vec.
		summaryVec = prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       measurement,
				Help:       measurement,
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			},
			keys,
		)
		err := prometheus.Register(summaryVec)
		if err != nil {
			sklog.Fatalf("Failed to register %q %v: %s", measurement, cleanTags, err)
		}
		p.float64SummaryVecs[summaryVecKey] = summaryVec
	}
	summary, err := summaryVec.GetMetricWith(prometheus.Labels(cleanTags))
	if err != nil {
		sklog.Fatalf("Failed to get summary: %s", err)
	}
	ret = &promFloat64Summary{
		summary: summary,
	}
	p.float64Summaries[summaryKey] = ret
	return ret
}

func (c *promClient) Flush() error {
	// The Flush is a lie.
	return nil
}

func (c *promClient) NewLiveness(name string, tagsList ...map[string]string) Liveness {
	return newLiveness(c, name, true, tagsList...)
}

func (c *promClient) NewTimer(name string, tagsList ...map[string]string) Timer {
	return newTimer(c, name, true, tagsList...)
}

// Validate that the concrete structs faithfully implement their respective interfaces.
var _ Int64Metric = (*promInt64)(nil)
var _ Float64Metric = (*promFloat64)(nil)
var _ Float64SummaryMetric = (*promFloat64Summary)(nil)
var _ Counter = (*promCounter)(nil)
var _ Client = (*promClient)(nil)
