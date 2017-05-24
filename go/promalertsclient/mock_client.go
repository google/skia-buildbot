package promalertsclient

import (
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/mock"
)

type MockAPIClient struct {
	mock.Mock
}

func NewMockClient() *MockAPIClient {
	return &MockAPIClient{}
}

func (m *MockAPIClient) GetAlerts(filter func(model.Alert) bool) ([]model.Alert, error) {
	args := m.Called(filter)
	return args.Get(0).([]model.Alert), args.Error(1)
}

// Ensure MockAPIClient fulfils APIClient
var _ APIClient = (*MockAPIClient)(nil)
