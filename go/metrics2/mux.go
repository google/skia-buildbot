package metrics2

import "time"

// MuxClient is a Client that can be used to multiplex
// two Clients, it implements the Client interface.
type MuxClient struct {
	clients []Client
}

// Flush pushes any queued data immediately. Long running apps shouldn't worry about this as Client will auto-push every so often.
func (m *MuxClient) Flush() error {
	var err error
	for _, c := range m.clients {
		if nextErr := c.Flush(); nextErr != nil {
			err = nextErr
		}
	}
	return err
}

// GetCounter creates or retrieves a Counter with the given name and tag set and returns it.
func (m *MuxClient) GetCounter(name string, tagList ...map[string]string) Counter {
	ret := &MuxCounter{
		metrics: []Counter{},
	}
	for _, c := range m.clients {
		ret.metrics = append(ret.metrics, c.GetCounter(name, tagList...))
	}
	return ret
}

// GetFloat64Metric returns a Float64Metric instance.
func (m *MuxClient) GetFloat64Metric(name string, tagList ...map[string]string) Float64Metric {
	ret := &MuxFloat64Metric{
		metrics: []Float64Metric{},
	}
	for _, c := range m.clients {
		ret.metrics = append(ret.metrics, c.GetFloat64Metric(name, tagList...))
	}
	return ret
}

// GetInt64Metric returns an Int64Metric instance.
func (m *MuxClient) GetInt64Metric(name string, tagList ...map[string]string) Int64Metric {
	ret := &MuxInt64Metric{
		metrics: []Int64Metric{},
	}
	for _, c := range m.clients {
		ret.metrics = append(ret.metrics, c.GetInt64Metric(name, tagList...))
	}
	return ret
}

// NewLiveness creates a new Liveness metric helper.
func (m *MuxClient) NewLiveness(name string, tagList ...map[string]string) Liveness {
	ret := &MuxLiveness{
		livenesses: []Liveness{},
	}
	for _, c := range m.clients {
		ret.livenesses = append(ret.livenesses, c.NewLiveness(name, tagList...))
	}
	return ret
}

// NewTimer creates and returns a new started timer.
func (m *MuxClient) NewTimer(name string, tagList ...map[string]string) Timer {
	ret := &MuxTimer{
		timers: []Timer{},
	}
	for _, c := range m.clients {
		ret.timers = append(ret.timers, c.NewTimer(name, tagList...))
	}
	return ret
}

// MuxTimer is a struct used for measuring elapsed time. Unlike the other metrics
// helpers, timer does not continuously report data; instead, it reports a
// single data point when Stop() is called.
type MuxTimer struct {
	timers []Timer
}

// Start starts or resets the timer.
func (mt *MuxTimer) Start() {
	for _, t := range mt.timers {
		t.Start()
	}
}

// Stop stops the timer and reports the elapsed time.
func (mt *MuxTimer) Stop() {
	for _, t := range mt.timers {
		t.Start()
	}
}

// MuxLiveness keeps a time-since-last-successful-update metric.
//
// The unit of the metrics is in seconds.
//
// It is used to keep track of periodic processes to make sure that they are running
// successfully. Every liveness metric should have a corresponding alert set up that
// will fire of the time-since-last-successful-update metric gets too large.
type MuxLiveness struct {
	livenesses []Liveness
}

// Delete removes the Liveness from metrics.
func (ml *MuxLiveness) Delete() error {
	var err error
	for _, l := range ml.livenesses {
		l.Delete()
		if nextErr := l.Delete(); nextErr != nil {
			err = nextErr
		}
	}
	return err
}

// Get returns the current value of the Liveness.
func (ml *MuxLiveness) Get() int64 {
	return ml.livenesses[0].Get()
}

// ManualReset sets the last-successful-update time of the Liveness to a specific value. Useful for tracking processes whose lifetimes are outside of that of the current process, but should not be needed in most cases.
func (ml *MuxLiveness) ManualReset(lastSuccessfulUpdate time.Time) {
	for _, l := range ml.livenesses {
		l.ManualReset(lastSuccessfulUpdate)
	}
}

// Reset should be called when some work has been successfully completed.
func (ml *MuxLiveness) Reset() {
	for _, l := range ml.livenesses {
		l.Reset()
	}
}

// MuxInt64Metric is a metric which reports an int64 value.
type MuxInt64Metric struct {
	metrics []Int64Metric
}

// Delete removes the metric from its Client's registry.
func (mi *MuxInt64Metric) Delete() error {
	var err error
	for _, l := range mi.metrics {
		l.Delete()
		if nextErr := l.Delete(); nextErr != nil {
			err = nextErr
		}
	}
	return err
}

// Get returns the current value of the metric.
func (mi *MuxInt64Metric) Get() int64 {
	return mi.metrics[0].Get()
}

// Update adds a data point to the metric.
func (mi *MuxInt64Metric) Update(v int64) {
	for _, m := range mi.metrics {
		m.Update(v)
	}
}

// Float64Metric is a metric which reports a float64 value.
type MuxFloat64Metric struct {
	metrics []Float64Metric
}

// Delete removes the metric from its Client's registry.
func (mf *MuxFloat64Metric) Delete() error {
	var err error
	for _, l := range mf.metrics {
		l.Delete()
		if nextErr := l.Delete(); nextErr != nil {
			err = nextErr
		}
	}
	return err
}

// Get returns the current value of the metric.
func (mf *MuxFloat64Metric) Get() float64 {
	return mf.metrics[0].Get()
}

// Update adds a data point to the metric.
func (mf *MuxFloat64Metric) Update(v float64) {
	for _, m := range mf.metrics {
		m.Update(v)
	}
}

// Counter is a struct used for tracking metrics which increment or decrement.
type MuxCounter struct {
	metrics []Counter
}

// Dec decrements the counter by the given quantity.
func (mc *MuxCounter) Dec(i int64) {
	for _, m := range mc.metrics {
		m.Dec(i)
	}
}

// Delete removes the counter from metrics.
func (mc *MuxCounter) Delete() error {
	var err error
	for _, l := range mc.metrics {
		l.Delete()
		if nextErr := l.Delete(); nextErr != nil {
			err = nextErr
		}
	}
	return err
}

// Get returns the current value in the counter.
func (mc *MuxCounter) Get() int64 {
	return mc.metrics[0].Get()
}

// Inc increments the counter by the given quantity.
func (mc *MuxCounter) Inc(i int64) {
	for _, m := range mc.metrics {
		m.Inc(i)
	}
}

// Reset sets the counter to zero.
func (mc *MuxCounter) Reset() {
	for _, m := range mc.metrics {
		m.Reset()
	}
}

// Validate that the concrete structs faithfully implement their respective interfaces.
var _ Client = (*MuxClient)(nil)
var _ Counter = (*MuxCounter)(nil)
var _ Float64Metric = (*MuxFloat64Metric)(nil)
var _ Int64Metric = (*MuxInt64Metric)(nil)
var _ Liveness = (*MuxLiveness)(nil)
var _ Timer = (*MuxTimer)(nil)
