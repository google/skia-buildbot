package ct_autoscaler

type MockCTAutoscaler struct {
	RegisterGCETaskTimesCalled   int
	UnregisterGCETaskTimesCalled int
}

func (m *MockCTAutoscaler) RegisterGCETask(taskId string) error {
	m.RegisterGCETaskTimesCalled += 1
	return nil
}

func (m *MockCTAutoscaler) UnregisterGCETask(taskId string) error {
	m.UnregisterGCETaskTimesCalled += 1
	return nil
}
