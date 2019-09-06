package ct_autoscaler

type MockCTAutoscaler struct {
	RegisterGCETaskTimesCalled int
}

func (m *MockCTAutoscaler) RegisterGCETask(taskId string) {
	m.RegisterGCETaskTimesCalled += 1
}
