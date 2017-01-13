package tests

import (
	"go.skia.org/infra/fuzzer/go/storage"
	"go.skia.org/infra/go/mockgcsclient"
)

type MockGCSClient struct {
	mockgcsclient.MockGCSClient
}

func NewMockGCSClient() *MockGCSClient {
	return &MockGCSClient{}
}

func (m *MockGCSClient) GetAllFuzzNamesInFolder(name string) ([]string, error) {
	args := m.Called(name)
	return args.Get(0).([]string), args.Error(1)
}

// Make sure MockGCSClient fulfills gs.GCSClient
var _ storage.FuzzerGCSClient = (*MockGCSClient)(nil)
