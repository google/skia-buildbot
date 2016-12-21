package metrics2

// int64Metric is a metric which uses int64.
type int64Metric struct {
	*rawMetric
}

// getInt64Metric returns an Int64Metric instance.
func (c *influxClient) getInt64Metric(measurement string, tags ...map[string]string) *int64Metric {
	return &int64Metric{
		c.getRawMetric(measurement, tags, int64(0)),
	}
}

// GetInt64Metric returns an Int64Metric instance.
func (c *influxClient) GetInt64Metric(measurement string, tags ...map[string]string) Int64MetricI {
	return &int64Metric{
		c.getRawMetric(measurement, tags, int64(0)),
	}
}

// GetInt64Metric returns an Int64Metric instance using the default client.
func GetInt64Metric(measurement string, tags ...map[string]string) Int64MetricI {
	return DefaultClient.GetInt64Metric(measurement, tags...)
}

// Get returns the current value of the metric.
func (m *int64Metric) Get() int64 {
	return m.get().(int64)
}

// Update adds a data point to the metric.
func (m *int64Metric) Update(v int64) {
	m.update(v)
}

// float64Metric is a metric which uses float64.
type float64Metric struct {
	*rawMetric
}

// GetFloat64Metric returns a Float64Metric instance. The current value is
func (c *influxClient) GetFloat64Metric(measurement string, tags ...map[string]string) Float64MetricI {
	return &float64Metric{
		c.getRawMetric(measurement, tags, float64(0)),
	}
}

// GetFloat64Metric returns a Float64Metric instance using the default client.
func GetFloat64Metric(measurement string, tags ...map[string]string) Float64MetricI {
	return DefaultClient.GetFloat64Metric(measurement, tags...)
}

// Get returns the current value of the metric.
func (m *float64Metric) Get() float64 {
	return m.get().(float64)
}

// Update adds a data point to the metric.
func (m *float64Metric) Update(v float64) {
	m.update(v)
}

// boolMetric is a metric which uses bool.
type boolMetric struct {
	*rawMetric
}

// GetBoolMetric returns a BoolMetric instance. The current value is reported
// at the given frequency; if the report frequency is zero, the value is only
// reported when it changes.
func (c *influxClient) GetBoolMetric(measurement string, tags ...map[string]string) BoolMetricI {
	return &boolMetric{
		c.getRawMetric(measurement, tags, false),
	}
}

// GetBoolMetric returns a BoolMetric instance using the default client. The
// current value is reported at the given frequency; if the report frequency is
// zero, the value is only reported when it changes.
func GetBoolMetric(measurement string, tags ...map[string]string) BoolMetricI {
	return DefaultClient.GetBoolMetric(measurement, tags...)
}

// Get returns the current value of the metric.
func (m *boolMetric) Get() bool {
	return m.get().(bool)
}

// Update adds a data point to the metric.
func (m *boolMetric) Update(v bool) {
	m.update(v)
}
