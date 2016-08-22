package db

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
)

func TestModifiedTasks(t *testing.T) {
	m := ModifiedTasks{}

	_, err := m.GetModifiedTasks("dummy-id")
	assert.True(t, IsUnknownId(err))

	id, err := m.StartTrackingModifiedTasks()
	assert.NoError(t, err)

	tasks, err := m.GetModifiedTasks(id)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))

	t1 := makeTask(time.Unix(0, 1470674132000000), []string{"a", "b", "c", "d"})
	t1.Id = "1"

	// Insert the task.
	m.TrackModifiedTask(t1)

	// Ensure that the task shows up in the modified list.
	tasks, err = m.GetModifiedTasks(id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1}, tasks)

	// Insert two more tasks.
	t2 := makeTask(time.Unix(0, 1470674376000000), []string{"e", "f"})
	t2.Id = "2"
	m.TrackModifiedTask(t2)
	t3 := makeTask(time.Unix(0, 1470674884000000), []string{"g", "h"})
	t3.Id = "3"
	m.TrackModifiedTask(t3)

	// Ensure that both tasks show up in the modified list.
	tasks, err = m.GetModifiedTasks(id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t2, t3}, tasks)
}

// Test that if a Task is modified multiple times, it only appears once in the
// result of GetModifiedTasks.
func TestMultipleModifications(t *testing.T) {
	m := ModifiedTasks{}

	id, err := m.StartTrackingModifiedTasks()
	assert.NoError(t, err)

	t1 := makeTask(time.Unix(0, 1470674132000000), []string{"a", "b", "c", "d"})
	t1.Id = "1"

	// Insert the task.
	m.TrackModifiedTask(t1)

	// Make several more modifications.
	t1.Status = TASK_STATUS_RUNNING
	m.TrackModifiedTask(t1)
	t1.Status = TASK_STATUS_SUCCESS
	m.TrackModifiedTask(t1)

	// Ensure that the task shows up only once in the modified list.
	tasks, err := m.GetModifiedTasks(id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1}, tasks)
}

func TestModifiedTasksTooManyUsers(t *testing.T) {
	m := ModifiedTasks{}

	// Max out the number of modified-tasks users; ensure that we error out.
	for i := 0; i < MAX_MODIFIED_BUILDS_USERS; i++ {
		_, err := m.StartTrackingModifiedTasks()
		assert.NoError(t, err)
	}
	_, err := m.StartTrackingModifiedTasks()
	assert.True(t, IsTooManyUsers(err))
}
