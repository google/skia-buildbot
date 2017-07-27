package autoscaler

type MockAutoscaler struct {
	StopAllInstancesTimesCalled  int
	StartAllInstancesTimesCalled int
}

func (m *MockAutoscaler) GetRunningInstances() ([]string, error) {
	return []string{"ct-gce-001", "ct-gce-002"}, nil
}

func (m *MockAutoscaler) StopAllInstances() error {
	m.StopAllInstancesTimesCalled += 1
	return nil
}

func (m *MockAutoscaler) StartAllInstances() error {
	m.StartAllInstancesTimesCalled += 1
	return nil
}
