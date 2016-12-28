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

// promInt64 implements the Float64Metric interface.
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
}

func newPromClient() *promClient {
	return &promClient{
		int64GaugeVecs:   map[string]*prometheus.GaugeVec{},
		int64Gauges:      map[string]*promInt64{},
		float64GaugeVecs: map[string]*prometheus.GaugeVec{},
		float64Gauges:    map[string]*promFloat64{},
	}
}

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

func (p *promClient) GetFloat64Metric(name string, tags ...map[string]string) Float64Metric {
	measurement, cleanTags, keys, gaugeKey, gaugeVecKey := p.commonGet(name, tags...)

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

// Validate that the concrete structs faithfully implement their respective interfaces.
var _ Int64Metric = (*promInt64)(nil)
var _ Float64Metric = (*promFloat64)(nil)
var _ Counter = (*promCounter)(nil)

/*
var _ Client = (*promClient)(nil)
var _ Liveness = (*promLiveness)(nil)
var _ Timer = (*promTimer)(nil)
*/
