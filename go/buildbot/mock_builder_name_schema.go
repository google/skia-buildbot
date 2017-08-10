package buildbot

import "github.com/stretchr/testify/mock"

type MockBuilderNameParser struct {
	mock.Mock
}

// NewMockBuilderNameParser returns a pointer to a newly created struct.  We return the pointer because we want to
// make sure the methods on mock.Mock stay accessible, e.g. m.On()
func NewMockBuilderNameParser() *MockBuilderNameParser {
	return &MockBuilderNameParser{}
}

func (m *MockBuilderNameParser) ParseBuilderName(name string) (map[string]string, error) {
	args := m.Called(name)
	return args.Get(0).(map[string]string), args.Error(1)
}

// Ensure MockBuilderNameParser fulfills BuilderNameParser
var _ BuilderNameParser = (*MockBuilderNameParser)(nil)
