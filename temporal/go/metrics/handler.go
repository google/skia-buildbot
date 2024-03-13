package metrics

import (
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.temporal.io/sdk/client"
)

type metricsHandler struct {
	client   metrics2.Client
	cm       sync.Mutex
	gm       sync.Mutex
	tm       sync.Mutex
	tags     map[string]string
	counters map[string]client.MetricsCounter
	gauges   map[string]client.MetricsGauge
	timers   map[string]client.MetricsTimer
}

// NewMetricsHandler returns a new handler that implements Temporal's MetricsHandler.
func NewMetricsHandler(tags map[string]string, c metrics2.Client) *metricsHandler {
	if c == nil {
		c = metrics2.GetDefaultClient()
	}
	return &metricsHandler{
		client:   c,
		tags:     tags,
		counters: make(map[string]client.MetricsCounter),
		gauges:   make(map[string]client.MetricsGauge),
		timers:   make(map[string]client.MetricsTimer),
	}
}

// timer implements client.MetricsTimer using Float64SummaryMetric
type timer struct {
	metrics2.Float64SummaryMetric
}

func (t *timer) Record(v time.Duration) {
	t.Observe(v.Seconds())
}

func (m *metricsHandler) WithTags(tags map[string]string) client.MetricsHandler {
	return NewMetricsHandler(tags, m.client)
}

func (m *metricsHandler) Counter(name string) client.MetricsCounter {
	if c, ok := m.counters[name]; ok {
		return c
	}
	m.cm.Lock()
	defer m.cm.Unlock()

	if c, ok := m.counters[name]; ok {
		return c
	}
	c := m.client.GetCounter(name, m.tags)
	m.counters[name] = c
	return c
}

func (m *metricsHandler) Gauge(name string) client.MetricsGauge {
	if g, ok := m.gauges[name]; ok {
		return g
	}

	m.gm.Lock()
	defer m.gm.Unlock()

	if g, ok := m.gauges[name]; ok {
		return g
	}
	g := m.client.GetFloat64Metric(name, m.tags)
	m.gauges[name] = g
	return g
}

func (m *metricsHandler) Timer(name string) client.MetricsTimer {
	if t, ok := m.timers[name]; ok {
		return t
	}

	m.tm.Lock()
	defer m.tm.Unlock()

	if t, ok := m.timers[name]; ok {
		return t
	}
	t := &timer{
		Float64SummaryMetric: m.client.GetFloat64SummaryMetric(name, m.tags),
	}
	m.timers[name] = t
	return t
}
