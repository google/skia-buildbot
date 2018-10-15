package db

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
)

func TestModifiedTasks(t *testing.T) {
	testutils.SmallTest(t)
	m := ModifiedTasks{}

	_, err := m.GetModifiedTasks("dummy-id")
	assert.True(t, IsUnknownId(err))

	id, err := m.StartTrackingModifiedTasks()
	assert.NoError(t, err)

	tasks, err := m.GetModifiedTasks(id)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))

	t1 := MakeTestTask(time.Unix(0, 1470674132000000), []string{"a", "b", "c", "d"})
	t1.Id = "1"

	// Insert the task.
	m.TrackModifiedTask(t1)

	// Ensure that the task shows up in the modified list.
	tasks, err = m.GetModifiedTasks(id)
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, []*Task{t1}, tasks)

	// Insert two more tasks.
	t2 := MakeTestTask(time.Unix(0, 1470674376000000), []string{"e", "f"})
	t2.Id = "2"
	m.TrackModifiedTask(t2)
	t3 := MakeTestTask(time.Unix(0, 1470674884000000), []string{"g", "h"})
	t3.Id = "3"
	m.TrackModifiedTask(t3)

	// Ensure that both tasks show up in the modified list.
	tasks, err = m.GetModifiedTasks(id)
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, []*Task{t2, t3}, tasks)

	// Check StopTrackingModifiedTasks.
	m.StopTrackingModifiedTasks(id)
	_, err = m.GetModifiedTasks(id)
	assert.True(t, IsUnknownId(err))
}

// Test that if a Task is modified multiple times, it only appears once in the
// result of GetModifiedTasks.
func TestMultipleTaskModifications(t *testing.T) {
	testutils.SmallTest(t)
	m := ModifiedTasks{}

	id, err := m.StartTrackingModifiedTasks()
	assert.NoError(t, err)

	t1 := MakeTestTask(time.Unix(0, 1470674132000000), []string{"a", "b", "c", "d"})
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
	deepequal.AssertDeepEqual(t, []*Task{t1}, tasks)
}

func TestModifiedTasksTooManyUsers(t *testing.T) {
	testutils.SmallTest(t)
	m := ModifiedTasks{}

	var oneId string
	// Max out the number of modified-tasks users; ensure that we error out.
	for i := 0; i < MAX_MODIFIED_DATA_USERS; i++ {
		id, err := m.StartTrackingModifiedTasks()
		assert.NoError(t, err)
		oneId = id
	}
	_, err := m.StartTrackingModifiedTasks()
	assert.True(t, IsTooManyUsers(err))

	m.StopTrackingModifiedTasks(oneId)
	_, err = m.StartTrackingModifiedTasks()
	assert.NoError(t, err)
}

func TestModifiedJobs(t *testing.T) {
	testutils.SmallTest(t)
	m := ModifiedJobs{}

	_, err := m.GetModifiedJobs("dummy-id")
	assert.True(t, IsUnknownId(err))

	id, err := m.StartTrackingModifiedJobs()
	assert.NoError(t, err)

	jobs, err := m.GetModifiedJobs(id)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(jobs))

	j1 := makeJob(time.Unix(0, 1470674132000000))
	j1.Id = "1"

	// Insert the job.
	m.TrackModifiedJob(j1)

	// Ensure that the job shows up in the modified list.
	jobs, err = m.GetModifiedJobs(id)
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, []*Job{j1}, jobs)

	// Insert two more jobs.
	j2 := makeJob(time.Unix(0, 1470674376000000))
	j2.Id = "2"
	m.TrackModifiedJob(j2)
	j3 := makeJob(time.Unix(0, 1470674884000000))
	j3.Id = "3"
	m.TrackModifiedJob(j3)

	// Ensure that both jobs show up in the modified list.
	jobs, err = m.GetModifiedJobs(id)
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, []*Job{j2, j3}, jobs)

	// Check StopTrackingModifiedJobs.
	m.StopTrackingModifiedJobs(id)
	_, err = m.GetModifiedJobs(id)
	assert.True(t, IsUnknownId(err))
}

func TestMultipleJobModifications(t *testing.T) {
	testutils.SmallTest(t)
	m := ModifiedJobs{}

	id, err := m.StartTrackingModifiedJobs()
	assert.NoError(t, err)

	j1 := makeJob(time.Unix(0, 1470674132000000))
	j1.Id = "1"

	// Insert the job.
	m.TrackModifiedJob(j1)

	// Make several more modifications.
	j1.Status = JOB_STATUS_IN_PROGRESS
	m.TrackModifiedJob(j1)
	j1.Status = JOB_STATUS_SUCCESS
	m.TrackModifiedJob(j1)

	// Ensure that the task shows up only once in the modified list.
	jobs, err := m.GetModifiedJobs(id)
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, []*Job{j1}, jobs)
}

func TestModifiedJobsTooManyUsers(t *testing.T) {
	testutils.SmallTest(t)
	m := ModifiedJobs{}

	var oneId string
	// Max out the number of modified-tasks users; ensure that we error out.
	for i := 0; i < MAX_MODIFIED_DATA_USERS; i++ {
		id, err := m.StartTrackingModifiedJobs()
		assert.NoError(t, err)
		oneId = id
	}
	_, err := m.StartTrackingModifiedJobs()
	assert.True(t, IsTooManyUsers(err))

	m.StopTrackingModifiedJobs(oneId)
	_, err = m.StartTrackingModifiedJobs()
	assert.NoError(t, err)
}
