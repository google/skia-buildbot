package db

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"sort"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/window"
)

const DEFAULT_TEST_REPO = "go-on-now.git"

// AssertDeepEqual does a deep equals comparison using the testutils.TestingT interface.
//
// Callers of these tests utils should assign a value to AssertDeepEqual beforehand, e.g.:
//
//	AssertDeepEqual = deepequal.AssertDeepEqual
//
// This is necessary to break the hard linking of this file to the "testing" module.
var AssertDeepEqual func(t testutils.TestingT, expected, actual interface{})

func MakeTestTask(ts time.Time, commits []string) *Task {
	return &Task{
		Created: ts,
		TaskKey: TaskKey{
			RepoState: RepoState{
				Repo:     DEFAULT_TEST_REPO,
				Revision: commits[0],
			},
			Name: "Test-Task",
		},
		Commits:        commits,
		SwarmingTaskId: "swarmid",
	}
}

func makeJob(ts time.Time) *Job {
	return &Job{
		Created:      ts,
		Dependencies: map[string][]string{},
		RepoState: RepoState{
			Repo: DEFAULT_TEST_REPO,
		},
		Name:  "Test-Job",
		Tasks: map[string][]*TaskSummary{},
	}
}

// TestTaskDB performs basic tests for an implementation of TaskDB.
func TestTaskDB(t testutils.TestingT, db TaskDB) {
	_, err := db.GetModifiedTasks("dummy-id")
	assert.True(t, IsUnknownId(err))

	id, err := db.StartTrackingModifiedTasks()
	assert.NoError(t, err)

	tasks, err := db.GetModifiedTasks(id)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))

	t1 := MakeTestTask(time.Time{}, []string{"a", "b", "c", "d"})

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
	timeStart := time.Now()
	now := timeStart.Add(time.Nanosecond)
	t1.Created = now

	// Insert the task.
	assert.NoError(t, db.PutTask(t1))

	// Check that DbModified was set.
	assert.False(t, util.TimeIsZero(t1.DbModified))
	t1LastModified := t1.DbModified

	// Task can now be retrieved by Id.
	t1Again, err := db.GetTaskById(t1.Id)
	assert.NoError(t, err)
	AssertDeepEqual(t, t1, t1Again)

	// Ensure that the task shows up in the modified list.
	tasks, err = db.GetModifiedTasks(id)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Task{t1}, tasks)

	// Ensure that the task shows up in the correct date ranges.
	t1Before := t1.Created
	t1After := t1Before.Add(1 * time.Nanosecond)
	timeEnd := now.Add(2 * time.Nanosecond)
	tasks, err = db.GetTasksFromDateRange(timeStart, t1Before, "")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))
	tasks, err = db.GetTasksFromDateRange(t1Before, t1After, "")
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Task{t1}, tasks)
	tasks, err = db.GetTasksFromDateRange(t1After, timeEnd, "")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))

	// Insert two more tasks. Ensure at least 1 nanosecond between task Created
	// times so that t1After != t2Before and t2After != t3Before.
	t2 := MakeTestTask(now.Add(time.Nanosecond), []string{"e", "f"})
	t3 := MakeTestTask(now.Add(2*time.Nanosecond), []string{"g", "h"})
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
	AssertDeepEqual(t, []*Task{t2, t3}, tasks)

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
	AssertDeepEqual(t, []*Task{t1, t2}, tasks)

	// Ensure that all tasks show up in the correct time ranges, in sorted order.
	t2Before := t2.Created
	t2After := t2Before.Add(1 * time.Nanosecond)

	t3Before := t3.Created
	t3After := t3Before.Add(1 * time.Nanosecond)

	timeEnd = now.Add(3 * time.Nanosecond)

	tasks, err = db.GetTasksFromDateRange(timeStart, t1Before, "")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))

	tasks, err = db.GetTasksFromDateRange(timeStart, t1After, "")
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Task{t1}, tasks)

	tasks, err = db.GetTasksFromDateRange(timeStart, t2Before, "")
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Task{t1}, tasks)

	tasks, err = db.GetTasksFromDateRange(timeStart, t2After, "")
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Task{t1, t2}, tasks)

	tasks, err = db.GetTasksFromDateRange(timeStart, t3Before, "")
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Task{t1, t2}, tasks)

	tasks, err = db.GetTasksFromDateRange(timeStart, t3After, "")
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Task{t1, t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(timeStart, timeEnd, "")
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Task{t1, t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(t1Before, timeEnd, "")
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Task{t1, t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(t1After, timeEnd, "")
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Task{t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(t2Before, timeEnd, "")
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Task{t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(t2After, timeEnd, "")
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Task{t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(t3Before, timeEnd, "")
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Task{t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(t3After, timeEnd, "")
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Task{}, tasks)
}

// Test that a TaskDB properly tracks its maximum number of users.
func TestTaskDBTooManyUsers(t testutils.TestingT, db TaskDB) {
	// Max out the number of modified-tasks users; ensure that we error out.
	for i := 0; i < MAX_MODIFIED_DATA_USERS; i++ {
		_, err := db.StartTrackingModifiedTasks()
		assert.NoError(t, err)
	}
	_, err := db.StartTrackingModifiedTasks()
	assert.True(t, IsTooManyUsers(err))
}

// Test that PutTask and PutTasks return ErrConcurrentUpdate when a cached Task
// has been updated in the DB.
func TestTaskDBConcurrentUpdate(t testutils.TestingT, db TaskDB) {
	// Insert a task.
	t1 := MakeTestTask(time.Now(), []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutTask(t1))

	// Retrieve a copy of the task.
	t1Cached, err := db.GetTaskById(t1.Id)
	assert.NoError(t, err)
	AssertDeepEqual(t, t1, t1Cached)

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
		AssertDeepEqual(t, t1, t1Again)
	}

	// Insert a second task.
	t2 := MakeTestTask(time.Now(), []string{"e", "f"})
	assert.NoError(t, db.PutTask(t2))

	// Update t2 at the same time as t1Cached; should still get an error.
	t2Before := t2.Copy()
	t2.Status = TASK_STATUS_MISHAP
	err = db.PutTasks([]*Task{t2, t1Cached})
	assert.True(t, IsConcurrentUpdate(err))

	{
		// DB should still have the old value of t1 and t2.
		t1Again, err := db.GetTaskById(t1.Id)
		assert.NoError(t, err)
		AssertDeepEqual(t, t1, t1Again)

		t2Again, err := db.GetTaskById(t2.Id)
		assert.NoError(t, err)
		AssertDeepEqual(t, t2Before, t2Again)
	}
}

// Test UpdateTasksWithRetries when no errors or retries.
func testUpdateTasksWithRetriesSimple(t testutils.TestingT, db TaskDB) {
	begin := time.Now()

	// Test no-op.
	tasks, err := UpdateTasksWithRetries(db, func() ([]*Task, error) {
		return nil, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))

	// Create new task t1. (UpdateTasksWithRetries isn't actually useful in this case.)
	tasks, err = UpdateTasksWithRetries(db, func() ([]*Task, error) {
		t1 := MakeTestTask(time.Time{}, []string{"a", "b", "c", "d"})
		assert.NoError(t, db.AssignId(t1))
		t1.Created = time.Now().Add(time.Nanosecond)
		return []*Task{t1}, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	t1 := tasks[0]

	// Update t1 and create t2.
	tasks, err = UpdateTasksWithRetries(db, func() ([]*Task, error) {
		t1, err := db.GetTaskById(t1.Id)
		assert.NoError(t, err)
		t1.Status = TASK_STATUS_RUNNING
		t2 := MakeTestTask(t1.Created.Add(time.Nanosecond), []string{"e", "f"})
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
	AssertDeepEqual(t, tasks[0], t1)
	AssertDeepEqual(t, tasks[1], t2)

	// Check no extra tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(begin, time.Now().Add(3*time.Nanosecond), "")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
	assert.Equal(t, t2.Id, tasks[1].Id)
}

// Test UpdateTasksWithRetries when there are some retries, but eventual success.
func testUpdateTasksWithRetriesSuccess(t testutils.TestingT, db TaskDB) {
	begin := time.Now()

	// Create and cache.
	t1 := MakeTestTask(begin.Add(time.Nanosecond), []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutTask(t1))
	t1Cached := t1.Copy()

	// Update original.
	t1.Status = TASK_STATUS_RUNNING
	assert.NoError(t, db.PutTask(t1))

	// Attempt update.
	callCount := 0
	tasks, err := UpdateTasksWithRetries(db, func() ([]*Task, error) {
		callCount++
		if callCount >= 3 {
			if task, err := db.GetTaskById(t1.Id); err != nil {
				return nil, err
			} else {
				t1Cached = task
			}
		}
		t1Cached.Status = TASK_STATUS_SUCCESS
		t2 := MakeTestTask(begin.Add(2*time.Nanosecond), []string{"e", "f"})
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
	AssertDeepEqual(t, tasks[0], t1)
	AssertDeepEqual(t, tasks[1], t2)

	// Check no extra tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(begin, time.Now().Add(3*time.Nanosecond), "")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
	assert.Equal(t, t2.Id, tasks[1].Id)
}

// Test UpdateTasksWithRetries when f returns an error.
func testUpdateTasksWithRetriesErrorInFunc(t testutils.TestingT, db TaskDB) {
	begin := time.Now()

	myErr := fmt.Errorf("NO! Bad dog!")
	callCount := 0
	tasks, err := UpdateTasksWithRetries(db, func() ([]*Task, error) {
		callCount++
		// Return a task just for fun.
		return []*Task{
			MakeTestTask(begin.Add(time.Nanosecond), []string{"a", "b", "c", "d"}),
		}, myErr
	})
	assert.Error(t, err)
	assert.Equal(t, myErr, err)
	assert.Equal(t, 0, len(tasks))
	assert.Equal(t, 1, callCount)

	// Check no tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(begin, time.Now().Add(2*time.Nanosecond), "")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))
}

// Test UpdateTasksWithRetries when PutTasks returns an error.
func testUpdateTasksWithRetriesErrorInPutTasks(t testutils.TestingT, db TaskDB) {
	begin := time.Now()

	callCount := 0
	tasks, err := UpdateTasksWithRetries(db, func() ([]*Task, error) {
		callCount++
		// Task has zero Created time.
		return []*Task{
			MakeTestTask(time.Time{}, []string{"a", "b", "c", "d"}),
		}, nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Created not set.")
	assert.Equal(t, 0, len(tasks))
	assert.Equal(t, 1, callCount)

	// Check no tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(begin, time.Now().Add(time.Nanosecond), "")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))
}

// Test UpdateTasksWithRetries when retries are exhausted.
func testUpdateTasksWithRetriesExhausted(t testutils.TestingT, db TaskDB) {
	begin := time.Now()

	// Create and cache.
	t1 := MakeTestTask(begin.Add(time.Nanosecond), []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutTask(t1))
	t1Cached := t1.Copy()

	// Update original.
	t1.Status = TASK_STATUS_RUNNING
	assert.NoError(t, db.PutTask(t1))

	// Attempt update.
	callCount := 0
	tasks, err := UpdateTasksWithRetries(db, func() ([]*Task, error) {
		callCount++
		t1Cached.Status = TASK_STATUS_SUCCESS
		t2 := MakeTestTask(begin.Add(2*time.Nanosecond), []string{"e", "f"})
		return []*Task{t1Cached, t2}, nil
	})
	assert.True(t, IsConcurrentUpdate(err))
	assert.Equal(t, NUM_RETRIES, callCount)
	assert.Equal(t, 0, len(tasks))

	// Check no extra tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(begin, time.Now().Add(3*time.Nanosecond), "")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
	assert.Equal(t, TASK_STATUS_RUNNING, tasks[0].Status)
}

// Test UpdateTaskWithRetries when no errors or retries.
func testUpdateTaskWithRetriesSimple(t testutils.TestingT, db TaskDB) {
	begin := time.Now()

	// Create new task t1.
	t1 := MakeTestTask(time.Time{}, []string{"a", "b", "c", "d"})
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
	AssertDeepEqual(t, t1Again, t1Updated)

	// Check no extra tasks in the TaskDB.
	tasks, err := db.GetTasksFromDateRange(begin, time.Now().Add(2*time.Nanosecond), "")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
}

// Test UpdateTaskWithRetries when there are some retries, but eventual success.
func testUpdateTaskWithRetriesSuccess(t testutils.TestingT, db TaskDB) {
	begin := time.Now()

	// Create new task t1.
	t1 := MakeTestTask(begin.Add(time.Nanosecond), []string{"a", "b", "c", "d"})
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
	AssertDeepEqual(t, t1Again, t1Updated)

	// Check no extra tasks in the DB.
	tasks, err := db.GetTasksFromDateRange(begin, time.Now().Add(2*time.Nanosecond), "")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
}

// Test UpdateTaskWithRetries when f returns an error.
func testUpdateTaskWithRetriesErrorInFunc(t testutils.TestingT, db TaskDB) {
	begin := time.Now()

	// Create new task t1.
	t1 := MakeTestTask(begin.Add(time.Nanosecond), []string{"a", "b", "c", "d"})
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
	AssertDeepEqual(t, t1, t1Again)

	// Check no extra tasks in the DB.
	tasks, err := db.GetTasksFromDateRange(begin, time.Now().Add(2*time.Nanosecond), "")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
}

// Test UpdateTaskWithRetries when retries are exhausted.
func testUpdateTaskWithRetriesExhausted(t testutils.TestingT, db TaskDB) {
	begin := time.Now()

	// Create new task t1.
	t1 := MakeTestTask(begin.Add(time.Nanosecond), []string{"a", "b", "c", "d"})
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
	AssertDeepEqual(t, t1, t1Again)

	// Check no extra tasks in the DB.
	tasks, err := db.GetTasksFromDateRange(begin, time.Now().Add(2*time.Nanosecond), "")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	assert.Equal(t, t1.Id, tasks[0].Id)
}

// Test UpdateTaskWithRetries when the given ID is not found in the DB.
func testUpdateTaskWithRetriesTaskNotFound(t testutils.TestingT, db TaskDB) {
	begin := time.Now()

	// Assign ID for a task, but don't put it in the DB.
	t1 := MakeTestTask(begin.Add(time.Nanosecond), []string{"a", "b", "c", "d"})
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
	tasks, err := db.GetTasksFromDateRange(begin, time.Now().Add(2*time.Nanosecond), "")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))
}

// Test UpdateTasksWithRetries and UpdateTaskWithRetries.
func TestUpdateTasksWithRetries(t testutils.TestingT, db TaskDB) {
	testUpdateTasksWithRetriesSimple(t, db)
	testUpdateTasksWithRetriesSuccess(t, db)
	testUpdateTasksWithRetriesErrorInFunc(t, db)
	testUpdateTasksWithRetriesErrorInPutTasks(t, db)
	testUpdateTasksWithRetriesExhausted(t, db)
	testUpdateTaskWithRetriesSimple(t, db)
	testUpdateTaskWithRetriesSuccess(t, db)
	testUpdateTaskWithRetriesErrorInFunc(t, db)
	testUpdateTaskWithRetriesExhausted(t, db)
	testUpdateTaskWithRetriesTaskNotFound(t, db)
}

// TestJobDB performs basic tests on an implementation of JobDB.
func TestJobDB(t testutils.TestingT, db JobDB) {
	_, err := db.GetModifiedJobs("dummy-id")
	assert.True(t, IsUnknownId(err))

	id, err := db.StartTrackingModifiedJobs()
	assert.NoError(t, err)

	jobs, err := db.GetModifiedJobs(id)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(jobs))

	now := time.Now().Add(time.Nanosecond)
	j1 := makeJob(now)

	// Insert the job.
	assert.NoError(t, db.PutJob(j1))

	// Ids must be URL-safe.
	assert.NotEqual(t, "", j1.Id)
	assert.Equal(t, url.QueryEscape(j1.Id), j1.Id)

	// Check that DbModified was set.
	assert.False(t, util.TimeIsZero(j1.DbModified))
	j1LastModified := j1.DbModified

	// Job can now be retrieved by Id.
	j1Again, err := db.GetJobById(j1.Id)
	assert.NoError(t, err)
	AssertDeepEqual(t, j1, j1Again)

	// Ensure that the job shows up in the modified list.
	jobs, err = db.GetModifiedJobs(id)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j1}, jobs)

	// Ensure that the job shows up in the correct date ranges.
	timeStart := time.Time{}
	j1Before := j1.Created
	j1After := j1Before.Add(1 * time.Nanosecond)
	timeEnd := now.Add(2 * time.Nanosecond)
	jobs, err = db.GetJobsFromDateRange(timeStart, j1Before)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(jobs))
	jobs, err = db.GetJobsFromDateRange(j1Before, j1After)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j1}, jobs)
	jobs, err = db.GetJobsFromDateRange(j1After, timeEnd)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(jobs))

	// Insert two more jobs. Ensure at least 1 nanosecond between job Created
	// times so that j1After != j2Before and j2After != j3Before.
	j2 := makeJob(now.Add(time.Nanosecond))
	j3 := makeJob(now.Add(2 * time.Nanosecond))
	assert.NoError(t, db.PutJobs([]*Job{j2, j3}))

	// Check that PutJobs assigned Ids.
	assert.NotEqual(t, "", j2.Id)
	assert.NotEqual(t, "", j3.Id)
	// Ids must be URL-safe.
	assert.Equal(t, url.QueryEscape(j2.Id), j2.Id)
	assert.Equal(t, url.QueryEscape(j3.Id), j3.Id)

	// Ensure that both jobs show up in the modified list.
	jobs, err = db.GetModifiedJobs(id)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j2, j3}, jobs)

	// Make an update to j1 and j2. Ensure modified times change.
	j2LastModified := j2.DbModified
	j1.Status = JOB_STATUS_IN_PROGRESS
	j2.Status = JOB_STATUS_SUCCESS
	assert.NoError(t, db.PutJobs([]*Job{j1, j2}))
	assert.False(t, j1.DbModified.Equal(j1LastModified))
	assert.False(t, j2.DbModified.Equal(j2LastModified))

	// Ensure that both jobs show up in the modified list.
	jobs, err = db.GetModifiedJobs(id)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j1, j2}, jobs)

	// Ensure that all jobs show up in the correct time ranges, in sorted order.
	j2Before := j2.Created
	j2After := j2Before.Add(1 * time.Nanosecond)

	j3Before := j3.Created
	j3After := j3Before.Add(1 * time.Nanosecond)

	timeEnd = now.Add(3 * time.Nanosecond)

	jobs, err = db.GetJobsFromDateRange(timeStart, j1Before)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(jobs))

	jobs, err = db.GetJobsFromDateRange(timeStart, j1After)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j1}, jobs)

	jobs, err = db.GetJobsFromDateRange(timeStart, j2Before)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j1}, jobs)

	jobs, err = db.GetJobsFromDateRange(timeStart, j2After)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j1, j2}, jobs)

	jobs, err = db.GetJobsFromDateRange(timeStart, j3Before)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j1, j2}, jobs)

	jobs, err = db.GetJobsFromDateRange(timeStart, j3After)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j1, j2, j3}, jobs)

	jobs, err = db.GetJobsFromDateRange(timeStart, timeEnd)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j1, j2, j3}, jobs)

	jobs, err = db.GetJobsFromDateRange(j1Before, timeEnd)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j1, j2, j3}, jobs)

	jobs, err = db.GetJobsFromDateRange(j1After, timeEnd)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j2, j3}, jobs)

	jobs, err = db.GetJobsFromDateRange(j2Before, timeEnd)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j2, j3}, jobs)

	jobs, err = db.GetJobsFromDateRange(j2After, timeEnd)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j3}, jobs)

	jobs, err = db.GetJobsFromDateRange(j3Before, timeEnd)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{j3}, jobs)

	jobs, err = db.GetJobsFromDateRange(j3After, timeEnd)
	assert.NoError(t, err)
	AssertDeepEqual(t, []*Job{}, jobs)
}

// Test that a JobDB properly tracks its maximum number of users.
func TestJobDBTooManyUsers(t testutils.TestingT, db JobDB) {
	// Max out the number of modified-jobs users; ensure that we error out.
	for i := 0; i < MAX_MODIFIED_DATA_USERS; i++ {
		_, err := db.StartTrackingModifiedJobs()
		assert.NoError(t, err)
	}
	_, err := db.StartTrackingModifiedJobs()
	assert.True(t, IsTooManyUsers(err))
}

// Test that PutJob and PutJobs return ErrConcurrentUpdate when a cached Job
// has been updated in the DB.
func TestJobDBConcurrentUpdate(t testutils.TestingT, db JobDB) {
	// Insert a job.
	j1 := makeJob(time.Now())
	assert.NoError(t, db.PutJob(j1))

	// Retrieve a copy of the job.
	j1Cached, err := db.GetJobById(j1.Id)
	assert.NoError(t, err)
	AssertDeepEqual(t, j1, j1Cached)

	// Update the original job.
	j1.Repo = "another-repo"
	assert.NoError(t, db.PutJob(j1))

	// Update the cached copy; should get concurrent update error.
	j1Cached.Status = JOB_STATUS_IN_PROGRESS
	err = db.PutJob(j1Cached)
	assert.True(t, IsConcurrentUpdate(err))

	{
		// DB should still have the old value of j1.
		j1Again, err := db.GetJobById(j1.Id)
		assert.NoError(t, err)
		AssertDeepEqual(t, j1, j1Again)
	}

	// Insert a second job.
	j2 := makeJob(time.Now())
	assert.NoError(t, db.PutJob(j2))

	// Update j2 at the same time as j1Cached; should still get an error.
	j2Before := j2.Copy()
	j2.Status = JOB_STATUS_MISHAP
	err = db.PutJobs([]*Job{j2, j1Cached})
	assert.True(t, IsConcurrentUpdate(err))

	{
		// DB should still have the old value of j1 and j2.
		j1Again, err := db.GetJobById(j1.Id)
		assert.NoError(t, err)
		AssertDeepEqual(t, j1, j1Again)

		j2Again, err := db.GetJobById(j2.Id)
		assert.NoError(t, err)
		AssertDeepEqual(t, j2Before, j2Again)
	}
}

// Test UpdateJobsWithRetries when no errors or retries.
func testUpdateJobsWithRetriesSimple(t testutils.TestingT, db JobDB) {
	begin := time.Now()

	// Test no-op.
	jobs, err := UpdateJobsWithRetries(db, func() ([]*Job, error) {
		return nil, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(jobs))

	// Create new job j1. (UpdateJobsWithRetries isn't actually useful in this case.)
	jobs, err = UpdateJobsWithRetries(db, func() ([]*Job, error) {
		j1 := makeJob(time.Time{})
		j1.Created = time.Now().Add(time.Nanosecond)
		return []*Job{j1}, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(jobs))
	j1 := jobs[0]

	// Update j1 and create j2.
	jobs, err = UpdateJobsWithRetries(db, func() ([]*Job, error) {
		j1, err := db.GetJobById(j1.Id)
		assert.NoError(t, err)
		j1.Status = JOB_STATUS_IN_PROGRESS
		j2 := makeJob(j1.Created.Add(time.Nanosecond))
		j2.Repo = "j2-repo"
		return []*Job{j1, j2}, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(jobs))
	assert.Equal(t, j1.Id, jobs[0].Id)
	assert.Equal(t, JOB_STATUS_IN_PROGRESS, jobs[0].Status)
	assert.Equal(t, "j2-repo", jobs[1].Repo)

	// Check that return value matches what's in the DB.
	j1, err = db.GetJobById(j1.Id)
	assert.NoError(t, err)
	j2, err := db.GetJobById(jobs[1].Id)
	assert.NoError(t, err)
	AssertDeepEqual(t, jobs[0], j1)
	AssertDeepEqual(t, jobs[1], j2)

	// Check no extra jobs in the DB.
	jobs, err = db.GetJobsFromDateRange(begin, time.Now().Add(3*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(jobs))
	assert.Equal(t, j1.Id, jobs[0].Id)
	assert.Equal(t, j2.Id, jobs[1].Id)
}

// Test UpdateJobsWithRetries when there are some retries, but eventual success.
func testUpdateJobsWithRetriesSuccess(t testutils.TestingT, db JobDB) {
	begin := time.Now()

	// Create and cache.
	j1 := makeJob(begin.Add(time.Nanosecond))
	assert.NoError(t, db.PutJob(j1))
	j1Cached := j1.Copy()

	// Update original.
	j1.Status = JOB_STATUS_IN_PROGRESS
	assert.NoError(t, db.PutJob(j1))

	// Attempt update.
	callCount := 0
	jobs, err := UpdateJobsWithRetries(db, func() ([]*Job, error) {
		callCount++
		if callCount >= 3 {
			if job, err := db.GetJobById(j1.Id); err != nil {
				return nil, err
			} else {
				j1Cached = job
			}
		}
		j1Cached.Status = JOB_STATUS_SUCCESS
		j2 := makeJob(begin.Add(2 * time.Nanosecond))
		j2.Repo = "j2-repo"
		return []*Job{j1Cached, j2}, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
	assert.Equal(t, 2, len(jobs))
	assert.Equal(t, j1.Id, jobs[0].Id)
	assert.Equal(t, JOB_STATUS_SUCCESS, jobs[0].Status)
	assert.Equal(t, "j2-repo", jobs[1].Repo)

	// Check that return value matches what's in the DB.
	j1, err = db.GetJobById(j1.Id)
	assert.NoError(t, err)
	j2, err := db.GetJobById(jobs[1].Id)
	assert.NoError(t, err)
	AssertDeepEqual(t, jobs[0], j1)
	AssertDeepEqual(t, jobs[1], j2)

	// Check no extra jobs in the DB.
	jobs, err = db.GetJobsFromDateRange(begin, time.Now().Add(3*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(jobs))
	assert.Equal(t, j1.Id, jobs[0].Id)
	assert.Equal(t, j2.Id, jobs[1].Id)
}

// Test UpdateJobsWithRetries when f returns an error.
func testUpdateJobsWithRetriesErrorInFunc(t testutils.TestingT, db JobDB) {
	begin := time.Now()

	myErr := fmt.Errorf("NO! Bad dog!")
	callCount := 0
	jobs, err := UpdateJobsWithRetries(db, func() ([]*Job, error) {
		callCount++
		// Return a job just for fun.
		return []*Job{
			makeJob(begin.Add(time.Nanosecond)),
		}, myErr
	})
	assert.Error(t, err)
	assert.Equal(t, myErr, err)
	assert.Equal(t, 0, len(jobs))
	assert.Equal(t, 1, callCount)

	// Check no jobs in the DB.
	jobs, err = db.GetJobsFromDateRange(begin, time.Now().Add(2*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 0, len(jobs))
}

// Test UpdateJobsWithRetries when PutJobs returns an error.
func testUpdateJobsWithRetriesErrorInPutJobs(t testutils.TestingT, db JobDB) {
	begin := time.Now()

	callCount := 0
	jobs, err := UpdateJobsWithRetries(db, func() ([]*Job, error) {
		callCount++
		// Job has zero Created time.
		return []*Job{
			makeJob(time.Time{}),
		}, nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Created not set.")
	assert.Equal(t, 0, len(jobs))
	assert.Equal(t, 1, callCount)

	// Check no jobs in the DB.
	jobs, err = db.GetJobsFromDateRange(begin, time.Now().Add(time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 0, len(jobs))
}

// Test UpdateJobsWithRetries when retries are exhausted.
func testUpdateJobsWithRetriesExhausted(t testutils.TestingT, db JobDB) {
	begin := time.Now()

	// Create and cache.
	j1 := makeJob(begin.Add(time.Nanosecond))
	assert.NoError(t, db.PutJob(j1))
	j1Cached := j1.Copy()

	// Update original.
	j1.Status = JOB_STATUS_IN_PROGRESS
	assert.NoError(t, db.PutJob(j1))

	// Attempt update.
	callCount := 0
	jobs, err := UpdateJobsWithRetries(db, func() ([]*Job, error) {
		callCount++
		j1Cached.Status = JOB_STATUS_SUCCESS
		j2 := makeJob(begin.Add(2 * time.Nanosecond))
		return []*Job{j1Cached, j2}, nil
	})
	assert.True(t, IsConcurrentUpdate(err))
	assert.Equal(t, NUM_RETRIES, callCount)
	assert.Equal(t, 0, len(jobs))

	// Check no extra jobs in the DB.
	jobs, err = db.GetJobsFromDateRange(begin, time.Now().Add(3*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(jobs))
	assert.Equal(t, j1.Id, jobs[0].Id)
	assert.Equal(t, JOB_STATUS_IN_PROGRESS, jobs[0].Status)
}

// Test UpdateJobWithRetries when no errors or retries.
func testUpdateJobWithRetriesSimple(t testutils.TestingT, db JobDB) {
	begin := time.Now()

	// Create new job j1.
	j1 := makeJob(time.Now())
	assert.NoError(t, db.PutJob(j1))

	// Update j1.
	j1Updated, err := UpdateJobWithRetries(db, j1.Id, func(job *Job) error {
		job.Status = JOB_STATUS_IN_PROGRESS
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, j1.Id, j1Updated.Id)
	assert.Equal(t, JOB_STATUS_IN_PROGRESS, j1Updated.Status)
	assert.NotEqual(t, j1.DbModified, j1Updated.DbModified)

	// Check that return value matches what's in the DB.
	j1Again, err := db.GetJobById(j1.Id)
	assert.NoError(t, err)
	AssertDeepEqual(t, j1Again, j1Updated)

	// Check no extra jobs in the JobDB.
	jobs, err := db.GetJobsFromDateRange(begin, time.Now().Add(2*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(jobs))
	assert.Equal(t, j1.Id, jobs[0].Id)
}

// Test UpdateJobWithRetries when there are some retries, but eventual success.
func testUpdateJobWithRetriesSuccess(t testutils.TestingT, db JobDB) {
	begin := time.Now()

	// Create new job j1.
	j1 := makeJob(begin.Add(time.Nanosecond))
	assert.NoError(t, db.PutJob(j1))

	// Attempt update.
	callCount := 0
	j1Updated, err := UpdateJobWithRetries(db, j1.Id, func(job *Job) error {
		callCount++
		if callCount < 3 {
			// Sneakily make an update in the background.
			j1.Repo = "some-other-repo.git"
			assert.NoError(t, db.PutJob(j1))
		}
		job.Status = JOB_STATUS_SUCCESS
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
	assert.Equal(t, j1.Id, j1Updated.Id)
	assert.Equal(t, JOB_STATUS_SUCCESS, j1Updated.Status)

	// Check that return value matches what's in the DB.
	j1Again, err := db.GetJobById(j1.Id)
	assert.NoError(t, err)
	AssertDeepEqual(t, j1Again, j1Updated)

	// Check no extra jobs in the DB.
	jobs, err := db.GetJobsFromDateRange(begin, time.Now().Add(2*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(jobs))
	assert.Equal(t, j1.Id, jobs[0].Id)
}

// Test UpdateJobWithRetries when f returns an error.
func testUpdateJobWithRetriesErrorInFunc(t testutils.TestingT, db JobDB) {
	begin := time.Now()

	// Create new job j1.
	j1 := makeJob(begin.Add(time.Nanosecond))
	assert.NoError(t, db.PutJob(j1))

	// Update and return an error.
	myErr := fmt.Errorf("Um, actually, I didn't want to update that job.")
	callCount := 0
	noJob, err := UpdateJobWithRetries(db, j1.Id, func(job *Job) error {
		callCount++
		// Update job to test nothing changes in DB.
		job.Status = JOB_STATUS_IN_PROGRESS
		return myErr
	})
	assert.Error(t, err)
	assert.Equal(t, myErr, err)
	assert.Nil(t, noJob)
	assert.Equal(t, 1, callCount)

	// Check job did not change in the DB.
	j1Again, err := db.GetJobById(j1.Id)
	assert.NoError(t, err)
	AssertDeepEqual(t, j1, j1Again)

	// Check no extra jobs in the DB.
	jobs, err := db.GetJobsFromDateRange(begin, time.Now().Add(2*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(jobs))
	assert.Equal(t, j1.Id, jobs[0].Id)
}

// Test UpdateJobWithRetries when retries are exhausted.
func testUpdateJobWithRetriesExhausted(t testutils.TestingT, db JobDB) {
	begin := time.Now()

	// Create new job j1.
	j1 := makeJob(begin.Add(time.Nanosecond))
	assert.NoError(t, db.PutJob(j1))

	// Update original.
	j1.Status = JOB_STATUS_IN_PROGRESS
	assert.NoError(t, db.PutJob(j1))

	// Attempt update.
	callCount := 0
	noJob, err := UpdateJobWithRetries(db, j1.Id, func(job *Job) error {
		callCount++
		// Sneakily make an update in the background.
		j1.Repo = "some-other-repo"
		assert.NoError(t, db.PutJob(j1))

		job.Status = JOB_STATUS_SUCCESS
		return nil
	})
	assert.True(t, IsConcurrentUpdate(err))
	assert.Equal(t, NUM_RETRIES, callCount)
	assert.Nil(t, noJob)

	// Check job did not change in the DB.
	j1Again, err := db.GetJobById(j1.Id)
	assert.NoError(t, err)
	AssertDeepEqual(t, j1, j1Again)

	// Check no extra jobs in the DB.
	jobs, err := db.GetJobsFromDateRange(begin, time.Now().Add(2*time.Nanosecond))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(jobs))
	assert.Equal(t, j1.Id, jobs[0].Id)
}

// Test UpdateJobsWithRetries and UpdateJobWithRetries.
func TestUpdateJobsWithRetries(t testutils.TestingT, db JobDB) {
	testUpdateJobsWithRetriesSimple(t, db)
	testUpdateJobsWithRetriesSuccess(t, db)
	testUpdateJobsWithRetriesErrorInFunc(t, db)
	testUpdateJobsWithRetriesErrorInPutJobs(t, db)
	testUpdateJobsWithRetriesExhausted(t, db)
	testUpdateJobWithRetriesSimple(t, db)
	testUpdateJobWithRetriesSuccess(t, db)
	testUpdateJobWithRetriesErrorInFunc(t, db)
	testUpdateJobWithRetriesExhausted(t, db)
}

// makeTaskComment creates a comment with its ID fields based on the given repo,
// name, commit, and ts, and other fields based on n.
func makeTaskComment(n int, repo int, name int, commit int, ts time.Time) *TaskComment {
	return &TaskComment{
		Repo:      fmt.Sprintf("r%d", repo),
		Revision:  fmt.Sprintf("c%d", commit),
		Name:      fmt.Sprintf("n%d", name),
		Timestamp: ts,
		TaskId:    fmt.Sprintf("id%d", n),
		User:      fmt.Sprintf("u%d", n),
		Message:   fmt.Sprintf("m%d", n),
	}
}

// makeTaskSpecComment creates a comment with its ID fields based on the given
// repo, name, and ts, and other fields based on n.
func makeTaskSpecComment(n int, repo int, name int, ts time.Time) *TaskSpecComment {
	return &TaskSpecComment{
		Repo:          fmt.Sprintf("r%d", repo),
		Name:          fmt.Sprintf("n%d", name),
		Timestamp:     ts,
		User:          fmt.Sprintf("u%d", n),
		Flaky:         n%2 == 0,
		IgnoreFailure: n>>1%2 == 0,
		Message:       fmt.Sprintf("m%d", n),
	}
}

// makeCommitComment creates a comment with its ID fields based on the given
// repo, commit, and ts, and other fields based on n.
func makeCommitComment(n int, repo int, commit int, ts time.Time) *CommitComment {
	return &CommitComment{
		Repo:          fmt.Sprintf("r%d", repo),
		Revision:      fmt.Sprintf("c%d", commit),
		Timestamp:     ts,
		User:          fmt.Sprintf("u%d", n),
		IgnoreFailure: n>>1%2 == 0,
		Message:       fmt.Sprintf("m%d", n),
	}
}

// TestCommentDB validates that db correctly implements the CommentDB interface.
func TestCommentDB(t testutils.TestingT, db CommentDB) {
	now := time.Now()

	// Empty db.
	{
		actual, err := db.GetCommentsForRepos([]string{"r0", "r1", "r2"}, now.Add(-10000*time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, 3, len(actual))
		assert.Equal(t, "r0", actual[0].Repo)
		assert.Equal(t, "r1", actual[1].Repo)
		assert.Equal(t, "r2", actual[2].Repo)
		for _, rc := range actual {
			assert.Equal(t, 0, len(rc.TaskComments))
			assert.Equal(t, 0, len(rc.TaskSpecComments))
			assert.Equal(t, 0, len(rc.CommitComments))
		}
	}

	// Add some comments.
	tc1 := makeTaskComment(1, 1, 1, 1, now)
	tc2 := makeTaskComment(2, 1, 1, 1, now.Add(2*time.Second))
	tc3 := makeTaskComment(3, 1, 1, 1, now.Add(time.Second))
	tc4 := makeTaskComment(4, 1, 1, 2, now)
	tc5 := makeTaskComment(5, 1, 2, 2, now)
	tc6 := makeTaskComment(6, 2, 3, 3, now)
	tc6copy := tc6.Copy() // Adding identical comment should be ignored.
	for _, c := range []*TaskComment{tc1, tc2, tc3, tc4, tc5, tc6, tc6copy} {
		assert.NoError(t, db.PutTaskComment(c))
	}
	tc6.Message = "modifying after Put shouldn't affect stored comment"

	sc1 := makeTaskSpecComment(1, 1, 1, now)
	sc2 := makeTaskSpecComment(2, 1, 1, now.Add(2*time.Second))
	sc3 := makeTaskSpecComment(3, 1, 1, now.Add(time.Second))
	sc4 := makeTaskSpecComment(4, 1, 2, now)
	sc5 := makeTaskSpecComment(5, 2, 3, now)
	sc5copy := sc5.Copy() // Adding identical comment should be ignored.
	for _, c := range []*TaskSpecComment{sc1, sc2, sc3, sc4, sc5, sc5copy} {
		assert.NoError(t, db.PutTaskSpecComment(c))
	}
	sc5.Message = "modifying after Put shouldn't affect stored comment"

	cc1 := makeCommitComment(1, 1, 1, now)
	cc2 := makeCommitComment(2, 1, 1, now.Add(2*time.Second))
	cc3 := makeCommitComment(3, 1, 1, now.Add(time.Second))
	cc4 := makeCommitComment(4, 1, 2, now)
	cc5 := makeCommitComment(5, 2, 3, now)
	cc5copy := cc5.Copy() // Adding identical comment should be ignored.
	for _, c := range []*CommitComment{cc1, cc2, cc3, cc4, cc5, cc5copy} {
		assert.NoError(t, db.PutCommitComment(c))
	}
	cc5.Message = "modifying after Put shouldn't affect stored comment"

	// Check that adding duplicate non-identical comment gives an error.
	tc1different := tc1.Copy()
	tc1different.Message = "not the same"
	assert.True(t, IsAlreadyExists(db.PutTaskComment(tc1different)))
	sc1different := sc1.Copy()
	sc1different.Message = "not the same"
	assert.True(t, IsAlreadyExists(db.PutTaskSpecComment(sc1different)))
	cc1different := cc1.Copy()
	cc1different.Message = "not the same"
	assert.True(t, IsAlreadyExists(db.PutCommitComment(cc1different)))

	expected := []*RepoComments{
		{Repo: "r0"},
		{
			Repo: "r1",
			TaskComments: map[string]map[string][]*TaskComment{
				"c1": {
					"n1": {tc1, tc3, tc2},
				},
				"c2": {
					"n1": {tc4},
					"n2": {tc5},
				},
			},
			TaskSpecComments: map[string][]*TaskSpecComment{
				"n1": {sc1, sc3, sc2},
				"n2": {sc4},
			},
			CommitComments: map[string][]*CommitComment{
				"c1": {cc1, cc3, cc2},
				"c2": {cc4},
			},
		},
		{
			Repo: "r2",
			TaskComments: map[string]map[string][]*TaskComment{
				"c3": {
					"n3": {tc6copy},
				},
			},
			TaskSpecComments: map[string][]*TaskSpecComment{
				"n3": {sc5copy},
			},
			CommitComments: map[string][]*CommitComment{
				"c3": {cc5copy},
			},
		},
	}
	{
		actual, err := db.GetCommentsForRepos([]string{"r0", "r1", "r2"}, now.Add(-10000*time.Hour))
		assert.NoError(t, err)
		AssertDeepEqual(t, expected, actual)
	}

	// Specifying a cutoff time shouldn't drop required comments.
	{
		actual, err := db.GetCommentsForRepos([]string{"r1"}, now.Add(time.Second))
		assert.NoError(t, err)
		assert.Equal(t, 1, len(actual))
		{
			tcs := actual[0].TaskComments["c1"]["n1"]
			assert.True(t, len(tcs) >= 2)
			offset := 0
			if !tcs[0].Timestamp.Equal(tc3.Timestamp) {
				offset = 1
			}
			AssertDeepEqual(t, tc3, tcs[offset])
			AssertDeepEqual(t, tc2, tcs[offset+1])
		}
		{
			scs := actual[0].TaskSpecComments["n1"]
			assert.True(t, len(scs) >= 2)
			offset := 0
			if !scs[0].Timestamp.Equal(sc3.Timestamp) {
				offset = 1
			}
			AssertDeepEqual(t, sc3, scs[offset])
			AssertDeepEqual(t, sc2, scs[offset+1])
		}
		{
			ccs := actual[0].CommitComments["c1"]
			assert.True(t, len(ccs) >= 2)
			offset := 0
			if !ccs[0].Timestamp.Equal(cc3.Timestamp) {
				offset = 1
			}
			AssertDeepEqual(t, cc3, ccs[offset])
			AssertDeepEqual(t, cc2, ccs[offset+1])
		}
	}

	// Delete some comments.
	assert.NoError(t, db.DeleteTaskComment(tc3))
	assert.NoError(t, db.DeleteTaskSpecComment(sc3))
	assert.NoError(t, db.DeleteCommitComment(cc3))
	// Delete should only look at the ID fields.
	assert.NoError(t, db.DeleteTaskComment(tc1different))
	assert.NoError(t, db.DeleteTaskSpecComment(sc1different))
	assert.NoError(t, db.DeleteCommitComment(cc1different))
	// Delete of nonexistent task should succeed.
	assert.NoError(t, db.DeleteTaskComment(makeTaskComment(99, 1, 1, 1, now.Add(99*time.Second))))
	assert.NoError(t, db.DeleteTaskComment(makeTaskComment(99, 1, 1, 99, now)))
	assert.NoError(t, db.DeleteTaskComment(makeTaskComment(99, 1, 99, 1, now)))
	assert.NoError(t, db.DeleteTaskComment(makeTaskComment(99, 99, 1, 1, now)))
	assert.NoError(t, db.DeleteTaskSpecComment(makeTaskSpecComment(99, 1, 1, now.Add(99*time.Second))))
	assert.NoError(t, db.DeleteTaskSpecComment(makeTaskSpecComment(99, 1, 99, now)))
	assert.NoError(t, db.DeleteTaskSpecComment(makeTaskSpecComment(99, 99, 1, now)))
	assert.NoError(t, db.DeleteCommitComment(makeCommitComment(99, 1, 1, now.Add(99*time.Second))))
	assert.NoError(t, db.DeleteCommitComment(makeCommitComment(99, 1, 99, now)))
	assert.NoError(t, db.DeleteCommitComment(makeCommitComment(99, 99, 1, now)))

	expected[1].TaskComments["c1"]["n1"] = []*TaskComment{tc2}
	expected[1].TaskSpecComments["n1"] = []*TaskSpecComment{sc2}
	expected[1].CommitComments["c1"] = []*CommitComment{cc2}
	{
		actual, err := db.GetCommentsForRepos([]string{"r0", "r1", "r2"}, now.Add(-10000*time.Hour))
		assert.NoError(t, err)
		AssertDeepEqual(t, expected, actual)
	}

	// Delete all the comments.
	for _, c := range []*TaskComment{tc2, tc4, tc5, tc6} {
		assert.NoError(t, db.DeleteTaskComment(c))
	}
	for _, c := range []*TaskSpecComment{sc2, sc4, sc5} {
		assert.NoError(t, db.DeleteTaskSpecComment(c))
	}
	for _, c := range []*CommitComment{cc2, cc4, cc5} {
		assert.NoError(t, db.DeleteCommitComment(c))
	}
	{
		actual, err := db.GetCommentsForRepos([]string{"r0", "r1", "r2"}, now.Add(-10000*time.Hour))
		assert.NoError(t, err)
		assert.Equal(t, 3, len(actual))
		assert.Equal(t, "r0", actual[0].Repo)
		assert.Equal(t, "r1", actual[1].Repo)
		assert.Equal(t, "r2", actual[2].Repo)
		for _, rc := range actual {
			assert.Equal(t, 0, len(rc.TaskComments))
			assert.Equal(t, 0, len(rc.TaskSpecComments))
			assert.Equal(t, 0, len(rc.CommitComments))
		}
	}
}

func DummyGetRevisionTimestamp(ts time.Time) GetRevisionTimestamp {
	return func(string, string) (time.Time, error) { return ts, nil }
}

func TestTaskDBGetTasksFromDateRangeByRepo(t testutils.TestingT, db TaskDB) {
	r1 := common.REPO_SKIA
	r2 := common.REPO_SKIA_INFRA
	r3 := common.REPO_CHROMIUM
	repos := []string{r1, r2, r3}
	start := time.Now().Add(-50 * time.Nanosecond)
	end := start
	for _, repo := range repos {
		for i := 0; i < 10; i++ {
			task := MakeTestTask(end, []string{"c"})
			task.Repo = repo
			assert.NoError(t, db.PutTask(task))
			end = end.Add(time.Nanosecond)
		}
	}
	tasks, err := db.GetTasksFromDateRange(start, end, "")
	assert.NoError(t, err)
	assert.Equal(t, 30, len(tasks))
	assert.True(t, sort.IsSorted(TaskSlice(tasks)))
	for _, repo := range repos {
		tasks, err := db.GetTasksFromDateRange(start, end, repo)
		assert.NoError(t, err)
		assert.Equal(t, 10, len(tasks))
		assert.True(t, sort.IsSorted(TaskSlice(tasks)))
		for _, task := range tasks {
			assert.Equal(t, repo, task.Repo)
		}
	}
}

func TestTaskDBGetTasksFromWindow(t testutils.TestingT, db TaskDB) {
	now := time.Now()
	timeWindow := 24 * time.Hour
	// Offset commit timestamps for different repos to ensure that we get
	// a consistent sorting order.
	repoOffset := time.Minute
	curOffset := repoOffset
	f := "somefile"
	setup := func(numCommits int) (string, *repograph.Graph, func()) {
		ctx := context.Background()
		gb := git_testutils.GitInit(t, ctx)
		repoUrl := gb.RepoUrl()
		t0 := now.Add(-timeWindow).Add(curOffset)
		for i := 0; i < numCommits; i++ {
			ts := t0.Add(time.Duration(i) * timeWindow / time.Duration(numCommits))
			gb.AddGen(ctx, f)
			hash := gb.CommitMsgAt(ctx, fmt.Sprintf("Commit %d", i), ts)
			task := MakeTestTask(ts, []string{hash})
			task.Repo = repoUrl
			assert.NoError(t, db.PutTask(task))
		}
		tmp, err := ioutil.TempDir("", "")
		assert.NoError(t, err)
		repo, err := repograph.NewGraph(ctx, gb.Dir(), tmp)
		assert.NoError(t, err)
		assert.NoError(t, repo.Update(ctx))
		curOffset += repoOffset
		return repoUrl, repo, func() {
			gb.Cleanup()
			testutils.RemoveAll(t, tmp)
		}
	}
	url1, r1, cleanup1 := setup(5)
	defer cleanup1()
	url2, r2, cleanup2 := setup(10)
	defer cleanup2()
	url3, r3, cleanup3 := setup(20)
	defer cleanup3()

	test := func(windowSize time.Duration, numCommits int, repos repograph.Map, expectTasks int) {
		w, err := window.New(windowSize, numCommits, repos)
		assert.NoError(t, err)
		tasks, err := GetTasksFromWindow(db, w, now)
		assert.NoError(t, err)
		assert.Equal(t, expectTasks, len(tasks))
		assert.True(t, sort.IsSorted(TaskSlice(tasks)))

		// Verify that TaskCache behaves correctly too.
		cache, err := NewTaskCache(db, w)
		assert.NoError(t, err)
		tasks2, err := cache.GetTasksFromDateRange(time.Time{}, now)
		assert.NoError(t, err)
		assert.Equal(t, expectTasks, len(tasks2))
		assert.True(t, sort.IsSorted(TaskSlice(tasks2)))
		deepequal.AssertDeepEqual(t, tasks, tasks2)
	}

	// Test 1: No repos in window. Only the timeWindow matters in this case,
	// and since tasks from all repos have been inserted into the DB, the
	// cache will return tasks from all repos within the time window, even
	// though the window.Window instance doesn't know anything about the
	// repos.
	repos := repograph.Map{}
	test(timeWindow, 0, repos, 35)
	test(time.Duration(0), 5, repos, 0)
	test(timeWindow, 100, repos, 35)

	// Test 2: One repo in window. Now, the greater of the timeWindow or the
	// window containing the last numCommits wins. Note that with at least
	// one repo specified to the window, we now exclude tasks which are not
	// in that repo from the TaskCache.
	repos[url1] = r1
	test(timeWindow, 5, repos, 5)
	test(time.Duration(0), 5, repos, 5)
	test(timeWindow, 2, repos, 5)

	// Test 3: Two repos. This is the same as #2 in that the greater of
	// timeWindow or the window containing the last numCommits wins, for
	// each repo. When timeWindow is sufficiently small, we get the last
	// numCommits from each repo, even that implies different time windows
	// for each.
	repos[url2] = r2
	test(timeWindow, 5, repos, 15)
	test(time.Duration(0), 5, repos, 10)
	test(time.Duration(0), 20, repos, 15)

	// Test 4: Three repos.
	repos[url3] = r3
	test(timeWindow, 0, repos, 35)
	test(time.Duration(0), 100, repos, 35)
	test(time.Duration(0), 3, repos, 9)
}
