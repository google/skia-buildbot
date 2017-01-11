package tests

import (
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/mockgcsclient"
)

type MockGCSClient struct {
	mockgcsclient.MockGCSClient
}

func NewMockGCSClient() *MockGCSClient {
	return new(MockGCSClient)
}

func (m *MockGCSClient) SetFileContents(path string, opts gs.FileWriteOptions, contents []byte) error {
	args := m.Called(path, opts, contents)
	return args.Error(0)
}
