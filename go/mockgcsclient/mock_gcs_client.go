package mockgcsclient

import (
	"io"

	"go.skia.org/infra/go/gs"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/context"
)

// MockGCSClient
type MockGCSClient struct {
	mock.Mock
}

func New() *MockGCSClient {
	return new(MockGCSClient)
}

func (m *MockGCSClient) GetFileContents(path string) ([]byte, error) {
	args := m.Called(path)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockGCSClient) GetFileContentsCTX(path string, ctx context.Context) ([]byte, error) {
	args := m.Called(path, ctx)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockGCSClient) GetFileWriter(path string, options gs.FileWriteOptions) io.WriteCloser {
	args := m.Called(path, options)
	return args.Get(0).(io.WriteCloser)
}

func (m *MockGCSClient) GetFileWriterCTX(path string, options gs.FileWriteOptions, ctx context.Context) io.WriteCloser {
	args := m.Called(path, options, ctx)
	return args.Get(0).(io.WriteCloser)
}

func (m *MockGCSClient) ExecuteOnAllFilesInFolder(folder string, callback func(item *storage.ObjectAttrs)) error {
	args := m.Called(folder, callback)
	return args.Error(0)
}

func (m *MockGCSClient) ExecuteOnAllFilesInFolderCTX(folder string, callback func(item *storage.ObjectAttrs), ctx context.Context) error {
	args := m.Called(folder, callback, ctx)
	return args.Error(0)
}
