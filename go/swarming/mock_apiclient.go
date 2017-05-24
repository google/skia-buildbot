package swarming

import (
	"time"

	swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/stretchr/testify/mock"
)

// MockCommonImpl is a mock of swarming.ApiClient. All the methods are mocked using
// testify's mocking library.
type MockApiClient struct {
	mock.Mock
}

// NewAPIClient returns a pointer to a newly created struct.  We return the pointer because we want to
// make sure the methods on mock.Mock stay accessible, e.g. m.On()
func NewMockApiClient() *MockApiClient {
	return &MockApiClient{}
}

// SwarmingService is just here to fully implement the interface - it should not be
// used in unit tests.
func (m *MockApiClient) SwarmingService() *swarming.Service {
	return nil
}

func (m *MockApiClient) ListBots(dimensions map[string]string) ([]*swarming.SwarmingRpcsBotInfo, error) {
	args := m.Called(dimensions)
	return args.Get(0).([]*swarming.SwarmingRpcsBotInfo), args.Error(1)
}

func (m *MockApiClient) ListFreeBots(pool string) ([]*swarming.SwarmingRpcsBotInfo, error) {
	args := m.Called(pool)
	return args.Get(0).([]*swarming.SwarmingRpcsBotInfo), args.Error(1)
}

func (m *MockApiClient) ListDownBots(pool string) ([]*swarming.SwarmingRpcsBotInfo, error) {
	args := m.Called(pool)
	return args.Get(0).([]*swarming.SwarmingRpcsBotInfo), args.Error(1)
}

func (m *MockApiClient) ListBotsForPool(pool string) ([]*swarming.SwarmingRpcsBotInfo, error) {
	args := m.Called(pool)
	return args.Get(0).([]*swarming.SwarmingRpcsBotInfo), args.Error(1)
}

func (m *MockApiClient) GetStdoutOfTask(id string) (*swarming.SwarmingRpcsTaskOutput, error) {
	return nil, nil
}

func (m *MockApiClient) GracefullyShutdownBot(id string) (*swarming.SwarmingRpcsTerminateResponse, error) {
	return nil, nil
}

func (m *MockApiClient) ListTasks(start, end time.Time, tags []string, state string) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return nil, nil
}

func (m *MockApiClient) ListSkiaTasks(start, end time.Time) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return nil, nil
}

func (m *MockApiClient) ListTaskResults(start, end time.Time, tags []string, state string, includePerformanceStats bool) ([]*swarming.SwarmingRpcsTaskResult, error) {
	return nil, nil
}

func (m *MockApiClient) CancelTask(id string) error {
	return nil
}

func (m *MockApiClient) TriggerTask(t *swarming.SwarmingRpcsNewTaskRequest) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return nil, nil
}

func (m *MockApiClient) RetryTask(t *swarming.SwarmingRpcsTaskRequestMetadata) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return nil, nil
}

func (m *MockApiClient) GetTask(id string, includePerformanceStats bool) (*swarming.SwarmingRpcsTaskResult, error) {
	return nil, nil
}

func (m *MockApiClient) GetTaskMetadata(id string) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return nil, nil
}

// Make sure MockCommonImpl fulfills common.CommonImpl
var _ ApiClient = (*MockApiClient)(nil)
