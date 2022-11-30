package metrics2

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	// invalidChar is used to force metric and tag names to conform to Prometheus's restrictions.
	invalidChar = regexp.MustCompile("([^a-zA-Z0-9_:])")
)

func clean(s string) string {
	if invalidChar.MatchString(s) {
		sklog.Warningf("Hey, metrics string %s should not have invalid characters in it", s)
	}
	return invalidChar.ReplaceAllLiteralString(s, "_")
}

// promInt64 implements the Int64Metric interface.
type promInt64 struct {
	// i tracks the value of the gauge, because prometheus client lib doesn't
	// support get on Gauge values.
	mutex  sync.Mutex
	i      int64
	gauge  prometheus.Gauge
	delete func() error
}

func (m *promInt64) Get() int64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.i
}

func (m *promInt64) Update(v int64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.i = v
	m.gauge.Set(float64(v))
}

func (m *promInt64) Delete() error {
	return m.delete()
}

// promBool is a facade around promInt64 and implements the BoolMetric interface. It lets the caller
// operate in terms of bools but stores them as 1 and 0 in Prometheus.
type promBool struct {
	promInt *promInt64
}

func (pb *promBool) Delete() error {
	return pb.promInt.Delete()
}

func (pb *promBool) Get() bool {
	return pb.promInt.Get() == 0
}

func (pb *promBool) Update(v bool) {
	var i int64
	if v {
		i = 1
	}
	pb.promInt.Update(i)
}

// promFloat64 implements the Float64Metric interface.
type promFloat64 struct {
	// i tracks the value of the gauge, because prometheus client lib doesn't
	// support get on Gauge values.
	mutex  sync.Mutex
	i      float64
	gauge  prometheus.Gauge
	delete func() error
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
	return m.delete()
}

// promFloat64Summary implements the Float64Metric interface.
type promFloat64Summary struct {
	observer prometheus.Observer
}

func (m *promFloat64Summary) Observe(v float64) {
	m.observer.Observe(v)
}

// promCounter implements the Counter interface.
type promCounter struct {
	pi    *promInt64
	mutex sync.Mutex
}

func (pc *promCounter) Get() int64 {
	// Doesn't need to be locked: Get is atomic, and if Inc or Dec is called concurrently, either old
	// or new value is fine.
	return pc.pi.Get()
}

func (pc *promCounter) Inc(i int64) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()
	pc.pi.Update(pc.pi.Get() + i)
}

func (pc *promCounter) Dec(i int64) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()
	pc.pi.Update(pc.pi.Get() - i)
}

func (pc *promCounter) Reset() {
	// Needs a lock to avoid race with Inc/Dec.
	pc.mutex.Lock()
	defer pc.mutex.Unlock()
	pc.pi.Update(0)
}

func (pc *promCounter) Delete() error {
	return pc.pi.delete()
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

func NewPromClient() *promClient {
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

	p.int64Mutex.Lock()
	defer p.int64Mutex.Unlock()

	if ret, ok := p.int64Gauges[gaugeKey]; ok {
		return ret
	}

	// Didn't find the metric, so we need to look for a GaugeVec to create it under.
	gaugeVec, ok := p.int64GaugeVecs[gaugeVecKey]
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
			sklog.Fatalf("Failed to register %q: %s", measurement, skerr.Wrap(err))
		}
		p.int64GaugeVecs[gaugeVecKey] = gaugeVec
	}

	labels := prometheus.Labels(cleanTags)
	gauge, err := gaugeVec.GetMetricWith(labels)
	if err != nil {
		sklog.Fatalf("Failed to get gauge: %s", skerr.Wrap(err))
	}
	ret := &promInt64{
		delete: func() error {
			p.int64Mutex.Lock()
			defer p.int64Mutex.Unlock()
			if !gaugeVec.Delete(labels) {
				return fmt.Errorf("Failed to delete metric %s-%#v.", measurement, labels)
			}
			delete(p.int64Gauges, gaugeKey)
			return nil
		},
		gauge: gauge,
	}

	p.int64Gauges[gaugeKey] = ret
	return ret
}

func (p *promClient) GetBoolMetric(name string, tags ...map[string]string) BoolMetric {
	intMetric := p.GetInt64Metric(name, tags...)
	return &promBool{
		promInt: intMetric.(*promInt64),
	}
}

func (p *promClient) GetCounter(name string, tags ...map[string]string) Counter {
	i64 := p.GetInt64Metric(name, tags...)
	return &promCounter{
		pi: (i64.(*promInt64)),
	}
}

func (p *promClient) GetFloat64Metric(name string, tags ...map[string]string) Float64Metric {
	measurement, cleanTags, keys, gaugeKey, gaugeVecKey := p.commonGet(name, tags...)

	p.float64Mutex.Lock()
	defer p.float64Mutex.Unlock()

	if ret, ok := p.float64Gauges[gaugeKey]; ok {
		return ret
	}

	// Didn't find the metric, so we need to look for a GaugeVec to create it under.
	gaugeVec, ok := p.float64GaugeVecs[gaugeVecKey]
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
			sklog.Fatalf("Failed to register %q: %s", measurement, skerr.Wrap(err))
		}
		p.float64GaugeVecs[gaugeVecKey] = gaugeVec
	}

	labels := prometheus.Labels(cleanTags)
	gauge, err := gaugeVec.GetMetricWith(labels)
	if err != nil {
		sklog.Fatalf("Failed to get gauge: %s", skerr.Wrap(err))
	}
	ret := &promFloat64{
		delete: func() error {
			p.float64Mutex.Lock()
			defer p.float64Mutex.Unlock()
			if !gaugeVec.Delete(labels) {
				return fmt.Errorf("Failed to delete metric %s-%#v.", measurement, labels)
			}
			delete(p.float64Gauges, gaugeKey)
			return nil
		},
		gauge: gauge,
	}
	p.float64Gauges[gaugeKey] = ret
	return ret
}

func (p *promClient) GetFloat64SummaryMetric(name string, tags ...map[string]string) Float64SummaryMetric {
	measurement, cleanTags, keys, summaryKey, summaryVecKey := p.commonGet(name, tags...)

	p.float64SummaryMutex.Lock()
	defer p.float64SummaryMutex.Unlock()

	if ret, ok := p.float64Summaries[summaryKey]; ok {
		return ret
	}

	// Didn't find the metric, so we need to look for a SummaryVec to create it under.
	summaryVec, ok := p.float64SummaryVecs[summaryVecKey]
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
			sklog.Fatalf("Failed to register %q %v: %s", measurement, cleanTags, skerr.Wrap(err))
		}
		p.float64SummaryVecs[summaryVecKey] = summaryVec
	}

	observer, err := summaryVec.GetMetricWith(cleanTags)
	if err != nil {
		sklog.Fatalf("Failed to get observer: %s", skerr.Wrap(err))
	}
	ret := &promFloat64Summary{
		observer: observer,
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

func (c *promClient) Int64MetricExists(name string, tags ...map[string]string) bool {
	_, _, _, gaugeKey, _ := c.commonGet(name, tags...)

	c.int64Mutex.Lock()
	defer c.int64Mutex.Unlock()

	_, ok := c.int64Gauges[gaugeKey]
	return ok
}

// Validate that the concrete structs faithfully implement their respective interfaces.
var _ Int64Metric = (*promInt64)(nil)
var _ BoolMetric = (*promBool)(nil)
var _ Float64Metric = (*promFloat64)(nil)
var _ Float64SummaryMetric = (*promFloat64Summary)(nil)
var _ Counter = (*promCounter)(nil)
var _ Client = (*promClient)(nil)
