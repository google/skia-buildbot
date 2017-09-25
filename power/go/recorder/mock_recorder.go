package recorder

import (
	"github.com/stretchr/testify/mock"
)

type MockRecorder struct {
	mock.Mock
}

// NewMockRecorder returns a pointer to a newly created struct.  We return the
// pointer to the struct instead of the Recorder interface because we want to
// make sure the methods on mock.Mock stay accessible, e.g. m.On()
func NewMockRecorder() *MockRecorder {
	return &MockRecorder{}
}

func (m *MockRecorder) NewlyDownBots(bots []string) {
	m.Called(bots)
}

func (m *MockRecorder) NewlyFixedBots(bots []string) {
	m.Called(bots)
}

// Ensure MockRecorder fulfills Recorder
var _ Recorder = (*MockRecorder)(nil)
