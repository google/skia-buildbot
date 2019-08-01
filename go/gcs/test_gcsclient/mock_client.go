package test_gcsclient

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/gcs"
)

// MockGCSClient is a mock of gcs.GCSClient. All the methods are mocked using testify's mocking
// library. See the README in this directory for some example mocks.
// This struct can be embedded to extend to instance-specific GCS functions. See
// fuzzer for an example.
type MockGCSClient struct {
	mock.Mock
}

// New returns a pointer to a newly created struct.  We return the pointer because we want to
// make sure the methods on mock.Mock stay accessible, e.g. m.On()
func NewMockClient() *MockGCSClient {
	return &MockGCSClient{}
}
func (m *MockGCSClient) FileReader(ctx context.Context, path string) (io.ReadCloser, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockGCSClient) FileWriter(ctx context.Context, path string, opts gcs.FileWriteOptions) io.WriteCloser {
	args := m.Called(ctx, path, opts)
	return args.Get(0).(io.WriteCloser)
}

func (m *MockGCSClient) DoesFileExist(ctx context.Context, path string) (bool, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(bool), args.Error(1)
}

func (m *MockGCSClient) GetFileContents(ctx context.Context, path string) ([]byte, error) {
	args := m.Called(ctx, path)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockGCSClient) SetFileContents(ctx context.Context, path string, opts gcs.FileWriteOptions, contents []byte) error {
	args := m.Called(ctx, path, opts, contents)
	return args.Error(0)
}

func (m *MockGCSClient) GetFileObjectAttrs(ctx context.Context, path string) (*storage.ObjectAttrs, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(*storage.ObjectAttrs), args.Error(1)
}

func (m *MockGCSClient) AllFilesInDirectory(ctx context.Context, folder string, callback func(item *storage.ObjectAttrs)) error {
	args := m.Called(ctx, folder, callback)
	return args.Error(0)
}

func (m *MockGCSClient) DeleteFile(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockGCSClient) Bucket() string {
	args := m.Called()
	return args.String(0)
}

// Make sure MockGCSClient fulfills gcs.GCSClient
var _ gcs.GCSClient = (*MockGCSClient)(nil)
