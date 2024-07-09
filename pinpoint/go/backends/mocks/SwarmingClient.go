// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	apipb "go.chromium.org/luci/swarming/proto/api_v2"

	context "context"

	mock "github.com/stretchr/testify/mock"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

// SwarmingClient is an autogenerated mock type for the SwarmingClient type
type SwarmingClient struct {
	mock.Mock
}

// CancelTasks provides a mock function with given fields: ctx, taskIDs
func (_m *SwarmingClient) CancelTasks(ctx context.Context, taskIDs []string) error {
	ret := _m.Called(ctx, taskIDs)

	if len(ret) == 0 {
		panic("no return value specified for CancelTasks")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []string) error); ok {
		r0 = rf(ctx, taskIDs)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// FetchFreeBots provides a mock function with given fields: ctx, builder
func (_m *SwarmingClient) FetchFreeBots(ctx context.Context, builder string) ([]*apipb.BotInfo, error) {
	ret := _m.Called(ctx, builder)

	if len(ret) == 0 {
		panic("no return value specified for FetchFreeBots")
	}

	var r0 []*apipb.BotInfo
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]*apipb.BotInfo, error)); ok {
		return rf(ctx, builder)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []*apipb.BotInfo); ok {
		r0 = rf(ctx, builder)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*apipb.BotInfo)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, builder)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBotTasksBetweenTwoTasks provides a mock function with given fields: ctx, botID, taskID1, taskID2
func (_m *SwarmingClient) GetBotTasksBetweenTwoTasks(ctx context.Context, botID string, taskID1 string, taskID2 string) (*apipb.TaskListResponse, error) {
	ret := _m.Called(ctx, botID, taskID1, taskID2)

	if len(ret) == 0 {
		panic("no return value specified for GetBotTasksBetweenTwoTasks")
	}

	var r0 *apipb.TaskListResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) (*apipb.TaskListResponse, error)); ok {
		return rf(ctx, botID, taskID1, taskID2)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) *apipb.TaskListResponse); ok {
		r0 = rf(ctx, botID, taskID1, taskID2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*apipb.TaskListResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, string) error); ok {
		r1 = rf(ctx, botID, taskID1, taskID2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCASOutput provides a mock function with given fields: ctx, taskID
func (_m *SwarmingClient) GetCASOutput(ctx context.Context, taskID string) (*apipb.CASReference, error) {
	ret := _m.Called(ctx, taskID)

	if len(ret) == 0 {
		panic("no return value specified for GetCASOutput")
	}

	var r0 *apipb.CASReference
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*apipb.CASReference, error)); ok {
		return rf(ctx, taskID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *apipb.CASReference); ok {
		r0 = rf(ctx, taskID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*apipb.CASReference)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, taskID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStartTime provides a mock function with given fields: ctx, taskID
func (_m *SwarmingClient) GetStartTime(ctx context.Context, taskID string) (*timestamppb.Timestamp, error) {
	ret := _m.Called(ctx, taskID)

	if len(ret) == 0 {
		panic("no return value specified for GetStartTime")
	}

	var r0 *timestamppb.Timestamp
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*timestamppb.Timestamp, error)); ok {
		return rf(ctx, taskID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *timestamppb.Timestamp); ok {
		r0 = rf(ctx, taskID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*timestamppb.Timestamp)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, taskID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStates provides a mock function with given fields: ctx, taskIDs
func (_m *SwarmingClient) GetStates(ctx context.Context, taskIDs []string) ([]string, error) {
	ret := _m.Called(ctx, taskIDs)

	if len(ret) == 0 {
		panic("no return value specified for GetStates")
	}

	var r0 []string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []string) ([]string, error)); ok {
		return rf(ctx, taskIDs)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []string) []string); ok {
		r0 = rf(ctx, taskIDs)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []string) error); ok {
		r1 = rf(ctx, taskIDs)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStatus provides a mock function with given fields: ctx, taskID
func (_m *SwarmingClient) GetStatus(ctx context.Context, taskID string) (string, error) {
	ret := _m.Called(ctx, taskID)

	if len(ret) == 0 {
		panic("no return value specified for GetStatus")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (string, error)); ok {
		return rf(ctx, taskID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) string); ok {
		r0 = rf(ctx, taskID)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, taskID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListPinpointTasks provides a mock function with given fields: ctx, jobID, buildArtifact
func (_m *SwarmingClient) ListPinpointTasks(ctx context.Context, jobID string, buildArtifact *apipb.CASReference) ([]string, error) {
	ret := _m.Called(ctx, jobID, buildArtifact)

	if len(ret) == 0 {
		panic("no return value specified for ListPinpointTasks")
	}

	var r0 []string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *apipb.CASReference) ([]string, error)); ok {
		return rf(ctx, jobID, buildArtifact)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, *apipb.CASReference) []string); ok {
		r0 = rf(ctx, jobID, buildArtifact)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, *apipb.CASReference) error); ok {
		r1 = rf(ctx, jobID, buildArtifact)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// TriggerTask provides a mock function with given fields: ctx, req
func (_m *SwarmingClient) TriggerTask(ctx context.Context, req *apipb.NewTaskRequest) (*apipb.TaskRequestMetadataResponse, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for TriggerTask")
	}

	var r0 *apipb.TaskRequestMetadataResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *apipb.NewTaskRequest) (*apipb.TaskRequestMetadataResponse, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *apipb.NewTaskRequest) *apipb.TaskRequestMetadataResponse); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*apipb.TaskRequestMetadataResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *apipb.NewTaskRequest) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewSwarmingClient creates a new instance of SwarmingClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewSwarmingClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *SwarmingClient {
	mock := &SwarmingClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
