package promalertsclient

import "github.com/stretchr/testify/mock"

type MockAPIClient struct {
	mock.Mock
}

func NewMockClient() *MockAPIClient {
	return &MockAPIClient{}
}

func (m *MockAPIClient) GetAlerts(filter func(Alert) bool) ([]Alert, error) {
	args := m.Called(filter)
	return args.Get(0).([]Alert), args.Error(1)
}

// Ensure MockAPIClient fulfills APIClient
var _ APIClient = (*MockAPIClient)(nil)
