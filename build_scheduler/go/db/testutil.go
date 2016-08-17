package db

import (
	"net/url"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
)

func makeTask(ts time.Time, commits []string) *Task {
	return &Task{
		Created: ts,
		Commits: commits,
		Name:    "Test-Task",
	}
}

func TestDB(t *testing.T, db DB) {
	defer testutils.AssertCloses(t, db)

	_, err := db.GetModifiedTasks("dummy-id")
	assert.True(t, IsUnknownId(err))

	id, err := db.StartTrackingModifiedTasks()
	assert.NoError(t, err)

	tasks, err := db.GetModifiedTasks(id)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))

	t1 := makeTask(time.Unix(0, 1470674132000000), []string{"a", "b", "c", "d"})

	// AssignId should fill in t1.Id.
	assert.Equal(t, "", t1.Id)
	assert.NoError(t, db.AssignId(t1))
	assert.NotEqual(t, "", t1.Id)
	// Ids must be URL-safe.
	assert.Equal(t, url.QueryEscape(t1.Id), t1.Id)

	// Task doesn't exist in DB yet.
	noTask, err := db.GetTaskById(t1.Id)
	assert.NoError(t, err)
	assert.Nil(t, noTask)

	// Insert the task.
	assert.NoError(t, db.PutTask(t1))

	// Task can now be retrieved by Id.
	t1Again, err := db.GetTaskById(t1.Id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, t1, t1Again)

	// Ensure that the task shows up in the modified list.
	tasks, err = db.GetModifiedTasks(id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1}, tasks)

	// Ensure that the task shows up in the correct date ranges.
	timeStart := time.Time{}
	t1Before := t1.Created
	t1After := t1Before.Add(1 * time.Millisecond)
	timeEnd := time.Now()
	tasks, err = db.GetTasksFromDateRange(timeStart, t1Before)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))
	tasks, err = db.GetTasksFromDateRange(t1Before, t1After)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1}, tasks)
	tasks, err = db.GetTasksFromDateRange(t1After, timeEnd)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))

	// Insert two more tasks.
	t2 := makeTask(time.Unix(0, 1470674376000000), []string{"e", "f"})
	t3 := makeTask(time.Unix(0, 1470674884000000), []string{"g", "h"})
	assert.NoError(t, db.PutTasks([]*Task{t2, t3}))

	// Check that PutTasks assigned Ids.
	assert.NotEqual(t, "", t2.Id)
	assert.NotEqual(t, "", t3.Id)
	// Ids must be URL-safe.
	assert.Equal(t, url.QueryEscape(t2.Id), t2.Id)
	assert.Equal(t, url.QueryEscape(t3.Id), t3.Id)

	// Ensure that both tasks show up in the modified list.
	tasks, err = db.GetModifiedTasks(id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t2, t3}, tasks)

	// Ensure that all tasks show up in the correct time ranges, in sorted order.
	t2Before := t2.Created
	t2After := t2Before.Add(1 * time.Millisecond)

	t3Before := t3.Created
	t3After := t3Before.Add(1 * time.Millisecond)

	tasks, err = db.GetTasksFromDateRange(timeStart, t1Before)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))

	tasks, err = db.GetTasksFromDateRange(timeStart, t1After)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1}, tasks)

	tasks, err = db.GetTasksFromDateRange(timeStart, t2Before)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1}, tasks)

	tasks, err = db.GetTasksFromDateRange(timeStart, t2After)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1, t2}, tasks)

	tasks, err = db.GetTasksFromDateRange(timeStart, t3Before)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1, t2}, tasks)

	tasks, err = db.GetTasksFromDateRange(timeStart, t3After)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1, t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(timeStart, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1, t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(t1Before, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1, t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(t1After, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(t2Before, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(t2After, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(t3Before, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(t3After, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{}, tasks)
}

func TestTooManyUsers(t *testing.T, db DB) {
	defer testutils.AssertCloses(t, db)

	// Max out the number of modified-tasks users; ensure that we error out.
	for i := 0; i < MAX_MODIFIED_BUILDS_USERS; i++ {
		_, err := db.StartTrackingModifiedTasks()
		assert.NoError(t, err)
	}
	_, err := db.StartTrackingModifiedTasks()
	assert.True(t, IsTooManyUsers(err))
}
