package metrics2

// Int64Metric is a metric which uses int64.
type Int64Metric struct {
	*rawMetric
}

// GetInt64Metric returns an Int64Metric instance.
func (c *Client) GetInt64Metric(measurement string, tags map[string]string) *Int64Metric {
	return &Int64Metric{
		c.getRawMetric(measurement, tags, int64(0)),
	}
}

// GetInt64Metric returns an Int64Metric instance using the default client.
func GetInt64Metric(measurement string, tags map[string]string) *Int64Metric {
	return DefaultClient.GetInt64Metric(measurement, tags)
}

// Get returns the current value of the metric.
func (m *Int64Metric) Get() int64 {
	return m.get().(int64)
}

// Update adds a data point to the metric.
func (m *Int64Metric) Update(v int64) {
	m.update(v)
}

// Float64Metric is a metric which uses float64.
type Float64Metric struct {
	*rawMetric
}

// GetFloat64Metric returns a Float64Metric instance. The current value is
func (c *Client) GetFloat64Metric(measurement string, tags map[string]string) *Float64Metric {
	return &Float64Metric{
		c.getRawMetric(measurement, tags, float64(0)),
	}
}

// GetFloat64Metric returns a Float64Metric instance using the default client.
func GetFloat64Metric(measurement string, tags map[string]string) *Float64Metric {
	return DefaultClient.GetFloat64Metric(measurement, tags)
}

// Get returns the current value of the metric.
func (m *Float64Metric) Get() float64 {
	return m.get().(float64)
}

// Update adds a data point to the metric.
func (m *Float64Metric) Update(v float64) {
	m.update(v)
}

// BoolMetric is a metric which uses bool.
type BoolMetric struct {
	*rawMetric
}

// GetBoolMetric returns a BoolMetric instance. The current value is reported
// at the given frequency; if the report frequency is zero, the value is only
// reported when it changes.
func (c *Client) GetBoolMetric(measurement string, tags map[string]string) *BoolMetric {
	return &BoolMetric{
		c.getRawMetric(measurement, tags, false),
	}
}

// GetBoolMetric returns a BoolMetric instance using the default client. The
// current value is reported at the given frequency; if the report frequency is
// zero, the value is only reported when it changes.
func GetBoolMetric(measurement string, tags map[string]string) *BoolMetric {
	return DefaultClient.GetBoolMetric(measurement, tags)
}

// Get returns the current value of the metric.
func (m *BoolMetric) Get() bool {
	return m.get().(bool)
}

// Update adds a data point to the metric.
func (m *BoolMetric) Update(v bool) {
	m.update(v)
}
