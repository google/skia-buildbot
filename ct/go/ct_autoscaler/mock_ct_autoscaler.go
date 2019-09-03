package ct_autoscaler

type MockCTAutoscaler struct {
	RegisterGCETaskTimesCalled int
}

func (m *MockCTAutoscaler) RegisterGCETask(taskId string) error {
	m.RegisterGCETaskTimesCalled += 1
	return nil
}
