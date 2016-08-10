package db

import (
	"fmt"
	"testing"
	"time"

	"github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils"
)

func makeTask(id string, ts time.Time, commits []string) *Task {
	return &Task{
		SwarmingRpcsTaskRequestMetadata: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskResult: &swarming.SwarmingRpcsTaskResult{
				CreatedTs: fmt.Sprintf("%d", ts.UnixNano()),
			},
		},
		Commits: commits,
		Id:      id,
		Name:    "Test-Task",
	}
}

func testDB(t *testing.T, db DB) {
	defer testutils.AssertCloses(t, db)

	_, err := db.GetModifiedTasks("dummy-id")
	assert.True(t, IsUnknownId(err))

	id, err := db.StartTrackingModifiedTasks()
	assert.NoError(t, err)

	tasks, err := db.GetModifiedTasks(id)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))

	// Insert a task.
	t1 := makeTask("task1", time.Unix(0, 1470674132000000), []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutTask(t1))

	// Ensure that the task shows up in the modified list.
	tasks, err = db.GetModifiedTasks(id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1}, tasks)

	// Ensure that the task shows up in the correct date ranges.
	timeStart := time.Time{}
	t1Before, err := t1.Created()
	assert.NoError(t, err)
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
	t2 := makeTask("task2", time.Unix(0, 1470674376000000), []string{"e", "f"})
	t3 := makeTask("task3", time.Unix(0, 1470674884000000), []string{"g", "h"})
	assert.NoError(t, db.PutTasks([]*Task{t2, t3}))

	// Ensure that both tasks show up in the modified list.
	tasks, err = db.GetModifiedTasks(id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t2, t3}, tasks)

	// Ensure that all tasks show up in the correct time ranges, in sorted order.
	t2Before, err := t2.Created()
	assert.NoError(t, err)
	t2After := t2Before.Add(1 * time.Millisecond)

	t3Before, err := t3.Created()
	assert.NoError(t, err)
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

func testTooManyUsers(t *testing.T, db DB) {
	defer testutils.AssertCloses(t, db)

	// Max out the number of modified-tasks users; ensure that we error out.
	for i := 0; i < MAX_MODIFIED_BUILDS_USERS; i++ {
		_, err := db.StartTrackingModifiedTasks()
		assert.NoError(t, err)
	}
	_, err := db.StartTrackingModifiedTasks()
	assert.True(t, IsTooManyUsers(err))
}

func TestInMemoryDB(t *testing.T) {
	testDB(t, NewInMemoryDB())
}

func TestInMemoryTooManyUsers(t *testing.T) {
	testTooManyUsers(t, NewInMemoryDB())
}
