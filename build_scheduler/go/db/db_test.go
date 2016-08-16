package db

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/luci/luci-go/common/api/swarming/swarming/v1"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
)

func TestUpdateFromSwarming(t *testing.T) {
	now := time.Now()
	task := Task{
		Id:       "A",
		Name:     "A",
		Repo:     "A",
		Revision: "A",
		Created:  now,
	}

	// Invalid input.
	assert.Error(t, task.UpdateFromSwarming(&swarming.SwarmingRpcsTaskRequestMetadata{}))
	assert.Error(t, task.UpdateFromSwarming(&swarming.SwarmingRpcsTaskRequestMetadata{
		TaskResult: &swarming.SwarmingRpcsTaskResult{
			Tags: []string{"invalid"},
		},
	}))
	// Unchanged.
	testutils.AssertDeepEqual(t, task, Task{
		Id:             "A",
		Name:           "A",
		Repo:           "A",
		Revision:       "A",
		Created:        now,
		Status:         TASK_STATUS_PENDING,
		IsolatedOutput: "",
		Swarming:       nil,
	})

	// Mismatched data.
	s := &swarming.SwarmingRpcsTaskRequestMetadata{
		TaskResult: &swarming.SwarmingRpcsTaskResult{
			CreatedTs: fmt.Sprintf("%d", now.UnixNano()),
			Failure:   false,
			State:     SWARMING_STATE_COMPLETED,
			Tags: []string{
				fmt.Sprintf("%s:B", SWARMING_TAG_ID),
				fmt.Sprintf("%s:A", SWARMING_TAG_NAME),
				fmt.Sprintf("%s:A", SWARMING_TAG_REPO),
				fmt.Sprintf("%s:A", SWARMING_TAG_REVISION),
			},
		},
	}
	assert.Error(t, task.UpdateFromSwarming(s))
	s.TaskResult.Tags[0] = fmt.Sprintf("%s:A", SWARMING_TAG_ID)
	s.TaskResult.Tags[1] = fmt.Sprintf("%s:B", SWARMING_TAG_NAME)
	assert.Error(t, task.UpdateFromSwarming(s))
	s.TaskResult.Tags[1] = fmt.Sprintf("%s:A", SWARMING_TAG_NAME)
	s.TaskResult.Tags[2] = fmt.Sprintf("%s:B", SWARMING_TAG_REPO)
	assert.Error(t, task.UpdateFromSwarming(s))
	s.TaskResult.Tags[0] = fmt.Sprintf("%s:A", SWARMING_TAG_REPO)
	s.TaskResult.Tags[1] = fmt.Sprintf("%s:B", SWARMING_TAG_REVISION)
	assert.Error(t, task.UpdateFromSwarming(s))
	s.TaskResult.Tags[1] = fmt.Sprintf("%s:A", SWARMING_TAG_REVISION)
	s.TaskResult.CreatedTs = fmt.Sprintf("%d", now.Add(time.Hour).UnixNano())
	assert.Error(t, task.UpdateFromSwarming(s))
	// Unchanged.
	testutils.AssertDeepEqual(t, task, Task{
		Id:             "A",
		Name:           "A",
		Repo:           "A",
		Revision:       "A",
		Created:        now,
		Status:         TASK_STATUS_PENDING,
		IsolatedOutput: "",
		Swarming:       nil,
	})

	// Basic update.
	task = Task{}
	s = &swarming.SwarmingRpcsTaskRequestMetadata{
		TaskResult: &swarming.SwarmingRpcsTaskResult{
			CreatedTs: fmt.Sprintf("%d", now.Add(2*time.Hour).UnixNano()),
			Failure:   false,
			State:     SWARMING_STATE_COMPLETED,
			Tags: []string{
				fmt.Sprintf("%s:C", SWARMING_TAG_ID),
				fmt.Sprintf("%s:C", SWARMING_TAG_NAME),
				fmt.Sprintf("%s:C", SWARMING_TAG_REPO),
				fmt.Sprintf("%s:C", SWARMING_TAG_REVISION),
			},
			OutputsRef: &swarming.SwarmingRpcsFilesRef{
				Isolated: "C",
			},
		},
	}
	assert.NoError(t, task.UpdateFromSwarming(s))
	testutils.AssertDeepEqual(t, task, Task{
		Id:             "C",
		Name:           "C",
		Repo:           "C",
		Revision:       "C",
		Created:        now.Add(2 * time.Hour),
		Status:         TASK_STATUS_SUCCESS,
		IsolatedOutput: "C",
		Swarming:       s,
	})

	// Status updates.

	s.TaskResult.OutputsRef = nil
	s.TaskResult.State = SWARMING_STATE_PENDING
	assert.NoError(t, task.UpdateFromSwarming(s))
	testutils.AssertDeepEqual(t, task, Task{
		Id:             "C",
		Name:           "C",
		Repo:           "C",
		Revision:       "C",
		Created:        now.Add(2 * time.Hour),
		Status:         TASK_STATUS_PENDING,
		IsolatedOutput: "",
		Swarming:       s,
	})

	s.TaskResult.State = SWARMING_STATE_RUNNING
	assert.NoError(t, task.UpdateFromSwarming(s))
	testutils.AssertDeepEqual(t, task, Task{
		Id:             "C",
		Name:           "C",
		Repo:           "C",
		Revision:       "C",
		Created:        now.Add(2 * time.Hour),
		Status:         TASK_STATUS_RUNNING,
		IsolatedOutput: "",
		Swarming:       s,
	})

	s.TaskResult.OutputsRef = &swarming.SwarmingRpcsFilesRef{
		Isolated: "",
	}
	for _, state := range []string{SWARMING_STATE_BOT_DIED, SWARMING_STATE_CANCELED, SWARMING_STATE_EXPIRED, SWARMING_STATE_TIMED_OUT} {
		s.TaskResult.State = state
		assert.NoError(t, task.UpdateFromSwarming(s))
		testutils.AssertDeepEqual(t, task, Task{
			Id:             "C",
			Name:           "C",
			Repo:           "C",
			Revision:       "C",
			Created:        now.Add(2 * time.Hour),
			Status:         TASK_STATUS_MISHAP,
			IsolatedOutput: "",
			Swarming:       s,
		})
	}

	s.TaskResult.OutputsRef.Isolated = "D"
	s.TaskResult.State = SWARMING_STATE_COMPLETED
	s.TaskResult.Failure = true
	assert.NoError(t, task.UpdateFromSwarming(s))
	testutils.AssertDeepEqual(t, task, Task{
		Id:             "C",
		Name:           "C",
		Repo:           "C",
		Revision:       "C",
		Created:        now.Add(2 * time.Hour),
		Status:         TASK_STATUS_FAILURE,
		IsolatedOutput: "D",
		Swarming:       s,
	})
}

func makeTask(ts time.Time, commits []string) *Task {
	return &Task{
		Created: ts,
		Commits: commits,
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
