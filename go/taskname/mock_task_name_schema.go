package taskname

import "github.com/stretchr/testify/mock"

type MockTaskNameParser struct {
	mock.Mock
}

// NewMockTaskNameParser returns a pointer to a newly created struct.  We return the pointer because we want to
// make sure the methods on mock.Mock stay accessible, e.g. m.On()
func NewMockTaskNameParser() *MockTaskNameParser {
	return &MockTaskNameParser{}
}

func (m *MockTaskNameParser) ParseTaskName(name string) (map[string]string, error) {
	args := m.Called(name)
	return args.Get(0).(map[string]string), args.Error(1)
}

// Ensure MockTaskNameParser fulfills TaskNameParser
var _ TaskNameParser = (*MockTaskNameParser)(nil)
