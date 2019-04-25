package tests

import (
	"go.skia.org/infra/fuzzer/go/storage"
	"go.skia.org/infra/go/gcs/test_gcsclient"
)

type MockGCSClient struct {
	test_gcsclient.MockGCSClient
}

func NewMockGCSClient() *MockGCSClient {
	return &MockGCSClient{}
}

func (m *MockGCSClient) GetAllFuzzNamesInFolder(name string) ([]string, error) {
	args := m.Called(name)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockGCSClient) DeleteAllFilesInFolder(folder string, processes int) error {
	args := m.Called(folder, processes)
	return args.Error(0)
}

func (m *MockGCSClient) DownloadAllFuzzes(downloadToPath, category, revision, architecture, fuzzType string, processes int) ([]string, error) {
	args := m.Called(downloadToPath, category, revision, architecture, fuzzType, processes)
	return args.Get(0).([]string), args.Error(1)
}

// Make sure MockGCSClient fulfills FuzzerGCSClient
var _ storage.FuzzerGCSClient = (*MockGCSClient)(nil)
