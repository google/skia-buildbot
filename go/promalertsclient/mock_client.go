package promalertsclient

import (
	"github.com/prometheus/alertmanager/dispatch"
	"github.com/stretchr/testify/mock"
)

type MockAPIClient struct {
	mock.Mock
}

func NewMockClient() *MockAPIClient {
	return &MockAPIClient{}
}

func (m *MockAPIClient) GetAlerts(filter func(dispatch.APIAlert) bool) ([]dispatch.APIAlert, error) {
	args := m.Called(filter)
	return args.Get(0).([]dispatch.APIAlert), args.Error(1)
}
