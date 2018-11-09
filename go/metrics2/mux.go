package metrics2

import (
	"fmt"
	"time"
)

// muxClient is a Client that can be used to multiplex
// N Clients, it implements the Client interface.
type muxClient struct {
	clients []Client
}

// newMuxClient creates a new Client that mirrors all calls to one or more
// Clients.
func newMuxClient(clients []Client) (Client, error) {
	if len(clients) == 0 {
		return nil, fmt.Errorf("At least one client must be passed to NewMuxClient.")
	}
	return &muxClient{
		clients: clients,
	}, nil
}

func (m *muxClient) Flush() error {
	var err error
	for _, c := range m.clients {
		if nextErr := c.Flush(); nextErr != nil {
			err = nextErr
		}
	}
	return err
}

func (m *muxClient) GetCounter(name string, tagList ...map[string]string) Counter {
	ret := &muxCounter{
		metrics: []Counter{},
	}
	for _, c := range m.clients {
		ret.metrics = append(ret.metrics, c.GetCounter(name, tagList...))
	}
	return ret
}

func (m *muxClient) GetFloat64Metric(name string, tagList ...map[string]string) Float64Metric {
	ret := &muxFloat64Metric{
		metrics: []Float64Metric{},
	}
	for _, c := range m.clients {
		ret.metrics = append(ret.metrics, c.GetFloat64Metric(name, tagList...))
	}
	return ret
}

func (m *muxClient) GetFloat64SummaryMetric(name string, tagList ...map[string]string) Float64SummaryMetric {
	ret := &muxFloat64SummaryMetric{
		metrics: []Float64SummaryMetric{},
	}
	for _, c := range m.clients {
		ret.metrics = append(ret.metrics, c.GetFloat64SummaryMetric(name, tagList...))
	}
	return ret
}

func (m *muxClient) GetInt64Metric(name string, tagList ...map[string]string) Int64Metric {
	ret := &muxInt64Metric{
		metrics: []Int64Metric{},
	}
	for _, c := range m.clients {
		ret.metrics = append(ret.metrics, c.GetInt64Metric(name, tagList...))
	}
	return ret
}

func (m *muxClient) NewLiveness(name string, tagList ...map[string]string) Liveness {
	ret := &muxLiveness{
		livenesses: []Liveness{},
	}
	for _, c := range m.clients {
		ret.livenesses = append(ret.livenesses, c.NewLiveness(name, tagList...))
	}
	return ret
}

func (m *muxClient) NewTimer(name string, tagList ...map[string]string) Timer {
	ret := &muxTimer{
		timers: []Timer{},
	}
	for _, c := range m.clients {
		ret.timers = append(ret.timers, c.NewTimer(name, tagList...))
	}
	return ret
}

// muxTimer implements the Timer interface.
type muxTimer struct {
	timers []Timer
}

func (mt *muxTimer) Start() {
	for _, t := range mt.timers {
		t.Start()
	}
}

func (mt *muxTimer) Stop() time.Duration {
	var d time.Duration
	for _, t := range mt.timers {
		d = t.Stop()
	}
	return d
}

// muxLiveness implements the Liveness interface.
type muxLiveness struct {
	livenesses []Liveness
}

func (ml *muxLiveness) Get() int64 {
	return ml.livenesses[0].Get()
}

func (ml *muxLiveness) ManualReset(lastSuccessfulUpdate time.Time) {
	for _, l := range ml.livenesses {
		l.ManualReset(lastSuccessfulUpdate)
	}
}

func (ml *muxLiveness) Reset() {
	for _, l := range ml.livenesses {
		l.Reset()
	}
}

// Close implements the Liveness interface.
func (ml *muxLiveness) Close() {
	for _, l := range ml.livenesses {
		l.Close()
	}
}

// muxInt64Metric implements Int64Metric.
type muxInt64Metric struct {
	metrics []Int64Metric
}

func (mi *muxInt64Metric) Delete() error {
	var err error
	for _, l := range mi.metrics {
		if nextErr := l.Delete(); nextErr != nil {
			err = nextErr
		}
	}
	return err
}

func (mi *muxInt64Metric) Get() int64 {
	return mi.metrics[0].Get()
}

func (mi *muxInt64Metric) Update(v int64) {
	for _, m := range mi.metrics {
		m.Update(v)
	}
}

// muxFloat64Metric implements the Float64Metric interface.
type muxFloat64Metric struct {
	metrics []Float64Metric
}

func (mf *muxFloat64Metric) Delete() error {
	var err error
	for _, l := range mf.metrics {
		if nextErr := l.Delete(); nextErr != nil {
			err = nextErr
		}
	}
	return err
}

func (mf *muxFloat64Metric) Get() float64 {
	return mf.metrics[0].Get()
}

func (mf *muxFloat64Metric) Update(v float64) {
	for _, m := range mf.metrics {
		m.Update(v)
	}
}

// muxFloat64SummaryMetric implements the Float64SummaryMetric interface.
type muxFloat64SummaryMetric struct {
	metrics []Float64SummaryMetric
}

func (mf *muxFloat64SummaryMetric) Observe(v float64) {
	for _, m := range mf.metrics {
		m.Observe(v)
	}
}

// muxCounter implements the Counter interface.
type muxCounter struct {
	metrics []Counter
}

func (mc *muxCounter) Dec(i int64) {
	for _, m := range mc.metrics {
		m.Dec(i)
	}
}

func (mc *muxCounter) Delete() error {
	var err error
	for _, l := range mc.metrics {
		if nextErr := l.Delete(); nextErr != nil {
			err = nextErr
		}
	}
	return err
}

func (mc *muxCounter) Get() int64 {
	return mc.metrics[0].Get()
}

func (mc *muxCounter) Inc(i int64) {
	for _, m := range mc.metrics {
		m.Inc(i)
	}
}

func (mc *muxCounter) Reset() {
	for _, m := range mc.metrics {
		m.Reset()
	}
}

// Validate that the concrete structs faithfully implement their respective interfaces.
var _ Client = (*muxClient)(nil)
var _ Counter = (*muxCounter)(nil)
var _ Float64Metric = (*muxFloat64Metric)(nil)
var _ Int64Metric = (*muxInt64Metric)(nil)
var _ Liveness = (*muxLiveness)(nil)
var _ Timer = (*muxTimer)(nil)
