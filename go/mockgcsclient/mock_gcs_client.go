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

// New returns a pointer to the struct because we want to make sure the methods on mock.Mock
// stay accessible, e.g. m.On()
func New() *MockGCSClient {
	return new(MockGCSClient)
}

func (m *MockGCSClient) GetFileContents(ctx context.Context, path string) ([]byte, error) {
	args := m.Called(ctx, path)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockGCSClient) GetFileWriter(ctx context.Context, path string, options gs.FileWriteOptions) io.WriteCloser {
	args := m.Called(ctx, path, options)
	return args.Get(0).(io.WriteCloser)
}

func (m *MockGCSClient) ExecuteOnAllFilesInFolder(ctx context.Context, folder string, callback func(item *storage.ObjectAttrs)) error {
	args := m.Called(ctx, folder, callback)
	return args.Error(0)
}

// Make sure MockGCSClient fulfils gs.GCSClient
var _ gs.GCSClient = (*MockGCSClient)(nil)
