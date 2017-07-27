package ct_autoscaler

import (
	"testing"

	"go.skia.org/infra/go/gce/autoscaler"
	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
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
	c := CTAutoscaler{a: mock}

	// Register 2 tasks.
	assert.Nil(t, c.RegisterGCETask("test-task1"))
	assert.Nil(t, c.RegisterGCETask("test-task2"))
	assert.Equal(t, 2, c.activeGCETasks)
	assert.Equal(t, 1, mock.StartAllInstancesTimesCalled)

	// Unregistering the 1st task should not stop all instances.
	assert.Nil(t, c.UnregisterGCETask("test-task1"))
	assert.Equal(t, 1, c.activeGCETasks)
	assert.Equal(t, 1, mock.StartAllInstancesTimesCalled)
	assert.Equal(t, 0, mock.StopAllInstancesTimesCalled)

	// Unregistering the 2nd task should stop all instances.
	assert.Nil(t, c.UnregisterGCETask("test-task2"))
	assert.Equal(t, 0, c.activeGCETasks)
	assert.Equal(t, 1, mock.StartAllInstancesTimesCalled)
	assert.Equal(t, 1, mock.StopAllInstancesTimesCalled)

	// Registering and then unregistering a 3rd task should start and stop all
	// instances.
	assert.Nil(t, c.RegisterGCETask("test-task3"))
	assert.Equal(t, 1, c.activeGCETasks)
	assert.Equal(t, 2, mock.StartAllInstancesTimesCalled)
	assert.Equal(t, 1, mock.StopAllInstancesTimesCalled)
	assert.Nil(t, c.UnregisterGCETask("test-task3"))
	assert.Equal(t, 0, c.activeGCETasks)
	assert.Equal(t, 2, mock.StartAllInstancesTimesCalled)
	assert.Equal(t, 2, mock.StopAllInstancesTimesCalled)
}
