package autoscaler

var (
	TestInstances = []string{"ct-gce-001", "ct-gce-002"}
)

type MockAutoscaler struct {
	StopAllInstancesTimesCalled  int
	StartAllInstancesTimesCalled int
}

func (m *MockAutoscaler) GetInstanceStatuses() map[string]bool {
	return nil
}

func (m *MockAutoscaler) GetOnlineInstances() []string {
	return TestInstances
}

func (m *MockAutoscaler) GetNamesOfManagedInstances() []string {
	return TestInstances
}

func (m *MockAutoscaler) Start([]string) error {
	return nil
}

func (m *MockAutoscaler) StartAllInstances() error {
	m.StartAllInstancesTimesCalled += 1
	return nil
}

func (m *MockAutoscaler) Stop([]string) error {
	return nil
}

func (m *MockAutoscaler) StopAllInstances() error {
	m.StopAllInstancesTimesCalled += 1
	return nil
}

func (m *MockAutoscaler) Update() error {
	return nil
}

var _ IAutoscaler = &MockAutoscaler{}
