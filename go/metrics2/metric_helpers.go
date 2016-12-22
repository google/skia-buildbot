package metrics2

// int64Metric implements Int64Metric.
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
func (c *influxClient) GetInt64Metric(measurement string, tags ...map[string]string) Int64Metric {
	return &int64Metric{
		c.getRawMetric(measurement, tags, int64(0)),
	}
}

// GetInt64Metric returns an Int64Metric instance using the default client.
func GetInt64Metric(measurement string, tags ...map[string]string) Int64Metric {
	return defaultClient.GetInt64Metric(measurement, tags...)
}

// Get returns the current value of the metric.
func (m *int64Metric) Get() int64 {
	return m.get().(int64)
}

// Update adds a data point to the metric.
func (m *int64Metric) Update(v int64) {
	m.update(v)
}

// float64Metric implements Float64Metric.
type float64Metric struct {
	*rawMetric
}

// GetFloat64Metric returns a Float64Metric instance.
func (c *influxClient) GetFloat64Metric(measurement string, tags ...map[string]string) Float64Metric {
	return &float64Metric{
		c.getRawMetric(measurement, tags, float64(0)),
	}
}

// GetFloat64Metric returns a Float64Metric instance using the default client.
func GetFloat64Metric(measurement string, tags ...map[string]string) Float64Metric {
	return defaultClient.GetFloat64Metric(measurement, tags...)
}

// Get returns the current value of the metric.
func (m *float64Metric) Get() float64 {
	return m.get().(float64)
}

// Update adds a data point to the metric.
func (m *float64Metric) Update(v float64) {
	m.update(v)
}
