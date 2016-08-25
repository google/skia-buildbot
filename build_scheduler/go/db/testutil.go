package db

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

const DEFAULT_TEST_REPO = "go-on-now.git"

func makeTask(ts time.Time, commits []string) *Task {
	return &Task{
		Created: ts,
		Repo:    DEFAULT_TEST_REPO,
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

	t1 := makeTask(time.Time{}, []string{"a", "b", "c", "d"})

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

	// Set Creation time. Ensure Created is not the time of AssignId to test the
	// sequence (1) AssignId, (2) initialize task, (3) PutTask.
	now := time.Now().Add(time.Nanosecond)
	t1.Created = now

	// Insert the task.
	assert.NoError(t, db.PutTask(t1))

	// Check that DbModified was set.
	assert.False(t, util.TimeIsZero(t1.DbModified))
	t1LastModified := t1.DbModified

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
	t1After := t1Before.Add(1 * time.Nanosecond)
	timeEnd := now.Add(2 * time.Nanosecond)
	tasks, err = db.GetTasksFromDateRange(timeStart, t1Before)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))
	tasks, err = db.GetTasksFromDateRange(t1Before, t1After)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1}, tasks)
	tasks, err = db.GetTasksFromDateRange(t1After, timeEnd)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))

	// Insert two more tasks. Ensure at least 1 nanosecond between task Created
	// times so that t1After != t2Before and t2After != t3Before.
	t2 := makeTask(now.Add(time.Nanosecond), []string{"e", "f"})
	t3 := makeTask(now.Add(2*time.Nanosecond), []string{"g", "h"})
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

	// Make an update to t1 and t2. Ensure modified times change.
	t2LastModified := t2.DbModified
	t1.Status = TASK_STATUS_RUNNING
	t2.Status = TASK_STATUS_SUCCESS
	assert.NoError(t, db.PutTasks([]*Task{t1, t2}))
	assert.False(t, t1.DbModified.Equal(t1LastModified))
	assert.False(t, t2.DbModified.Equal(t2LastModified))

	// Ensure that both tasks show up in the modified list.
	tasks, err = db.GetModifiedTasks(id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1, t2}, tasks)

	// Ensure that all tasks show up in the correct time ranges, in sorted order.
	t2Before := t2.Created
	t2After := t2Before.Add(1 * time.Nanosecond)

	t3Before := t3.Created
	t3After := t3Before.Add(1 * time.Nanosecond)

	timeEnd = now.Add(3 * time.Nanosecond)

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
	for i := 0; i < MAX_MODIFIED_TASKS_USERS; i++ {
		_, err := db.StartTrackingModifiedTasks()
		assert.NoError(t, err)
	}
	_, err := db.StartTrackingModifiedTasks()
	assert.True(t, IsTooManyUsers(err))
}

// Test that PutTask and PutTasks return ErrConcurrentUpdate when a cached Task
// has been updated in the DB.
func TestConcurrentUpdate(t *testing.T, db DB) {
	defer testutils.AssertCloses(t, db)

	// Insert a task.
	t1 := makeTask(time.Now(), []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutTask(t1))

	// Retrieve a copy of the task.
	t1Cached, err := db.GetTaskById(t1.Id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, t1, t1Cached)

	// Update the original task.
	t1.Commits = []string{"a", "b"}
	assert.NoError(t, db.PutTask(t1))

	// Update the cached copy; should get concurrent update error.
	t1Cached.Status = TASK_STATUS_RUNNING
	err = db.PutTask(t1Cached)
	assert.True(t, IsConcurrentUpdate(err))

	{
		// DB should still have the old value of t1.
		t1Again, err := db.GetTaskById(t1.Id)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, t1, t1Again)
	}

	// Insert a second task.
	t2 := makeTask(time.Now(), []string{"e", "f"})
	assert.NoError(t, db.PutTask(t2))

	// Update t2 at the same time as t1Cached; should still get an error.
	t2.Status = TASK_STATUS_MISHAP
	err = db.PutTasks([]*Task{t2, t1Cached})
	assert.True(t, IsConcurrentUpdate(err))

	{
		// DB should still have the old value of t1.
		t1Again, err := db.GetTaskById(t1.Id)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, t1, t1Again)

		// DB should also still have the old value of t2, but to keep InMemoryDB
		// simple, we don't check that here.
	}
}

// Test UpdateWithRetries when no errors or retries.
func testUpdateWithRetriesSimple(t *testing.T, db DB) {
	begin := time.Now()

	// Test no-op.
	tasks, err := UpdateWithRetries(db, func() ([]*Task, error) {
		return nil, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))

	// Create new task t1. (UpdateWithRetries isn't actually useful in this case.)
	tasks, err = UpdateWithRetries(db, func() ([]*Task, error) {
		t1 := makeTask(time.Time{}, []string{"a", "b", "c", "d"})
		assert.NoError(t, db.AssignId(t1))
		t1.Created = time.Now().Add(time.Nanosecond)
		return []*Task{t1}, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	t1 := tasks[0]

	// Update t1 and create t2.
	tasks, err = UpdateWithRetries(db, func() ([]*Task, error) {
		t1, err := db.GetTaskById(t1.Id)
		assert.NoError(t, err)
		t1.Status = TASK_STATUS_RUNNING
		t2 := makeTask(t1.Created.Add(time.Nanosecond), []string{"e", "f"})
		return []*Task{t1, t2}, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
	assert.Equal(t, TASK_STATUS_RUNNING, tasks[0].Status)
	assert.Equal(t, []string{"e", "f"}, tasks[1].Commits)

	// Check that return value matches what's in the DB.
	t1, err = db.GetTaskById(t1.Id)
	assert.NoError(t, err)
	t2, err := db.GetTaskById(tasks[1].Id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, tasks[0], t1)
	testutils.AssertDeepEqual(t, tasks[1], t2)

	// Check no extra tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(begin, time.Now().Add(3*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
	assert.Equal(t, t2.Id, tasks[1].Id)
}

// Test UpdateWithRetries when there are some retries, but eventual success.
func testUpdateWithRetriesSuccess(t *testing.T, db DB) {
	begin := time.Now()

	// Create and cache.
	t1 := makeTask(begin.Add(time.Nanosecond), []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutTask(t1))
	t1Cached := t1.Copy()

	// Update original.
	t1.Status = TASK_STATUS_RUNNING
	assert.NoError(t, db.PutTask(t1))

	// Attempt update.
	callCount := 0
	tasks, err := UpdateWithRetries(db, func() ([]*Task, error) {
		callCount++
		if callCount >= 3 {
			if task, err := db.GetTaskById(t1.Id); err != nil {
				return nil, err
			} else {
				t1Cached = task
			}
		}
		t1Cached.Status = TASK_STATUS_SUCCESS
		t2 := makeTask(begin.Add(2*time.Nanosecond), []string{"e", "f"})
		return []*Task{t1Cached, t2}, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
	assert.Equal(t, 2, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
	assert.Equal(t, TASK_STATUS_SUCCESS, tasks[0].Status)
	assert.Equal(t, []string{"e", "f"}, tasks[1].Commits)

	// Check that return value matches what's in the DB.
	t1, err = db.GetTaskById(t1.Id)
	assert.NoError(t, err)
	t2, err := db.GetTaskById(tasks[1].Id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, tasks[0], t1)
	testutils.AssertDeepEqual(t, tasks[1], t2)

	// Check no extra tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(begin, time.Now().Add(3*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
	assert.Equal(t, t2.Id, tasks[1].Id)
}

// Test UpdateWithRetries when f returns an error.
func testUpdateWithRetriesErrorInFunc(t *testing.T, db DB) {
	begin := time.Now()

	myErr := fmt.Errorf("NO! Bad dog!")
	callCount := 0
	tasks, err := UpdateWithRetries(db, func() ([]*Task, error) {
		callCount++
		// Return a task just for fun.
		return []*Task{
			makeTask(begin.Add(time.Nanosecond), []string{"a", "b", "c", "d"}),
		}, myErr
	})
	assert.Error(t, err)
	assert.Equal(t, myErr, err)
	assert.Equal(t, 0, len(tasks))
	assert.Equal(t, 1, callCount)

	// Check no tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(begin, time.Now().Add(2*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))
}

// Test UpdateWithRetries when PutTasks returns an error.
func testUpdateWithRetriesErrorInPutTasks(t *testing.T, db DB) {
	begin := time.Now()

	callCount := 0
	tasks, err := UpdateWithRetries(db, func() ([]*Task, error) {
		callCount++
		// Task has zero Created time.
		return []*Task{
			makeTask(time.Time{}, []string{"a", "b", "c", "d"}),
		}, nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Created not set.")
	assert.Equal(t, 0, len(tasks))
	assert.Equal(t, 1, callCount)

	// Check no tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(begin, time.Now().Add(time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))
}

// Test UpdateWithRetries when retries are exhausted.
func testUpdateWithRetriesExhausted(t *testing.T, db DB) {
	begin := time.Now()

	// Create and cache.
	t1 := makeTask(begin.Add(time.Nanosecond), []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutTask(t1))
	t1Cached := t1.Copy()

	// Update original.
	t1.Status = TASK_STATUS_RUNNING
	assert.NoError(t, db.PutTask(t1))

	// Attempt update.
	callCount := 0
	tasks, err := UpdateWithRetries(db, func() ([]*Task, error) {
		callCount++
		t1Cached.Status = TASK_STATUS_SUCCESS
		t2 := makeTask(begin.Add(2*time.Nanosecond), []string{"e", "f"})
		return []*Task{t1Cached, t2}, nil
	})
	assert.True(t, IsConcurrentUpdate(err))
	assert.Equal(t, NUM_RETRIES, callCount)
	assert.Equal(t, 0, len(tasks))

	// Check no extra tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(begin, time.Now().Add(3*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
	assert.Equal(t, TASK_STATUS_RUNNING, tasks[0].Status)
}

// Test UpdateTaskWithRetries when no errors or retries.
func testUpdateTaskWithRetriesSimple(t *testing.T, db DB) {
	begin := time.Now()

	// Create new task t1.
	t1 := makeTask(time.Time{}, []string{"a", "b", "c", "d"})
	assert.NoError(t, db.AssignId(t1))
	t1.Created = time.Now().Add(time.Nanosecond)
	assert.NoError(t, db.PutTask(t1))

	// Update t1.
	t1Updated, err := UpdateTaskWithRetries(db, t1.Id, func(task *Task) error {
		task.Status = TASK_STATUS_RUNNING
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, t1.Id, t1Updated.Id)
	assert.Equal(t, TASK_STATUS_RUNNING, t1Updated.Status)
	assert.NotEqual(t, t1.DbModified, t1Updated.DbModified)

	// Check that return value matches what's in the DB.
	t1Again, err := db.GetTaskById(t1.Id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, t1Again, t1Updated)

	// Check no extra tasks in the DB.
	tasks, err := db.GetTasksFromDateRange(begin, time.Now().Add(2*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
}

// Test UpdateTaskWithRetries when there are some retries, but eventual success.
func testUpdateTaskWithRetriesSuccess(t *testing.T, db DB) {
	begin := time.Now()

	// Create new task t1.
	t1 := makeTask(begin.Add(time.Nanosecond), []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutTask(t1))

	// Attempt update.
	callCount := 0
	t1Updated, err := UpdateTaskWithRetries(db, t1.Id, func(task *Task) error {
		callCount++
		if callCount < 3 {
			// Sneakily make an update in the background.
			t1.Commits = append(t1.Commits, fmt.Sprintf("z%d", callCount))
			assert.NoError(t, db.PutTask(t1))
		}
		task.Status = TASK_STATUS_SUCCESS
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
	assert.Equal(t, t1.Id, t1Updated.Id)
	assert.Equal(t, TASK_STATUS_SUCCESS, t1Updated.Status)

	// Check that return value matches what's in the DB.
	t1Again, err := db.GetTaskById(t1.Id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, t1Again, t1Updated)

	// Check no extra tasks in the DB.
	tasks, err := db.GetTasksFromDateRange(begin, time.Now().Add(2*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
}

// Test UpdateTaskWithRetries when f returns an error.
func testUpdateTaskWithRetriesErrorInFunc(t *testing.T, db DB) {
	begin := time.Now()

	// Create new task t1.
	t1 := makeTask(begin.Add(time.Nanosecond), []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutTask(t1))

	// Update and return an error.
	myErr := fmt.Errorf("Um, actually, I didn't want to update that task.")
	callCount := 0
	noTask, err := UpdateTaskWithRetries(db, t1.Id, func(task *Task) error {
		callCount++
		// Update task to test nothing changes in DB.
		task.Status = TASK_STATUS_RUNNING
		return myErr
	})
	assert.Error(t, err)
	assert.Equal(t, myErr, err)
	assert.Nil(t, noTask)
	assert.Equal(t, 1, callCount)

	// Check task did not change in the DB.
	t1Again, err := db.GetTaskById(t1.Id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, t1, t1Again)

	// Check no extra tasks in the DB.
	tasks, err := db.GetTasksFromDateRange(begin, time.Now().Add(2*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
}

// Test UpdateTaskWithRetries when retries are exhausted.
func testUpdateTaskWithRetriesExhausted(t *testing.T, db DB) {
	begin := time.Now()

	// Create new task t1.
	t1 := makeTask(begin.Add(time.Nanosecond), []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutTask(t1))

	// Update original.
	t1.Status = TASK_STATUS_RUNNING
	assert.NoError(t, db.PutTask(t1))

	// Attempt update.
	callCount := 0
	noTask, err := UpdateTaskWithRetries(db, t1.Id, func(task *Task) error {
		callCount++
		// Sneakily make an update in the background.
		t1.Commits = append(t1.Commits, fmt.Sprintf("z%d", callCount))
		assert.NoError(t, db.PutTask(t1))

		task.Status = TASK_STATUS_SUCCESS
		return nil
	})
	assert.True(t, IsConcurrentUpdate(err))
	assert.Equal(t, NUM_RETRIES, callCount)
	assert.Nil(t, noTask)

	// Check task did not change in the DB.
	t1Again, err := db.GetTaskById(t1.Id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, t1, t1Again)

	// Check no extra tasks in the DB.
	tasks, err := db.GetTasksFromDateRange(begin, time.Now().Add(2*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
}

// Test UpdateTaskWithRetries when the given ID is not found in the DB.
func testUpdateTaskWithRetriesTaskNotFound(t *testing.T, db DB) {
	begin := time.Now()

	// Assign ID for a task, but don't put it in the DB.
	t1 := makeTask(begin.Add(time.Nanosecond), []string{"a", "b", "c", "d"})
	assert.NoError(t, db.AssignId(t1))

	// Attempt to update non-existent task. Function shouldn't be called.
	callCount := 0
	noTask, err := UpdateTaskWithRetries(db, t1.Id, func(task *Task) error {
		callCount++
		task.Status = TASK_STATUS_RUNNING
		return nil
	})
	assert.True(t, IsNotFound(err))
	assert.Nil(t, noTask)
	assert.Equal(t, 0, callCount)

	// Check no tasks in the DB.
	tasks, err := db.GetTasksFromDateRange(begin, time.Now().Add(2*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))
}

// Test UpdateWithRetries and UpdateTaskWithRetries.
func TestUpdateWithRetries(t *testing.T, db DB) {
	testUpdateWithRetriesSimple(t, db)
	testUpdateWithRetriesSuccess(t, db)
	testUpdateWithRetriesErrorInFunc(t, db)
	testUpdateWithRetriesErrorInPutTasks(t, db)
	testUpdateWithRetriesExhausted(t, db)
	testUpdateTaskWithRetriesSimple(t, db)
	testUpdateTaskWithRetriesSuccess(t, db)
	testUpdateTaskWithRetriesErrorInFunc(t, db)
	testUpdateTaskWithRetriesExhausted(t, db)
	testUpdateTaskWithRetriesTaskNotFound(t, db)
}
