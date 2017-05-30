package promalertsclient

import "github.com/stretchr/testify/mock"

type MockAPIClient struct {
	mock.Mock
}

// NewMockClient returns a pointer to a newly created struct.  We return the pointer because we want to
// make sure the methods on mock.Mock stay accessible, e.g. m.On()
func NewMockClient() *MockAPIClient {
	return &MockAPIClient{}
}

func (m *MockAPIClient) GetAlerts(filter func(Alert) bool) ([]Alert, error) {
	args := m.Called(filter)
	return args.Get(0).([]Alert), args.Error(1)
}

// Ensure MockAPIClient fulfills APIClient
var _ APIClient = (*MockAPIClient)(nil)
