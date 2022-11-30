package metrics2

// GetInt64Metric returns an Int64Metric instance using the default client.
func GetInt64Metric(measurement string, tags ...map[string]string) Int64Metric {
	return defaultClient.GetInt64Metric(measurement, tags...)
}

// GetFloat64Metric returns a Float64Metric instance using the default client.
func GetFloat64Metric(measurement string, tags ...map[string]string) Float64Metric {
	return defaultClient.GetFloat64Metric(measurement, tags...)
}

// GetFloat64SummaryMetric returns a Float64SummaryMetric instance using the default client.
func GetFloat64SummaryMetric(measurement string, tags ...map[string]string) Float64SummaryMetric {
	return defaultClient.GetFloat64SummaryMetric(measurement, tags...)
}

// GetBoolMetric returns a BoolMetric instance using the default client.
func GetBoolMetric(measurement string, tags ...map[string]string) BoolMetric {
	return defaultClient.GetBoolMetric(measurement, tags...)
}
