package ct_autoscaler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gce/autoscaler"
	"go.skia.org/infra/go/swarming/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestRegisterGCETask(t *testing.T) {
	unittest.SmallTest(t)

	mockGCETasksCount := func(ctx context.Context) (int, error) {
		return 1, nil
	}
	mock := &autoscaler.MockAutoscaler{}
	c := CTAutoscaler{a: mock, getGCETasksCount: mockGCETasksCount}

	// Registering the first task should start all instances.
	c.RegisterGCETask("test-task1")
	require.Equal(t, true, c.botsUp)
	require.Equal(t, 1, mock.StartAllInstancesTimesCalled)
	require.Equal(t, 0, mock.StopAllInstancesTimesCalled)

	// Registering the next task should not start all instances.
	c.RegisterGCETask("test-task2")
	require.Equal(t, true, c.botsUp)
	require.Equal(t, 1, mock.StartAllInstancesTimesCalled)
	require.Equal(t, 0, mock.StopAllInstancesTimesCalled)
}

func TestUnRegisterGCETask(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	mockTasksCount := 0
	mockGCETasksCount := func(ctx context.Context) (int, error) {
		return mockTasksCount, nil
	}
	mock := &autoscaler.MockAutoscaler{}
	s := &mocks.ApiClient{}
	s.On("DeleteBots", testutils.AnyContext, autoscaler.TestInstances).Return(nil)
	defer s.AssertExpectations(t)
	c := CTAutoscaler{a: mock, s: s, getGCETasksCount: mockGCETasksCount}

	// Register 2 tasks.
	c.RegisterGCETask("test-task1")
	c.RegisterGCETask("test-task2")
	mockTasksCount += 2
	require.Equal(t, true, c.botsUp)
	require.Equal(t, 1, mock.StartAllInstancesTimesCalled)
	s.AssertNumberOfCalls(t, "DeleteBots", 0)

	// Unregistering the 1st task should not stop all instances.
	mockTasksCount -= 1
	require.Nil(t, c.maybeScaleDown(ctx))
	s.AssertNumberOfCalls(t, "DeleteBots", 0)
	require.Equal(t, true, c.botsUp)
	require.Equal(t, 1, mock.StartAllInstancesTimesCalled)
	require.Equal(t, 0, mock.StopAllInstancesTimesCalled)

	// Unregistering the 2nd task should stop all instances.
	s.On("DeleteBots", testutils.AnyContext, autoscaler.TestInstances).Return(nil)
	mockTasksCount -= 1
	require.Nil(t, c.maybeScaleDown(ctx))
	s.AssertNumberOfCalls(t, "DeleteBots", 1)
	require.Equal(t, false, c.botsUp)
	require.Equal(t, 1, mock.StartAllInstancesTimesCalled)
	require.Equal(t, 1, mock.StopAllInstancesTimesCalled)

	// Registering and then unregistering a 3rd task should start and stop all
	// instances.
	mockTasksCount += 1
	c.RegisterGCETask("test-task3")
	s.AssertNumberOfCalls(t, "DeleteBots", 1)
	require.Equal(t, true, c.botsUp)
	require.Equal(t, 2, mock.StartAllInstancesTimesCalled)
	require.Equal(t, 1, mock.StopAllInstancesTimesCalled)

	mockTasksCount -= 1
	require.Nil(t, c.maybeScaleDown(ctx))
	s.AssertNumberOfCalls(t, "DeleteBots", 2)
	require.Equal(t, false, c.botsUp)
	require.Equal(t, 2, mock.StartAllInstancesTimesCalled)
	require.Equal(t, 2, mock.StopAllInstancesTimesCalled)
}
