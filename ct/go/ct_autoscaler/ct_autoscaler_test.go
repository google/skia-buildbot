package ct_autoscaler

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gce/autoscaler"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"
)

func TestRegisterGCETask(t *testing.T) {
	testutils.SmallTest(t)
	mock := &autoscaler.MockAutoscaler{}
	c := CTAutoscaler{a: mock}

	// Registering the first task should start all instances.
	assert.Nil(t, c.RegisterGCETask("test-task1"))
	assert.Equal(t, 1, c.activeGCETasks)
	assert.Equal(t, 1, mock.StartAllInstancesTimesCalled)
	assert.Equal(t, 0, mock.StopAllInstancesTimesCalled)

	// Registering the next task should not start all instances.
	assert.Nil(t, c.RegisterGCETask("test-task2"))
	assert.Equal(t, 2, c.activeGCETasks)
	assert.Equal(t, 1, mock.StartAllInstancesTimesCalled)
	assert.Equal(t, 0, mock.StopAllInstancesTimesCalled)
}

func TestUnRegisterGCETask(t *testing.T) {
	testutils.SmallTest(t)
	mock := &autoscaler.MockAutoscaler{}
	s := swarming.NewMockApiClient()
	s.On("DeleteBots", autoscaler.TestInstances).Return(nil)
	defer s.AssertExpectations(t)
	c := CTAutoscaler{a: mock, s: s}

	// Register 2 tasks.
	assert.Nil(t, c.RegisterGCETask("test-task1"))
	assert.Nil(t, c.RegisterGCETask("test-task2"))
	assert.Equal(t, 2, c.activeGCETasks)
	assert.Equal(t, 1, mock.StartAllInstancesTimesCalled)
	s.AssertNumberOfCalls(t, "DeleteBots", 0)

	// Unregistering the 1st task should not stop all instances.
	assert.Nil(t, c.UnregisterGCETask("test-task1"))
	s.AssertNumberOfCalls(t, "DeleteBots", 0)
	assert.Equal(t, 1, c.activeGCETasks)
	assert.Equal(t, 1, mock.StartAllInstancesTimesCalled)
	assert.Equal(t, 0, mock.StopAllInstancesTimesCalled)

	// Unregistering the 2nd task should stop all instances.
	s.On("DeleteBots", autoscaler.TestInstances).Return(nil)
	assert.Nil(t, c.UnregisterGCETask("test-task2"))
	s.AssertNumberOfCalls(t, "DeleteBots", 1)
	assert.Equal(t, 0, c.activeGCETasks)
	assert.Equal(t, 1, mock.StartAllInstancesTimesCalled)
	assert.Equal(t, 1, mock.StopAllInstancesTimesCalled)

	// Registering and then unregistering a 3rd task should start and stop all
	// instances.
	assert.Nil(t, c.RegisterGCETask("test-task3"))
	s.AssertNumberOfCalls(t, "DeleteBots", 1)
	assert.Equal(t, 1, c.activeGCETasks)
	assert.Equal(t, 2, mock.StartAllInstancesTimesCalled)
	assert.Equal(t, 1, mock.StopAllInstancesTimesCalled)

	assert.Nil(t, c.UnregisterGCETask("test-task3"))
	s.AssertNumberOfCalls(t, "DeleteBots", 2)
	assert.Equal(t, 0, c.activeGCETasks)
	assert.Equal(t, 2, mock.StartAllInstancesTimesCalled)
	assert.Equal(t, 2, mock.StopAllInstancesTimesCalled)
}
