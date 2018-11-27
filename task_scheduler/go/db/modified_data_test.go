package db

import (
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestDefaultModifiedTasks(t testutils.TestingT) {
	testutils.MediumTest(t)
	TestModifiedTasks(t, &ModifiedTasksImpl{})
}

// Test that if a Task is modified multiple times, it only appears once in the
// result of GetModifiedTasks.
func TestDefaultMultipleTaskModifications(t testutils.TestingT) {
	testutils.MediumTest(t)
	TestMultipleTaskModifications(t, &ModifiedTasksImpl{})
}

func TestDefaultModifiedTasksTooManyUsers(t testutils.TestingT) {
	testutils.MediumTest(t)
	m := ModifiedTasksImpl{}

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

func TestDefaultModifiedJobs(t testutils.TestingT) {
	testutils.MediumTest(t)
	TestModifiedJobs(t, &ModifiedJobsImpl{})
}

func TestDefaultMultipleJobModifications(t testutils.TestingT) {
	testutils.MediumTest(t)
	TestMultipleJobModifications(t, &ModifiedJobsImpl{})
}

func TestDefaultModifiedJobsTooManyUsers(t testutils.TestingT) {
	testutils.MediumTest(t)
	m := ModifiedJobsImpl{}

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
