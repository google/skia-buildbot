package modified

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
)

func TestDefaultModifiedTasks(t *testing.T) {
	testutils.MediumTest(t)
	db.TestModifiedTasks(t, &ModifiedTasksImpl{})
}

// Test that if a Task is modified multiple times, it only appears once in the
// result of GetModifiedTasks.
func TestDefaultMultipleTaskModifications(t *testing.T) {
	testutils.MediumTest(t)
	db.TestMultipleTaskModifications(t, &ModifiedTasksImpl{})
}

func TestDefaultModifiedTasksTooManyUsers(t *testing.T) {
	testutils.MediumTest(t)
	m := ModifiedTasksImpl{}

	var oneId string
	// Max out the number of modified-tasks users; ensure that we error out.
	for i := 0; i < db.MAX_MODIFIED_DATA_USERS; i++ {
		id, err := m.StartTrackingModifiedTasks()
		assert.NoError(t, err)
		oneId = id
	}
	_, err := m.StartTrackingModifiedTasks()
	assert.True(t, db.IsTooManyUsers(err))

	m.StopTrackingModifiedTasks(oneId)
	_, err = m.StartTrackingModifiedTasks()
	assert.NoError(t, err)
}

func TestDefaultModifiedJobs(t *testing.T) {
	testutils.MediumTest(t)
	db.TestModifiedJobs(t, &ModifiedJobsImpl{})
}

func TestDefaultMultipleJobModifications(t *testing.T) {
	testutils.MediumTest(t)
	db.TestMultipleJobModifications(t, &ModifiedJobsImpl{})
}

func TestDefaultModifiedJobsTooManyUsers(t *testing.T) {
	testutils.MediumTest(t)
	m := ModifiedJobsImpl{}

	var oneId string
	// Max out the number of modified-tasks users; ensure that we error out.
	for i := 0; i < db.MAX_MODIFIED_DATA_USERS; i++ {
		id, err := m.StartTrackingModifiedJobs()
		assert.NoError(t, err)
		oneId = id
	}
	_, err := m.StartTrackingModifiedJobs()
	assert.True(t, db.IsTooManyUsers(err))

	m.StopTrackingModifiedJobs(oneId)
	_, err = m.StartTrackingModifiedJobs()
	assert.NoError(t, err)
}

func TestDefaultModifiedComments(t *testing.T) {
	testutils.MediumTest(t)
	m := &ModifiedCommentsImpl{}
	db.TestModifiedComments(t, m)
}

func TestDefaultMultipleCommentModifications(t *testing.T) {
	testutils.MediumTest(t)
	m := &ModifiedCommentsImpl{}
	db.TestMultipleCommentModifications(t, m)
}

func TestDefaultModifiedCommentsTooManyUsers(t *testing.T) {
	testutils.MediumTest(t)
	m := ModifiedCommentsImpl{}

	var oneId string
	// Max out the number of modified-tasks users; ensure that we error out.
	for i := 0; i < db.MAX_MODIFIED_DATA_USERS; i++ {
		id, err := m.StartTrackingModifiedComments()
		assert.NoError(t, err)
		oneId = id
	}
	_, err := m.StartTrackingModifiedComments()
	assert.True(t, db.IsTooManyUsers(err))

	m.StopTrackingModifiedComments(oneId)
	_, err = m.StartTrackingModifiedComments()
	assert.NoError(t, err)
}
