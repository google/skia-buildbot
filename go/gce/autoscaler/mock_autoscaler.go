package autoscaler

type MockAutoscaler struct {
	stopAllInstancesTimesCalled  int
	startAllInstancesTimesCalled int
}

func (m *MockAutoscaler) GetRunningInstances() ([]string, error) {
	return []string{"ct-gce-001", "ct-gce-002"}, nil
}

func (m *MockAutoscaler) StopAllInstances() error {
	m.stopAllInstancesTimesCalled += 1
	return nil
}

func (m *MockAutoscaler) StartAllInstances() error {
	m.startAllInstancesTimesCalled += 1
	return nil
}
