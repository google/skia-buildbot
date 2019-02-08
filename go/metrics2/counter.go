package metrics2

// GetCounter creates and returns a new Counter using the default client.
func GetCounter(name string, tags ...map[string]string) Counter {
	return defaultClient.GetCounter(name, tags...)
}
