// Code generated by mockery v1.0.0 (and then touched up by hand)
package swarming

import mock "github.com/stretchr/testify/mock"

import time "time"
import v1 "github.com/luci/luci-go/common/api/swarming/swarming/v1"

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

// CancelTask provides a mock function with given fields: id
func (_m *MockApiClient) CancelTask(id string) error {
	ret := _m.Called(id)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetStdoutOfTask provides a mock function with given fields: id
func (_m *MockApiClient) GetStdoutOfTask(id string) (*v1.SwarmingRpcsTaskOutput, error) {
	ret := _m.Called(id)

	var r0 *v1.SwarmingRpcsTaskOutput
	if rf, ok := ret.Get(0).(func(string) *v1.SwarmingRpcsTaskOutput); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.SwarmingRpcsTaskOutput)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTask provides a mock function with given fields: id, includePerformanceStats
func (_m *MockApiClient) GetTask(id string, includePerformanceStats bool) (*v1.SwarmingRpcsTaskResult, error) {
	ret := _m.Called(id, includePerformanceStats)

	var r0 *v1.SwarmingRpcsTaskResult
	if rf, ok := ret.Get(0).(func(string, bool) *v1.SwarmingRpcsTaskResult); ok {
		r0 = rf(id, includePerformanceStats)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.SwarmingRpcsTaskResult)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, bool) error); ok {
		r1 = rf(id, includePerformanceStats)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTaskMetadata provides a mock function with given fields: id
func (_m *MockApiClient) GetTaskMetadata(id string) (*v1.SwarmingRpcsTaskRequestMetadata, error) {
	ret := _m.Called(id)

	var r0 *v1.SwarmingRpcsTaskRequestMetadata
	if rf, ok := ret.Get(0).(func(string) *v1.SwarmingRpcsTaskRequestMetadata); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.SwarmingRpcsTaskRequestMetadata)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GracefullyShutdownBot provides a mock function with given fields: id
func (_m *MockApiClient) GracefullyShutdownBot(id string) (*v1.SwarmingRpcsTerminateResponse, error) {
	ret := _m.Called(id)

	var r0 *v1.SwarmingRpcsTerminateResponse
	if rf, ok := ret.Get(0).(func(string) *v1.SwarmingRpcsTerminateResponse); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.SwarmingRpcsTerminateResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListBots provides a mock function with given fields: dimensions
func (_m *MockApiClient) ListBots(dimensions map[string]string) ([]*v1.SwarmingRpcsBotInfo, error) {
	ret := _m.Called(dimensions)

	var r0 []*v1.SwarmingRpcsBotInfo
	if rf, ok := ret.Get(0).(func(map[string]string) []*v1.SwarmingRpcsBotInfo); ok {
		r0 = rf(dimensions)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1.SwarmingRpcsBotInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(map[string]string) error); ok {
		r1 = rf(dimensions)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListBotsForPool provides a mock function with given fields: pool
func (_m *MockApiClient) ListBotsForPool(pool string) ([]*v1.SwarmingRpcsBotInfo, error) {
	ret := _m.Called(pool)

	var r0 []*v1.SwarmingRpcsBotInfo
	if rf, ok := ret.Get(0).(func(string) []*v1.SwarmingRpcsBotInfo); ok {
		r0 = rf(pool)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1.SwarmingRpcsBotInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(pool)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListDownBots provides a mock function with given fields: pool
func (_m *MockApiClient) ListDownBots(pool string) ([]*v1.SwarmingRpcsBotInfo, error) {
	ret := _m.Called(pool)

	var r0 []*v1.SwarmingRpcsBotInfo
	if rf, ok := ret.Get(0).(func(string) []*v1.SwarmingRpcsBotInfo); ok {
		r0 = rf(pool)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1.SwarmingRpcsBotInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(pool)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListFreeBots provides a mock function with given fields: pool
func (_m *MockApiClient) ListFreeBots(pool string) ([]*v1.SwarmingRpcsBotInfo, error) {
	ret := _m.Called(pool)

	var r0 []*v1.SwarmingRpcsBotInfo
	if rf, ok := ret.Get(0).(func(string) []*v1.SwarmingRpcsBotInfo); ok {
		r0 = rf(pool)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1.SwarmingRpcsBotInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(pool)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListSkiaTasks provides a mock function with given fields: start, end
func (_m *MockApiClient) ListSkiaTasks(start time.Time, end time.Time) ([]*v1.SwarmingRpcsTaskRequestMetadata, error) {
	ret := _m.Called(start, end)

	var r0 []*v1.SwarmingRpcsTaskRequestMetadata
	if rf, ok := ret.Get(0).(func(time.Time, time.Time) []*v1.SwarmingRpcsTaskRequestMetadata); ok {
		r0 = rf(start, end)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1.SwarmingRpcsTaskRequestMetadata)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(time.Time, time.Time) error); ok {
		r1 = rf(start, end)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListTaskResults provides a mock function with given fields: start, end, tags, state, includePerformanceStats
func (_m *MockApiClient) ListTaskResults(start time.Time, end time.Time, tags []string, state string, includePerformanceStats bool) ([]*v1.SwarmingRpcsTaskResult, error) {
	ret := _m.Called(start, end, tags, state, includePerformanceStats)

	var r0 []*v1.SwarmingRpcsTaskResult
	if rf, ok := ret.Get(0).(func(time.Time, time.Time, []string, string, bool) []*v1.SwarmingRpcsTaskResult); ok {
		r0 = rf(start, end, tags, state, includePerformanceStats)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1.SwarmingRpcsTaskResult)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(time.Time, time.Time, []string, string, bool) error); ok {
		r1 = rf(start, end, tags, state, includePerformanceStats)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListTasks provides a mock function with given fields: start, end, tags, state
func (_m *MockApiClient) ListTasks(start time.Time, end time.Time, tags []string, state string) ([]*v1.SwarmingRpcsTaskRequestMetadata, error) {
	ret := _m.Called(start, end, tags, state)

	var r0 []*v1.SwarmingRpcsTaskRequestMetadata
	if rf, ok := ret.Get(0).(func(time.Time, time.Time, []string, string) []*v1.SwarmingRpcsTaskRequestMetadata); ok {
		r0 = rf(start, end, tags, state)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1.SwarmingRpcsTaskRequestMetadata)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(time.Time, time.Time, []string, string) error); ok {
		r1 = rf(start, end, tags, state)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RetryTask provides a mock function with given fields: t
func (_m *MockApiClient) RetryTask(t *v1.SwarmingRpcsTaskRequestMetadata) (*v1.SwarmingRpcsTaskRequestMetadata, error) {
	ret := _m.Called(t)

	var r0 *v1.SwarmingRpcsTaskRequestMetadata
	if rf, ok := ret.Get(0).(func(*v1.SwarmingRpcsTaskRequestMetadata) *v1.SwarmingRpcsTaskRequestMetadata); ok {
		r0 = rf(t)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.SwarmingRpcsTaskRequestMetadata)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*v1.SwarmingRpcsTaskRequestMetadata) error); ok {
		r1 = rf(t)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SwarmingService provides a mock function with given fields:
func (_m *MockApiClient) SwarmingService() *v1.Service {
	ret := _m.Called()

	var r0 *v1.Service
	if rf, ok := ret.Get(0).(func() *v1.Service); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.Service)
		}
	}

	return r0
}

// TriggerTask provides a mock function with given fields: t
func (_m *MockApiClient) TriggerTask(t *v1.SwarmingRpcsNewTaskRequest) (*v1.SwarmingRpcsTaskRequestMetadata, error) {
	ret := _m.Called(t)

	var r0 *v1.SwarmingRpcsTaskRequestMetadata
	if rf, ok := ret.Get(0).(func(*v1.SwarmingRpcsNewTaskRequest) *v1.SwarmingRpcsTaskRequestMetadata); ok {
		r0 = rf(t)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.SwarmingRpcsTaskRequestMetadata)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*v1.SwarmingRpcsNewTaskRequest) error); ok {
		r1 = rf(t)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Make sure MockApiClient fulfills ApiClient
var _ ApiClient = (*MockApiClient)(nil)
