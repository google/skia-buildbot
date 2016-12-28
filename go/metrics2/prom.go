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

// See interface.
func (m *promInt64) Get() int64 {
	return atomic.LoadInt64(&(m.i))
}

// See interface.
func (m *promInt64) Update(v int64) {
	atomic.StoreInt64(&(m.i), v)
	m.gauge.Set(float64(v))
}

// See interface.
func (m *promInt64) Delete() error {
	// The delete is a lie.
	return nil
}

// promClient implements the Client interface.
type promClient struct {
	int64GaugeVecs map[string]*prometheus.GaugeVec
	int64Gauges    map[string]*promInt64
	int64Mutex     sync.Mutex
}

func newPromClient() *promClient {
	return &promClient{
		int64GaugeVecs: map[string]*prometheus.GaugeVec{},
		int64Gauges:    map[string]*promInt64{},
	}
}

// See interface.
func (p *promClient) GetInt64Metric(measurement string, tags ...map[string]string) Int64Metric {
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

	p.int64Mutex.Lock()
	ret, ok := p.int64Gauges[gaugeKey]
	p.int64Mutex.Unlock()

	if ok {
		return ret
	}

	// Didn't find the metric, so we need to look for a GaugeVec to create it under.
	gaugeVecKey := fmt.Sprintf("%s %v", measurement, keys)
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

// Validate that the concrete structs faithfully implement their respective interfaces.
var _ Int64Metric = (*promInt64)(nil)

/*
var _ Client = (*promClient)(nil)
var _ Counter = (*promCounter)(nil)
var _ Float64Metric = (*promFloat64)(nil)
var _ Liveness = (*promLiveness)(nil)
var _ Timer = (*promTimer)(nil)
*/
