package perfclient

import (
	"time"

	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/perf/go/ingest/format"
)

type MockPerfClient struct {
	mock.Mock
}

// NewMockPerfClient returns a pointer to a newly created struct.  We return the pointer because we want to
// make sure the methods on mock.Mock stay accessible, e.g. m.On()
func NewMockPerfClient() *MockPerfClient {
	return &MockPerfClient{}
}

func (m *MockPerfClient) PushToPerf(now time.Time, folderName, filePrefix string, data format.BenchData) error {
	args := m.Called(now, folderName, filePrefix, data)
	return args.Error(0)
}

// Ensure MockPerfClient fulfills ClientInterface
var _ ClientInterface = (*MockPerfClient)(nil)
