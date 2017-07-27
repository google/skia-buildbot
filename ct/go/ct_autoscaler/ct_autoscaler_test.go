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
	assert.Equal(t, 1, mock.startAllInstancesTimesCalled)
	assert.Equal(t, 0, mock.stopAllInstancesTimesCalled)

	// Registering the next task should not start all instances.
	assert.Nil(t, c.RegisterGCETask("test-task2"))
	assert.Equal(t, 2, c.activeGCETasks)
	assert.Equal(t, 1, mock.startAllInstancesTimesCalled)
	assert.Equal(t, 0, mock.stopAllInstancesTimesCalled)
}

func TestUnRegisterGCETask(t *testing.T) {
	testutils.SmallTest(t)
	mock := &autoscaler.MockAutoscaler{}
	c := CTAutoscaler{a: mock}

	// Register 2 tasks.
	assert.Nil(t, c.RegisterGCETask("test-task1"))
	assert.Nil(t, c.RegisterGCETask("test-task2"))
	assert.Equal(t, 2, c.activeGCETasks)
	assert.Equal(t, 1, mock.startAllInstancesTimesCalled)

	// Unregister the 1st task.
	assert.Nil(t, c.UnregisterGCETask("test-task1"))
	assert.Equal(t, 1, c.activeGCETasks)
	assert.Equal(t, 1, mock.startAllInstancesTimesCalled)
	assert.Equal(t, 0, mock.stopAllInstancesTimesCalled)

	// Unregister the 2nd task.
	assert.Nil(t, c.UnregisterGCETask("test-task2"))
	assert.Equal(t, 0, c.activeGCETasks)
	assert.Equal(t, 1, mock.startAllInstancesTimesCalled)
	assert.Equal(t, 1, mock.stopAllInstancesTimesCalled)

	// Register and unregister a 3rd task.
	assert.Nil(t, c.RegisterGCETask("test-task3"))
	assert.Equal(t, 1, c.activeGCETasks)
	assert.Equal(t, 2, mock.startAllInstancesTimesCalled)
	assert.Equal(t, 1, mock.stopAllInstancesTimesCalled)
	assert.Nil(t, c.UnregisterGCETask("test-task3"))
	assert.Equal(t, 0, c.activeGCETasks)
	assert.Equal(t, 2, mock.startAllInstancesTimesCalled)
	assert.Equal(t, 2, mock.stopAllInstancesTimesCalled)
}
