package testutils

import "github.com/stretchr/testify/mock"

// MockGCSClient
type MockGCSClient struct {
	mock.Mock
}

func NewMockGCSClient() *MockGCSClient {
	return new(MockGCSClient)
}

func (m *MockGCSClient) GetFileContents(path string) ([]byte, error) {
	args := m.Called(path)
	return args.Get(0).([]byte), args.Error(1)
}
