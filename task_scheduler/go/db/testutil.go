package db

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

const (
	TS_RESOLUTION = time.Microsecond
)

// AssertDeepEqual does a deep equals comparison using the testutils.TestingT interface.
//
// Callers of these tests utils should assign a value to AssertDeepEqual beforehand, e.g.:
//
//	AssertDeepEqual = assertdeep.Equal
//
// This is necessary to break the hard linking of this file to the "testing" module.
var AssertDeepEqual func(t sktest.TestingT, expected, actual interface{})

// findModifiedTasks asserts that GetModifiedTasks returns at least the given
// expected set of tasks.
func findModifiedTasks(t sktest.TestingT, m <-chan []*types.Task, expect ...*types.Task) {
	expectMap := make(map[string]*types.Task, len(expect))
	for _, e := range expect {
		expectMap[e.Id] = e
	}
	actualMap := make(map[string]*types.Task, len(expectMap))
	require.Eventually(t, func() bool {
		tasks := <-m
		for _, a := range tasks {
			// Ignore tasks not in the expected list.
			if _, ok := expectMap[a.Id]; !ok {
				continue
			}
			exist, ok := actualMap[a.Id]
			if !ok || a.DbModified.After(exist.DbModified) {
				actualMap[a.Id] = a
			}
		}
		if len(actualMap) != len(expectMap) {
			sklog.Debugf("  want %d but have %d", len(expectMap), len(actualMap))
			return false
		}
		assertdeep.Equal(t, expectMap, actualMap)
		return true
	}, 10*time.Second, 100*time.Millisecond)
}

func findModifiedJobs(t sktest.TestingT, m <-chan []*types.Job, expect ...*types.Job) {
	expectMap := make(map[string]*types.Job, len(expect))
	for _, e := range expect {
		expectMap[e.Id] = e
	}
	actualMap := make(map[string]*types.Job, len(expectMap))
	require.Eventually(t, func() bool {
		jobs := <-m
		for _, a := range jobs {
			// Ignore tasks not in the expected list.
			if _, ok := expectMap[a.Id]; !ok {
				continue
			}
			exist, ok := actualMap[a.Id]
			if !ok || a.DbModified.After(exist.DbModified) {
				actualMap[a.Id] = a
			}
		}
		if len(actualMap) != len(expectMap) {
			return false
		}
		assertdeep.Equal(t, expectMap, actualMap)
		return true
	}, 10*time.Second, 100*time.Millisecond)
}

func findModifiedComments(t sktest.TestingT, tc <-chan []*types.TaskComment, tsc <-chan []*types.TaskSpecComment, cc <-chan []*types.CommitComment, e1Slice []*types.TaskComment, e2Slice []*types.TaskSpecComment, e3Slice []*types.CommitComment) {
	e1 := make(map[string]*types.TaskComment, len(e1Slice))
	for _, c := range e1Slice {
		e1[c.Id()] = c
	}
	e2 := make(map[string]*types.TaskSpecComment, len(e2Slice))
	for _, c := range e2Slice {
		e2[c.Id()] = c
	}
	e3 := make(map[string]*types.CommitComment, len(e3Slice))
	for _, c := range e3Slice {
		e3[c.Id()] = c
	}
	a1 := make(map[string]*types.TaskComment, len(e1))
	a2 := make(map[string]*types.TaskSpecComment, len(e2))
	a3 := make(map[string]*types.CommitComment, len(e3))
	require.Eventually(t, func() bool {
		select {
		case c1 := <-tc:
			for _, c := range c1 {
				if _, ok := e1[c.Id()]; !ok {
					continue
				}
				a1[c.Id()] = c
			}
		case c2 := <-tsc:
			for _, c := range c2 {
				if _, ok := e2[c.Id()]; !ok {
					continue
				}
				a2[c.Id()] = c
			}
		case c3 := <-cc:
			for _, c := range c3 {
				if _, ok := e3[c.Id()]; !ok {
					continue
				}
				a3[c.Id()] = c
			}
		}
		if len(a1) != len(e1) || len(a2) != len(e2) || len(a3) != len(e3) {
			return false
		}
		assertdeep.Equal(t, e1, a1)
		assertdeep.Equal(t, e2, a2)
		assertdeep.Equal(t, e3, a3)
		return true
	}, 10*time.Second, 100*time.Millisecond)
}

// TestTaskDB performs basic tests for an implementation of TaskDB.
func TestTaskDB(t sktest.TestingT, db TaskDB) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mod := db.ModifiedTasksCh(ctx)

	tasks := <-mod
	require.Equal(t, 0, len(tasks))

	t1 := types.MakeTestTask(time.Time{}, []string{"a", "b", "c", "d"})

	// AssignId should fill in t1.Id.
	require.Equal(t, "", t1.Id)
	require.NoError(t, db.AssignId(ctx, t1))
	require.NotEqual(t, "", t1.Id)
	// Ids must be URL-safe.
	require.Equal(t, url.QueryEscape(t1.Id), t1.Id)

	// Task doesn't exist in DB yet.
	noTask, err := db.GetTaskById(ctx, t1.Id)
	require.NoError(t, err)
	require.Nil(t, noTask)

	// Set Creation time. Ensure Created is not the time of AssignId to test the
	// sequence (1) AssignId, (2) initialize task, (3) PutTask.
	timeStart := time.Now()
	now := timeStart.Add(TS_RESOLUTION)
	t1.Created = now

	// Insert the task.
	require.NoError(t, db.PutTask(ctx, t1))

	// Check that DbModified was set.
	require.False(t, util.TimeIsZero(t1.DbModified))
	t1LastModified := t1.DbModified

	// Task can now be retrieved by Id.
	t1Again, err := db.GetTaskById(ctx, t1.Id)
	require.NoError(t, err)
	AssertDeepEqual(t, t1, t1Again)

	// Ensure that the task shows up in the modified list.
	findModifiedTasks(t, mod, t1)

	// Ensure that the task shows up in the correct date ranges.
	t1Before := t1.Created
	t1After := t1Before.Add(1 * TS_RESOLUTION)
	timeEnd := now.Add(2 * TS_RESOLUTION)
	tasks, err = db.GetTasksFromDateRange(ctx, timeStart, t1Before, "")
	require.NoError(t, err)
	require.Equal(t, 0, len(tasks))
	tasks, err = db.GetTasksFromDateRange(ctx, t1Before, t1After, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Task{t1}, tasks)
	tasks, err = db.GetTasksFromDateRange(ctx, t1After, timeEnd, "")
	require.NoError(t, err)
	require.Equal(t, 0, len(tasks))

	// Insert two more tasks. Ensure at least 1 microsecond between task Created
	// times so that t1After != t2Before and t2After != t3Before.
	t2 := types.MakeTestTask(now.Add(TS_RESOLUTION), []string{"e", "f"})
	t3 := types.MakeTestTask(now.Add(2*TS_RESOLUTION), []string{"g", "h"})
	require.NoError(t, db.PutTasks(ctx, []*types.Task{t2, t3}))

	// Check that PutTasks assigned Ids.
	require.NotEqual(t, "", t2.Id)
	require.NotEqual(t, "", t3.Id)
	// Ids must be URL-safe.
	require.Equal(t, url.QueryEscape(t2.Id), t2.Id)
	require.Equal(t, url.QueryEscape(t3.Id), t3.Id)

	// Ensure that both tasks show up in the modified list.
	findModifiedTasks(t, mod, t2, t3)

	// Make an update to t1 and t2. Ensure modified times change.
	t2LastModified := t2.DbModified
	t1.Status = types.TASK_STATUS_RUNNING
	t2.Status = types.TASK_STATUS_SUCCESS
	require.NoError(t, db.PutTasks(ctx, []*types.Task{t1, t2}))
	require.False(t, t1.DbModified.Equal(t1LastModified))
	require.False(t, t2.DbModified.Equal(t2LastModified))

	// Ensure that both tasks show up in the modified list.
	findModifiedTasks(t, mod, t1, t2)

	// Ensure that all tasks show up in the correct time ranges, in sorted order.
	t2Before := t2.Created
	t2After := t2Before.Add(1 * TS_RESOLUTION)

	t3Before := t3.Created
	t3After := t3Before.Add(1 * TS_RESOLUTION)

	timeEnd = now.Add(3 * TS_RESOLUTION)

	tasks, err = db.GetTasksFromDateRange(ctx, timeStart, t1Before, "")
	require.NoError(t, err)
	require.Equal(t, 0, len(tasks))

	tasks, err = db.GetTasksFromDateRange(ctx, timeStart, t1After, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Task{t1}, tasks)

	tasks, err = db.GetTasksFromDateRange(ctx, timeStart, t2Before, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Task{t1}, tasks)

	tasks, err = db.GetTasksFromDateRange(ctx, timeStart, t2After, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Task{t1, t2}, tasks)

	tasks, err = db.GetTasksFromDateRange(ctx, timeStart, t3Before, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Task{t1, t2}, tasks)

	tasks, err = db.GetTasksFromDateRange(ctx, timeStart, t3After, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Task{t1, t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(ctx, timeStart, timeEnd, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Task{t1, t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(ctx, t1Before, timeEnd, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Task{t1, t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(ctx, t1After, timeEnd, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Task{t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(ctx, t2Before, timeEnd, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Task{t2, t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(ctx, t2After, timeEnd, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Task{t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(ctx, t3Before, timeEnd, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Task{t3}, tasks)

	tasks, err = db.GetTasksFromDateRange(ctx, t3After, timeEnd, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Task{}, tasks)
}

// Test that PutTask and PutTasks return ErrConcurrentUpdate when a cached Task
// has been updated in the DB.
func TestTaskDBConcurrentUpdate(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()

	// Insert a task.
	t1 := types.MakeTestTask(time.Now(), []string{"a", "b", "c", "d"})
	require.NoError(t, db.PutTask(ctx, t1))

	// Retrieve a copy of the task.
	t1Cached, err := db.GetTaskById(ctx, t1.Id)
	require.NoError(t, err)
	AssertDeepEqual(t, t1, t1Cached)

	// Update the original task.
	t1.Commits = []string{"a", "b"}
	require.NoError(t, db.PutTask(ctx, t1))

	// Update the cached copy; should get concurrent update error.
	t1Cached.Status = types.TASK_STATUS_RUNNING
	err = db.PutTask(ctx, t1Cached)
	require.True(t, IsConcurrentUpdate(err))

	{
		// DB should still have the old value of t1.
		t1Again, err := db.GetTaskById(ctx, t1.Id)
		require.NoError(t, err)
		AssertDeepEqual(t, t1, t1Again)
	}

	// Insert a second task.
	t2 := types.MakeTestTask(time.Now(), []string{"e", "f"})
	require.NoError(t, db.PutTask(ctx, t2))

	// Update t2 at the same time as t1Cached; should still get an error.
	t2Before := t2.Copy()
	t2.Status = types.TASK_STATUS_MISHAP
	err = db.PutTasks(ctx, []*types.Task{t2, t1Cached})
	require.True(t, IsConcurrentUpdate(err))

	{
		// DB should still have the old value of t1 and t2.
		t1Again, err := db.GetTaskById(ctx, t1.Id)
		require.NoError(t, err)
		AssertDeepEqual(t, t1, t1Again)

		t2Again, err := db.GetTaskById(ctx, t2.Id)
		require.NoError(t, err)
		AssertDeepEqual(t, t2Before, t2Again)
	}
}

// Test UpdateTasksWithRetries when no errors or retries.
func testUpdateTasksWithRetriesSimple(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()
	begin := time.Now()

	// Test no-op.
	tasks, err := UpdateTasksWithRetries(ctx, db, func() ([]*types.Task, error) {
		return nil, nil
	})
	require.NoError(t, err)
	require.Equal(t, 0, len(tasks))

	// Create new task t1. (UpdateTasksWithRetries isn't actually useful in this case.)
	tasks, err = UpdateTasksWithRetries(ctx, db, func() ([]*types.Task, error) {
		t1 := types.MakeTestTask(time.Time{}, []string{"a", "b", "c", "d"})
		require.NoError(t, db.AssignId(ctx, t1))
		t1.Created = time.Now().Add(TS_RESOLUTION)
		return []*types.Task{t1}, nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks))
	t1 := tasks[0]

	// Update t1 and create t2.
	tasks, err = UpdateTasksWithRetries(ctx, db, func() ([]*types.Task, error) {
		t1, err := db.GetTaskById(ctx, t1.Id)
		require.NoError(t, err)
		t1.Status = types.TASK_STATUS_RUNNING
		t2 := types.MakeTestTask(t1.Created.Add(TS_RESOLUTION), []string{"e", "f"})
		return []*types.Task{t1, t2}, nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, len(tasks))
	require.Equal(t, t1.Id, tasks[0].Id)
	require.Equal(t, types.TASK_STATUS_RUNNING, tasks[0].Status)
	require.Equal(t, []string{"e", "f"}, tasks[1].Commits)

	// Check that return value matches what's in the DB.
	t1, err = db.GetTaskById(ctx, t1.Id)
	require.NoError(t, err)
	t2, err := db.GetTaskById(ctx, tasks[1].Id)
	require.NoError(t, err)
	AssertDeepEqual(t, tasks[0], t1)
	AssertDeepEqual(t, tasks[1], t2)

	// Check no extra tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(ctx, begin, time.Now().Add(3*TS_RESOLUTION), "")
	require.NoError(t, err)
	require.Equal(t, 2, len(tasks))
	require.Equal(t, t1.Id, tasks[0].Id)
	require.Equal(t, t2.Id, tasks[1].Id)
}

// Test UpdateTasksWithRetries when there are some retries, but eventual success.
func testUpdateTasksWithRetriesSuccess(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()
	begin := time.Now()

	// Create and cache.
	t1 := types.MakeTestTask(begin.Add(TS_RESOLUTION), []string{"a", "b", "c", "d"})
	require.NoError(t, db.PutTask(ctx, t1))
	t1Cached := t1.Copy()

	// Update original.
	t1.Status = types.TASK_STATUS_RUNNING
	require.NoError(t, db.PutTask(ctx, t1))

	// Attempt update.
	callCount := 0
	tasks, err := UpdateTasksWithRetries(ctx, db, func() ([]*types.Task, error) {
		callCount++
		if callCount >= 3 {
			if task, err := db.GetTaskById(ctx, t1.Id); err != nil {
				return nil, err
			} else {
				t1Cached = task
			}
		}
		t1Cached.Status = types.TASK_STATUS_SUCCESS
		t2 := types.MakeTestTask(begin.Add(2*TS_RESOLUTION), []string{"e", "f"})
		return []*types.Task{t1Cached, t2}, nil
	})
	require.NoError(t, err)
	require.Equal(t, 3, callCount)
	require.Equal(t, 2, len(tasks))
	require.Equal(t, t1.Id, tasks[0].Id)
	require.Equal(t, types.TASK_STATUS_SUCCESS, tasks[0].Status)
	require.Equal(t, []string{"e", "f"}, tasks[1].Commits)

	// Check that return value matches what's in the DB.
	t1, err = db.GetTaskById(ctx, t1.Id)
	require.NoError(t, err)
	t2, err := db.GetTaskById(ctx, tasks[1].Id)
	require.NoError(t, err)
	AssertDeepEqual(t, tasks[0], t1)
	AssertDeepEqual(t, tasks[1], t2)

	// Check no extra tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(ctx, begin, time.Now().Add(3*TS_RESOLUTION), "")
	require.NoError(t, err)
	require.Equal(t, 2, len(tasks))
	require.Equal(t, t1.Id, tasks[0].Id)
	require.Equal(t, t2.Id, tasks[1].Id)
}

// Test UpdateTasksWithRetries when f returns an error.
func testUpdateTasksWithRetriesErrorInFunc(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()
	begin := time.Now()

	myErr := fmt.Errorf("NO! Bad dog!")
	callCount := 0
	tasks, err := UpdateTasksWithRetries(ctx, db, func() ([]*types.Task, error) {
		callCount++
		// Return a task just for fun.
		return []*types.Task{
			types.MakeTestTask(begin.Add(TS_RESOLUTION), []string{"a", "b", "c", "d"}),
		}, myErr
	})
	require.Error(t, err)
	require.Equal(t, myErr, err)
	require.Equal(t, 0, len(tasks))
	require.Equal(t, 1, callCount)

	// Check no tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(ctx, begin, time.Now().Add(2*TS_RESOLUTION), "")
	require.NoError(t, err)
	require.Equal(t, 0, len(tasks))
}

// Test UpdateTasksWithRetries when PutTasks returns an error.
func testUpdateTasksWithRetriesErrorInPutTasks(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()
	begin := time.Now()

	callCount := 0
	tasks, err := UpdateTasksWithRetries(ctx, db, func() ([]*types.Task, error) {
		callCount++
		// Task has zero Created time.
		return []*types.Task{
			types.MakeTestTask(time.Time{}, []string{"a", "b", "c", "d"}),
		}, nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Created not set.")
	require.Equal(t, 0, len(tasks))
	require.Equal(t, 1, callCount)

	// Check no tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(ctx, begin, time.Now().Add(TS_RESOLUTION), "")
	require.NoError(t, err)
	require.Equal(t, 0, len(tasks))
}

// Test UpdateTasksWithRetries when retries are exhausted.
func testUpdateTasksWithRetriesExhausted(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()
	begin := time.Now()

	// Create and cache.
	t1 := types.MakeTestTask(begin.Add(TS_RESOLUTION), []string{"a", "b", "c", "d"})
	require.NoError(t, db.PutTask(ctx, t1))
	t1Cached := t1.Copy()

	// Update original.
	t1.Status = types.TASK_STATUS_RUNNING
	require.NoError(t, db.PutTask(ctx, t1))

	// Attempt update.
	callCount := 0
	tasks, err := UpdateTasksWithRetries(ctx, db, func() ([]*types.Task, error) {
		callCount++
		t1Cached.Status = types.TASK_STATUS_SUCCESS
		t2 := types.MakeTestTask(begin.Add(2*TS_RESOLUTION), []string{"e", "f"})
		return []*types.Task{t1Cached, t2}, nil
	})
	require.True(t, IsConcurrentUpdate(err))
	require.Equal(t, NUM_RETRIES, callCount)
	require.Equal(t, 0, len(tasks))

	// Check no extra tasks in the DB.
	tasks, err = db.GetTasksFromDateRange(ctx, begin, time.Now().Add(3*TS_RESOLUTION), "")
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks))
	require.Equal(t, t1.Id, tasks[0].Id)
	require.Equal(t, types.TASK_STATUS_RUNNING, tasks[0].Status)
}

// Test UpdateTaskWithRetries when no errors or retries.
func testUpdateTaskWithRetriesSimple(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()
	begin := time.Now()

	// Create new task t1.
	t1 := types.MakeTestTask(time.Time{}, []string{"a", "b", "c", "d"})
	require.NoError(t, db.AssignId(ctx, t1))
	t1.Created = time.Now().Add(TS_RESOLUTION)
	require.NoError(t, db.PutTask(ctx, t1))

	// Update t1.
	t1Updated, err := UpdateTaskWithRetries(ctx, db, t1.Id, func(task *types.Task) error {
		task.Status = types.TASK_STATUS_RUNNING
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, t1.Id, t1Updated.Id)
	require.Equal(t, types.TASK_STATUS_RUNNING, t1Updated.Status)
	require.NotEqual(t, t1.DbModified, t1Updated.DbModified)

	// Check that return value matches what's in the DB.
	t1Again, err := db.GetTaskById(ctx, t1.Id)
	require.NoError(t, err)
	AssertDeepEqual(t, t1Again, t1Updated)

	// Check no extra tasks in the TaskDB.
	tasks, err := db.GetTasksFromDateRange(ctx, begin, time.Now().Add(2*TS_RESOLUTION), "")
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks))
	require.Equal(t, t1.Id, tasks[0].Id)
}

// Test UpdateTaskWithRetries when there are some retries, but eventual success.
func testUpdateTaskWithRetriesSuccess(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()
	begin := time.Now()

	// Create new task t1.
	t1 := types.MakeTestTask(begin.Add(TS_RESOLUTION), []string{"a", "b", "c", "d"})
	require.NoError(t, db.PutTask(ctx, t1))

	// Attempt update.
	callCount := 0
	t1Updated, err := UpdateTaskWithRetries(ctx, db, t1.Id, func(task *types.Task) error {
		callCount++
		if callCount < 3 {
			// Sneakily make an update in the background.
			t1.Commits = append(t1.Commits, fmt.Sprintf("z%d", callCount))
			require.NoError(t, db.PutTask(ctx, t1))
		}
		task.Status = types.TASK_STATUS_SUCCESS
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 3, callCount)
	require.Equal(t, t1.Id, t1Updated.Id)
	require.Equal(t, types.TASK_STATUS_SUCCESS, t1Updated.Status)

	// Check that return value matches what's in the DB.
	t1Again, err := db.GetTaskById(ctx, t1.Id)
	require.NoError(t, err)
	AssertDeepEqual(t, t1Again, t1Updated)

	// Check no extra tasks in the DB.
	tasks, err := db.GetTasksFromDateRange(ctx, begin, time.Now().Add(2*TS_RESOLUTION), "")
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks))
	require.Equal(t, t1.Id, tasks[0].Id)
}

// Test UpdateTaskWithRetries when f returns an error.
func testUpdateTaskWithRetriesErrorInFunc(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()
	begin := time.Now()

	// Create new task t1.
	t1 := types.MakeTestTask(begin.Add(TS_RESOLUTION), []string{"a", "b", "c", "d"})
	require.NoError(t, db.PutTask(ctx, t1))

	// Update and return an error.
	myErr := fmt.Errorf("Um, actually, I didn't want to update that task.")
	callCount := 0
	noTask, err := UpdateTaskWithRetries(ctx, db, t1.Id, func(task *types.Task) error {
		callCount++
		// Update task to test nothing changes in DB.
		task.Status = types.TASK_STATUS_RUNNING
		return myErr
	})
	require.Error(t, err)
	require.Equal(t, myErr, err)
	require.Nil(t, noTask)
	require.Equal(t, 1, callCount)

	// Check task did not change in the DB.
	t1Again, err := db.GetTaskById(ctx, t1.Id)
	require.NoError(t, err)
	AssertDeepEqual(t, t1, t1Again)

	// Check no extra tasks in the DB.
	tasks, err := db.GetTasksFromDateRange(ctx, begin, time.Now().Add(2*TS_RESOLUTION), "")
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks))
	require.Equal(t, t1.Id, tasks[0].Id)
}

// Test UpdateTaskWithRetries when retries are exhausted.
func testUpdateTaskWithRetriesExhausted(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()
	begin := time.Now()

	// Create new task t1.
	t1 := types.MakeTestTask(begin.Add(TS_RESOLUTION), []string{"a", "b", "c", "d"})
	require.NoError(t, db.PutTask(ctx, t1))

	// Update original.
	t1.Status = types.TASK_STATUS_RUNNING
	require.NoError(t, db.PutTask(ctx, t1))

	// Attempt update.
	callCount := 0
	noTask, err := UpdateTaskWithRetries(ctx, db, t1.Id, func(task *types.Task) error {
		callCount++
		// Sneakily make an update in the background.
		t1.Commits = append(t1.Commits, fmt.Sprintf("z%d", callCount))
		require.NoError(t, db.PutTask(ctx, t1))

		task.Status = types.TASK_STATUS_SUCCESS
		return nil
	})
	require.True(t, IsConcurrentUpdate(err))
	require.Equal(t, NUM_RETRIES, callCount)
	require.Nil(t, noTask)

	// Check task did not change in the DB.
	t1Again, err := db.GetTaskById(ctx, t1.Id)
	require.NoError(t, err)
	AssertDeepEqual(t, t1, t1Again)

	// Check no extra tasks in the DB.
	tasks, err := db.GetTasksFromDateRange(ctx, begin, time.Now().Add(2*TS_RESOLUTION), "")
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks))
	require.Equal(t, t1.Id, tasks[0].Id)
}

// Test UpdateTaskWithRetries when the given ID is not found in the DB.
func testUpdateTaskWithRetriesTaskNotFound(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()
	begin := time.Now()

	// Assign ID for a task, but don't put it in the DB.
	t1 := types.MakeTestTask(begin.Add(TS_RESOLUTION), []string{"a", "b", "c", "d"})
	require.NoError(t, db.AssignId(ctx, t1))

	// Attempt to update non-existent task. Function shouldn't be called.
	callCount := 0
	noTask, err := UpdateTaskWithRetries(ctx, db, t1.Id, func(task *types.Task) error {
		callCount++
		task.Status = types.TASK_STATUS_RUNNING
		return nil
	})
	require.True(t, IsNotFound(err))
	require.Nil(t, noTask)
	require.Equal(t, 0, callCount)

	// Check no tasks in the DB.
	tasks, err := db.GetTasksFromDateRange(ctx, begin, time.Now().Add(2*TS_RESOLUTION), "")
	require.NoError(t, err)
	require.Equal(t, 0, len(tasks))
}

// Test UpdateTasksWithRetries and UpdateTaskWithRetries.
func TestUpdateTasksWithRetries(t sktest.TestingT, db TaskDB) {
	testUpdateTasksWithRetriesSimple(t, db)
	time.Sleep(TS_RESOLUTION)
	testUpdateTasksWithRetriesSuccess(t, db)
	time.Sleep(TS_RESOLUTION)
	testUpdateTasksWithRetriesErrorInFunc(t, db)
	time.Sleep(TS_RESOLUTION)
	testUpdateTasksWithRetriesErrorInPutTasks(t, db)
	time.Sleep(TS_RESOLUTION)
	testUpdateTasksWithRetriesExhausted(t, db)
	time.Sleep(TS_RESOLUTION)
	testUpdateTaskWithRetriesSimple(t, db)
	time.Sleep(TS_RESOLUTION)
	testUpdateTaskWithRetriesSuccess(t, db)
	time.Sleep(TS_RESOLUTION)
	testUpdateTaskWithRetriesErrorInFunc(t, db)
	time.Sleep(TS_RESOLUTION)
	testUpdateTaskWithRetriesExhausted(t, db)
	time.Sleep(TS_RESOLUTION)
	testUpdateTaskWithRetriesTaskNotFound(t, db)
}

// TestTaskDBSearch tests TaskReader.SearchTasks.
func TestTaskDBSearch(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()
	tStart := time.Unix(1614172856, 0)
	tCurrent := tStart
	makeTask := func() *types.Task {
		task := types.MakeTestTask(tCurrent, []string{"abc123"})
		task.Id = uuid.New().String()
		task.Name = "my-task"
		task.Repo = "repo1"
		task.ForcedJobId = ""
		task.Issue = ""
		task.Patchset = ""
		tCurrent = tCurrent.Add(2 * time.Second)
		return task
	}

	t1 := makeTask()
	t1.ForcedJobId = "123456789"
	t1.Issue = "12345"
	t1.Patchset = "1"
	t1.Status = types.TASK_STATUS_FAILURE

	t2 := makeTask()
	t2.ForcedJobId = "987654321"
	t2.Issue = "12345"
	t2.Patchset = "2"
	t2.Revision = "def456"
	t2.Status = types.TASK_STATUS_RUNNING

	t3 := makeTask()
	t3.Revision = "def456"
	t3.Status = types.TASK_STATUS_SUCCESS

	t4 := makeTask()
	t4.Status = types.TASK_STATUS_MISHAP

	t5 := makeTask()
	t5.Name = "my-other-task"
	t5.Repo = "repo2"
	t5.Revision = "bbad"
	t5.Status = types.TASK_STATUS_FAILURE

	t6 := makeTask()
	t6.Attempt = 2

	require.NoError(t, db.PutTasks(ctx, []*types.Task{t1, t2, t3, t4, t5, t6}))

	test := func(params *TaskSearchParams, expect ...*types.Task) {
		actual, err := SearchTasks(ctx, db, params)
		require.NoError(t, err)
		sort.Sort(types.TaskSlice(expect))
		sort.Sort(types.TaskSlice(actual))
		require.Equal(t, expect, actual)
	}
	i := func(v int64) *int64 {
		return &v
	}
	s := func(v string) *string {
		return &v
	}
	ts := func(v time.Time) *time.Time {
		return &v
	}

	test(&TaskSearchParams{
		ForcedJobId: s("123456789"),
		TimeStart:   &tStart,
		TimeEnd:     &tCurrent,
	}, t1)
	test(&TaskSearchParams{
		Issue:     s("12345"),
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, t1, t2)
	test(&TaskSearchParams{
		Patchset:  s("2"),
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, t2)
	st := types.TASK_STATUS_FAILURE
	test(&TaskSearchParams{
		Status:    &st,
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, t1, t5)
	test(&TaskSearchParams{
		Revision:  s("def456"),
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, t2, t3)
	test(&TaskSearchParams{
		Repo:      s("repo2"),
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, t5)
	test(&TaskSearchParams{
		Repo:      s(t1.Repo),
		Issue:     s("12345"),
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, t1, t2)
	test(&TaskSearchParams{
		Repo:      s(t1.Repo),
		Issue:     s("12345"),
		TimeStart: ts(tStart),
		TimeEnd:   ts(tCurrent.Add(time.Second)),
	}, t1, t2)
	test(&TaskSearchParams{
		Repo:      s(t1.Repo),
		Issue:     s("12345"),
		TimeStart: ts(t1.Created.Add(firestore.TS_RESOLUTION)),
		TimeEnd:   ts(tCurrent.Add(time.Second)),
	}, t2)
	test(&TaskSearchParams{
		Repo:      s(t1.Repo),
		Issue:     s("12345"),
		TimeStart: ts(tStart),
		TimeEnd:   ts(t2.Created.Add(-firestore.TS_RESOLUTION)),
	}, t1)
	test(&TaskSearchParams{
		Attempt:   i(2),
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, t6)
	test(&TaskSearchParams{
		Name:      s("my-task"),
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, t1, t2, t3, t4, t6)
}

// TestJobDB performs basic tests on an implementation of JobDB.
func TestJobDB(t sktest.TestingT, db JobDB) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mod := db.ModifiedJobsCh(ctx)

	jobs := <-mod
	require.Equal(t, 0, len(jobs))

	now := time.Now().Add(TS_RESOLUTION)
	j1 := types.MakeTestJob(now)

	// Insert the job.
	require.NoError(t, db.PutJob(ctx, j1))

	// Ids must be URL-safe.
	require.NotEqual(t, "", j1.Id)
	require.Equal(t, url.QueryEscape(j1.Id), j1.Id)

	// Check that DbModified was set.
	require.False(t, util.TimeIsZero(j1.DbModified))
	j1LastModified := j1.DbModified

	// Job can now be retrieved by Id.
	j1Again, err := db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	AssertDeepEqual(t, j1, j1Again)

	// Ensure that the job shows up in the modified list.
	findModifiedJobs(t, mod, j1)

	// Ensure that the job shows up in the correct date ranges.
	timeStart := time.Unix(0, 0).UTC()
	j1Before := j1.Created
	j1After := j1Before.Add(1 * TS_RESOLUTION)
	timeEnd := now.Add(2 * TS_RESOLUTION)
	jobs, err = db.GetJobsFromDateRange(ctx, timeStart, j1Before, "")
	require.NoError(t, err)
	require.Equal(t, 0, len(jobs))
	jobs, err = db.GetJobsFromDateRange(ctx, j1Before, j1After, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Job{j1}, jobs)
	jobs, err = db.GetJobsFromDateRange(ctx, j1After, timeEnd, "")
	require.NoError(t, err)
	require.Equal(t, 0, len(jobs))
	jobs, err = db.GetJobsFromDateRange(ctx, j1Before, j1After, "bogusRepo")
	require.NoError(t, err)
	require.Equal(t, 0, len(jobs))
	jobs, err = db.GetJobsFromDateRange(ctx, j1Before, j1After, j1.Repo)
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Job{j1}, jobs)
	require.NotEqual(t, "", j1.Repo)

	// Insert two more jobs. Ensure at least 1 microsecond between job Created
	// times so that j1After != j2Before and j2After != j3Before.
	j2 := types.MakeTestJob(now.Add(TS_RESOLUTION))
	j3 := types.MakeTestJob(now.Add(2 * TS_RESOLUTION))
	require.NoError(t, db.PutJobs(ctx, []*types.Job{j2, j3}))

	// Check that PutJobs assigned Ids.
	require.NotEqual(t, "", j2.Id)
	require.NotEqual(t, "", j3.Id)
	// Ids must be URL-safe.
	require.Equal(t, url.QueryEscape(j2.Id), j2.Id)
	require.Equal(t, url.QueryEscape(j3.Id), j3.Id)

	// Ensure that both jobs show up in the modified list.
	findModifiedJobs(t, mod, j2, j3)

	// Make an update to j1 and j2. Ensure modified times change.
	j2LastModified := j2.DbModified
	j1.Status = types.JOB_STATUS_IN_PROGRESS
	j2.Status = types.JOB_STATUS_SUCCESS
	require.NoError(t, db.PutJobs(ctx, []*types.Job{j1, j2}))
	require.False(t, j1.DbModified.Equal(j1LastModified))
	require.False(t, j2.DbModified.Equal(j2LastModified))

	// Ensure that both jobs show up in the modified list.
	findModifiedJobs(t, mod, j1, j2)

	// Ensure that all jobs show up in the correct time ranges, in sorted order.
	j2Before := j2.Created
	j2After := j2Before.Add(1 * TS_RESOLUTION)

	j3Before := j3.Created
	j3After := j3Before.Add(1 * TS_RESOLUTION)

	timeEnd = now.Add(3 * TS_RESOLUTION)

	jobs, err = db.GetJobsFromDateRange(ctx, timeStart, j1Before, "")
	require.NoError(t, err)
	require.Equal(t, 0, len(jobs))

	jobs, err = db.GetJobsFromDateRange(ctx, timeStart, j1After, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Job{j1}, jobs)

	jobs, err = db.GetJobsFromDateRange(ctx, timeStart, j2Before, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Job{j1}, jobs)

	jobs, err = db.GetJobsFromDateRange(ctx, timeStart, j2After, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Job{j1, j2}, jobs)

	jobs, err = db.GetJobsFromDateRange(ctx, timeStart, j3Before, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Job{j1, j2}, jobs)

	jobs, err = db.GetJobsFromDateRange(ctx, timeStart, j3After, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Job{j1, j2, j3}, jobs)

	jobs, err = db.GetJobsFromDateRange(ctx, timeStart, timeEnd, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Job{j1, j2, j3}, jobs)

	jobs, err = db.GetJobsFromDateRange(ctx, j1Before, timeEnd, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Job{j1, j2, j3}, jobs)

	jobs, err = db.GetJobsFromDateRange(ctx, j1After, timeEnd, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Job{j2, j3}, jobs)

	jobs, err = db.GetJobsFromDateRange(ctx, j2Before, timeEnd, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Job{j2, j3}, jobs)

	jobs, err = db.GetJobsFromDateRange(ctx, j2After, timeEnd, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Job{j3}, jobs)

	jobs, err = db.GetJobsFromDateRange(ctx, j3Before, timeEnd, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Job{j3}, jobs)

	jobs, err = db.GetJobsFromDateRange(ctx, j3After, timeEnd, "")
	require.NoError(t, err)
	AssertDeepEqual(t, []*types.Job{}, jobs)
}

// Test that PutJob and PutJobs return ErrConcurrentUpdate when a cached Job
// has been updated in the DB.
func TestJobDBConcurrentUpdate(t sktest.TestingT, db JobDB) {
	ctx := context.Background()
	// Insert a job.
	j1 := types.MakeTestJob(time.Now())
	require.NoError(t, db.PutJob(ctx, j1))

	// Retrieve a copy of the job.
	j1Cached, err := db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	AssertDeepEqual(t, j1, j1Cached)

	// Update the original job.
	j1.Repo = "another-repo"
	require.NoError(t, db.PutJob(ctx, j1))

	// Update the cached copy; should get concurrent update error.
	j1Cached.Status = types.JOB_STATUS_IN_PROGRESS
	err = db.PutJob(ctx, j1Cached)
	require.True(t, IsConcurrentUpdate(err))

	{
		// DB should still have the old value of j1.
		j1Again, err := db.GetJobById(ctx, j1.Id)
		require.NoError(t, err)
		AssertDeepEqual(t, j1, j1Again)
	}

	// Insert a second job.
	j2 := types.MakeTestJob(time.Now())
	require.NoError(t, db.PutJob(ctx, j2))

	// Update j2 at the same time as j1Cached; should still get an error.
	j2Before := j2.Copy()
	j2.Status = types.JOB_STATUS_MISHAP
	err = db.PutJobs(ctx, []*types.Job{j2, j1Cached})
	require.True(t, IsConcurrentUpdate(err))

	{
		// DB should still have the old value of j1 and j2.
		j1Again, err := db.GetJobById(ctx, j1.Id)
		require.NoError(t, err)
		AssertDeepEqual(t, j1, j1Again)

		j2Again, err := db.GetJobById(ctx, j2.Id)
		require.NoError(t, err)
		AssertDeepEqual(t, j2Before, j2Again)
	}
}

// TestJobDBSearch tests JobReader.SearchJobs.
func TestJobDBSearch(t sktest.TestingT, db JobDB) {
	ctx := context.Background()
	tStart := time.Unix(1614172856, 0)
	tCurrent := tStart
	makeJob := func() *types.Job {
		job := types.MakeTestJob(tCurrent)
		job.Id = uuid.New().String()
		job.Name = "my-job"
		job.Repo = "repo1"
		job.BuildbucketBuildId = 0
		job.Issue = ""
		job.Patchset = ""
		tCurrent = tCurrent.Add(2 * time.Second)
		return job
	}

	j1 := makeJob()
	j1.BuildbucketBuildId = 123456789
	j1.Issue = "12345"
	j1.Patchset = "1"
	j1.Revision = "abc123"
	j1.Status = types.JOB_STATUS_FAILURE

	j2 := makeJob()
	j2.BuildbucketBuildId = 987654321
	j2.Issue = "12345"
	j2.Patchset = "2"
	j2.Revision = "def456"
	j2.Status = types.JOB_STATUS_IN_PROGRESS

	j3 := makeJob()
	j3.Revision = "def456"
	j3.Status = types.JOB_STATUS_SUCCESS

	j4 := makeJob()
	j4.Revision = "abc123"
	j4.Status = types.JOB_STATUS_MISHAP

	j5 := makeJob()
	j5.Name = "my-other-job"
	j5.Repo = "repo2"
	j5.Revision = "bbad"
	j5.Status = types.JOB_STATUS_FAILURE

	require.NoError(t, db.PutJobs(ctx, []*types.Job{j1, j2, j3, j4, j5}))

	test := func(params *JobSearchParams, expect ...*types.Job) {
		actual, err := SearchJobs(ctx, db, params)
		require.NoError(t, err)
		sort.Sort(types.JobSlice(expect))
		sort.Sort(types.JobSlice(actual))
		require.Equal(t, expect, actual)
	}
	i := func(v int64) *int64 {
		return &v
	}
	s := func(v string) *string {
		return &v
	}
	ts := func(v time.Time) *time.Time {
		return &v
	}

	test(&JobSearchParams{
		BuildbucketBuildID: i(123456789),
		TimeStart:          &tStart,
		TimeEnd:            &tCurrent,
	}, j1)
	test(&JobSearchParams{
		Issue:     s("12345"),
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, j1, j2)
	test(&JobSearchParams{
		Patchset:  s("2"),
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, j2)
	st := types.JOB_STATUS_FAILURE
	test(&JobSearchParams{
		Status:    &st,
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, j1, j5)
	test(&JobSearchParams{
		Revision:  s("def456"),
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, j2, j3)
	test(&JobSearchParams{
		Repo:      s("repo2"),
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, j5)
	test(&JobSearchParams{
		Repo:      s(j1.Repo),
		Issue:     s("12345"),
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, j1, j2)
	test(&JobSearchParams{
		Repo:      s(j1.Repo),
		Issue:     s("12345"),
		TimeStart: ts(tStart),
		TimeEnd:   ts(tCurrent.Add(time.Second)),
	}, j1, j2)
	test(&JobSearchParams{
		Repo:      s(j1.Repo),
		Issue:     s("12345"),
		TimeStart: ts(j1.Created.Add(firestore.TS_RESOLUTION)),
		TimeEnd:   ts(tCurrent.Add(time.Second)),
	}, j2)
	test(&JobSearchParams{
		Repo:      s(j1.Repo),
		Issue:     s("12345"),
		TimeStart: ts(tStart),
		TimeEnd:   ts(j2.Created.Add(-firestore.TS_RESOLUTION)),
	}, j1)
	test(&JobSearchParams{
		Name:      s("my-job"),
		TimeStart: &tStart,
		TimeEnd:   &tCurrent,
	}, j1, j2, j3, j4)
}

// TestCommentDB validates that db correctly implements the CommentDB interface.
func TestCommentDB(t sktest.TestingT, db CommentDB) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	modTC := db.ModifiedTaskCommentsCh(ctx)
	modTSC := db.ModifiedTaskSpecCommentsCh(ctx)
	modCC := db.ModifiedCommitCommentsCh(ctx)

	c1 := <-modTC
	c2 := <-modTSC
	c3 := <-modCC
	require.Equal(t, 0, len(c1))
	require.Equal(t, 0, len(c2))
	require.Equal(t, 0, len(c3))

	now := time.Now().Truncate(TS_RESOLUTION)

	// Empty db.
	r0 := fmt.Sprintf("%s%d", common.REPO_SKIA, 0)
	r1 := fmt.Sprintf("%s%d", common.REPO_SKIA, 1)
	r2 := fmt.Sprintf("%s%d", common.REPO_SKIA, 2)
	{
		actual, err := db.GetCommentsForRepos(ctx, []string{r0, r1, r2}, now.Add(-10000*time.Hour))
		require.NoError(t, err)
		require.Equal(t, 3, len(actual))
		require.Equal(t, r0, actual[0].Repo)
		require.Equal(t, r1, actual[1].Repo)
		require.Equal(t, r2, actual[2].Repo)
		for _, rc := range actual {
			require.Equal(t, 0, len(rc.TaskComments))
			require.Equal(t, 0, len(rc.TaskSpecComments))
			require.Equal(t, 0, len(rc.CommitComments))
		}
	}

	// Add some comments.
	tc1 := types.MakeTaskComment(1, 1, 1, 1, now.Add(-2*time.Second))
	tc2 := types.MakeTaskComment(2, 1, 1, 1, now)
	tc3 := types.MakeTaskComment(3, 1, 1, 1, now.Add(-time.Second))
	tc4 := types.MakeTaskComment(4, 1, 1, 2, now.Add(-2*time.Second+time.Millisecond))
	tc5 := types.MakeTaskComment(5, 1, 2, 2, now.Add(-2*time.Second+2*time.Millisecond))
	tc6 := types.MakeTaskComment(6, 2, 3, 3, now.Add(-2*time.Second+3*time.Millisecond))
	for _, c := range []*types.TaskComment{tc1, tc2, tc3, tc4, tc5, tc6} {
		require.NoError(t, db.PutTaskComment(ctx, c))
	}
	tc6copy := tc6.Copy()
	tc6.Message = "modifying after Put shouldn't affect stored comment"

	sc1 := types.MakeTaskSpecComment(1, 1, 1, now.Add(-2*time.Second))
	sc2 := types.MakeTaskSpecComment(2, 1, 1, now)
	sc3 := types.MakeTaskSpecComment(3, 1, 1, now.Add(-time.Second))
	sc4 := types.MakeTaskSpecComment(4, 1, 2, now.Add(-2*time.Second+time.Millisecond))
	sc5 := types.MakeTaskSpecComment(5, 2, 3, now.Add(-2*time.Second+2*time.Millisecond))
	for _, c := range []*types.TaskSpecComment{sc1, sc2, sc3, sc4, sc5} {
		require.NoError(t, db.PutTaskSpecComment(ctx, c))
	}
	sc5copy := sc5.Copy()
	sc5.Message = "modifying after Put shouldn't affect stored comment"

	cc1 := types.MakeCommitComment(1, 1, 1, now.Add(-2*time.Second))
	cc2 := types.MakeCommitComment(2, 1, 1, now)
	cc3 := types.MakeCommitComment(3, 1, 1, now.Add(-time.Second))
	cc4 := types.MakeCommitComment(4, 1, 2, now.Add(-2*time.Second+time.Millisecond))
	cc5 := types.MakeCommitComment(5, 2, 3, now.Add(-2*time.Second+2*time.Millisecond))
	for _, c := range []*types.CommitComment{cc1, cc2, cc3, cc4, cc5} {
		require.NoError(t, db.PutCommitComment(ctx, c))
	}
	cc5copy := cc5.Copy()
	cc5.Message = "modifying after Put shouldn't affect stored comment"

	// Ensure that all comments show up in the modified list.
	findModifiedComments(t, modTC, modTSC, modCC, []*types.TaskComment{tc1, tc4, tc5, tc6copy, tc3, tc2}, []*types.TaskSpecComment{sc1, sc4, sc5copy, sc3, sc2}, []*types.CommitComment{cc1, cc4, cc5copy, cc3, cc2})

	// Check that adding duplicate non-identical comment gives an error.
	tc1different := tc1.Copy()
	tc1different.Message = "not the same"
	require.True(t, IsAlreadyExists(db.PutTaskComment(ctx, tc1different)))
	sc1different := sc1.Copy()
	sc1different.Message = "not the same"
	require.True(t, IsAlreadyExists(db.PutTaskSpecComment(ctx, sc1different)))
	cc1different := cc1.Copy()
	cc1different.Message = "not the same"
	require.True(t, IsAlreadyExists(db.PutCommitComment(ctx, cc1different)))

	expected := []*types.RepoComments{
		{Repo: r0},
		{
			Repo: r1,
			TaskComments: map[string]map[string][]*types.TaskComment{
				"c1": {
					"n1": {tc1, tc3, tc2},
				},
				"c2": {
					"n1": {tc4},
					"n2": {tc5},
				},
			},
			TaskSpecComments: map[string][]*types.TaskSpecComment{
				"n1": {sc1, sc3, sc2},
				"n2": {sc4},
			},
			CommitComments: map[string][]*types.CommitComment{
				"c1": {cc1, cc3, cc2},
				"c2": {cc4},
			},
		},
		{
			Repo: r2,
			TaskComments: map[string]map[string][]*types.TaskComment{
				"c3": {
					"n3": {tc6copy},
				},
			},
			TaskSpecComments: map[string][]*types.TaskSpecComment{
				"n3": {sc5copy},
			},
			CommitComments: map[string][]*types.CommitComment{
				"c3": {cc5copy},
			},
		},
	}
	{
		actual, err := db.GetCommentsForRepos(ctx, []string{r0, r1, r2}, now.Add(-10000*time.Hour))
		require.NoError(t, err)
		AssertDeepEqual(t, expected, actual)
	}

	// Specifying a cutoff time shouldn't drop required comments.
	{
		actual, err := db.GetCommentsForRepos(ctx, []string{r1}, now.Add(-time.Second))
		require.NoError(t, err)
		require.Equal(t, 1, len(actual))
		{
			tcs := actual[0].TaskComments["c1"]["n1"]
			require.True(t, len(tcs) >= 2)
			offset := 0
			if !tcs[0].Timestamp.Equal(tc3.Timestamp) {
				offset = 1
			}
			AssertDeepEqual(t, tc3, tcs[offset])
			AssertDeepEqual(t, tc2, tcs[offset+1])
		}
		{
			scs := actual[0].TaskSpecComments["n1"]
			require.True(t, len(scs) >= 2)
			offset := 0
			if !scs[0].Timestamp.Equal(sc3.Timestamp) {
				offset = 1
			}
			AssertDeepEqual(t, sc3, scs[offset])
			AssertDeepEqual(t, sc2, scs[offset+1])
		}
		{
			ccs := actual[0].CommitComments["c1"]
			require.True(t, len(ccs) >= 2)
			offset := 0
			if !ccs[0].Timestamp.Equal(cc3.Timestamp) {
				offset = 1
			}
			AssertDeepEqual(t, cc3, ccs[offset])
			AssertDeepEqual(t, cc2, ccs[offset+1])
		}
	}

	// Delete some comments.
	require.NoError(t, db.DeleteTaskComment(ctx, tc3))
	require.NoError(t, db.DeleteTaskSpecComment(ctx, sc3))
	require.NoError(t, db.DeleteCommitComment(ctx, cc3))
	// Delete should only look at the ID fields.
	require.NoError(t, db.DeleteTaskComment(ctx, tc1different))
	require.NoError(t, db.DeleteTaskSpecComment(ctx, sc1different))
	require.NoError(t, db.DeleteCommitComment(ctx, cc1different))
	// Delete of nonexistent task should succeed.
	require.NoError(t, db.DeleteTaskComment(ctx, types.MakeTaskComment(99, 1, 1, 1, now.Add(99*time.Second))))
	require.NoError(t, db.DeleteTaskComment(ctx, types.MakeTaskComment(99, 1, 1, 99, now)))
	require.NoError(t, db.DeleteTaskComment(ctx, types.MakeTaskComment(99, 1, 99, 1, now)))
	require.NoError(t, db.DeleteTaskComment(ctx, types.MakeTaskComment(99, 99, 1, 1, now)))
	require.NoError(t, db.DeleteTaskSpecComment(ctx, types.MakeTaskSpecComment(99, 1, 1, now.Add(99*time.Second))))
	require.NoError(t, db.DeleteTaskSpecComment(ctx, types.MakeTaskSpecComment(99, 1, 99, now)))
	require.NoError(t, db.DeleteTaskSpecComment(ctx, types.MakeTaskSpecComment(99, 99, 1, now)))
	require.NoError(t, db.DeleteCommitComment(ctx, types.MakeCommitComment(99, 1, 1, now.Add(99*time.Second))))
	require.NoError(t, db.DeleteCommitComment(ctx, types.MakeCommitComment(99, 1, 99, now)))
	require.NoError(t, db.DeleteCommitComment(ctx, types.MakeCommitComment(99, 99, 1, now)))

	expected[1].TaskComments["c1"]["n1"] = []*types.TaskComment{tc2}
	expected[1].TaskSpecComments["n1"] = []*types.TaskSpecComment{sc2}
	expected[1].CommitComments["c1"] = []*types.CommitComment{cc2}
	{
		actual, err := db.GetCommentsForRepos(ctx, []string{r0, r1, r2}, now.Add(-10000*time.Hour))
		require.NoError(t, err)
		AssertDeepEqual(t, expected, actual)
		deleted := true
		tc1.Deleted = &deleted
		sc1.Deleted = &deleted
		cc1.Deleted = &deleted
		tc3.Deleted = &deleted
		sc3.Deleted = &deleted
		cc3.Deleted = &deleted
		findModifiedComments(t, modTC, modTSC, modCC, []*types.TaskComment{tc1, tc3}, []*types.TaskSpecComment{sc1, sc3}, []*types.CommitComment{cc1, cc3})
	}

	// Delete all the comments.
	for _, c := range []*types.TaskComment{tc2, tc4, tc5, tc6} {
		require.NoError(t, db.DeleteTaskComment(ctx, c))
	}
	for _, c := range []*types.TaskSpecComment{sc2, sc4, sc5} {
		require.NoError(t, db.DeleteTaskSpecComment(ctx, c))
	}
	for _, c := range []*types.CommitComment{cc2, cc4, cc5} {
		require.NoError(t, db.DeleteCommitComment(ctx, c))
	}
	{
		actual, err := db.GetCommentsForRepos(ctx, []string{r0, r1, r2}, now.Add(-10000*time.Hour))
		require.NoError(t, err)
		require.Equal(t, 3, len(actual))
		require.Equal(t, r0, actual[0].Repo)
		require.Equal(t, r1, actual[1].Repo)
		require.Equal(t, r2, actual[2].Repo)
		for _, rc := range actual {
			require.Equal(t, 0, len(rc.TaskComments))
			require.Equal(t, 0, len(rc.TaskSpecComments))
			require.Equal(t, 0, len(rc.CommitComments))
		}
	}
}

func TestTaskDBGetTasksFromDateRangeByRepo(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()
	r1 := common.REPO_SKIA
	r2 := common.REPO_SKIA_INFRA
	r3 := common.REPO_CHROMIUM
	repos := []string{r1, r2, r3}
	start := time.Now().Add(-50 * TS_RESOLUTION)
	end := start
	for _, repo := range repos {
		for i := 0; i < 10; i++ {
			task := types.MakeTestTask(end, []string{"c"})
			task.Repo = repo
			require.NoError(t, db.PutTask(ctx, task))
			end = end.Add(TS_RESOLUTION)
		}
	}
	tasks, err := db.GetTasksFromDateRange(ctx, start, end, "")
	require.NoError(t, err)
	require.Equal(t, 30, len(tasks))
	require.True(t, sort.IsSorted(types.TaskSlice(tasks)))
	for _, repo := range repos {
		tasks, err := db.GetTasksFromDateRange(ctx, start, end, repo)
		require.NoError(t, err)
		require.Equal(t, 10, len(tasks))
		require.True(t, sort.IsSorted(types.TaskSlice(tasks)))
		for _, task := range tasks {
			require.Equal(t, repo, task.Repo)
		}
	}
}

func TestTaskDBGetTasksFromWindow(t sktest.TestingT, db TaskDB) {
	ctx := cipd_git.UseGitFinder(context.Background())
	now := time.Now()
	timeWindow := 24 * time.Hour
	// Offset commit timestamps for different repos to ensure that we get
	// a consistent sorting order.
	repoOffset := time.Minute
	curOffset := repoOffset
	f := "somefile"
	setup := func(numCommits int) (string, *repograph.Graph, func()) {
		gb := git_testutils.GitInit(t, ctx)
		repoUrl := gb.RepoUrl()
		t0 := now.Add(-timeWindow).Add(curOffset)
		for i := 0; i < numCommits; i++ {
			ts := t0.Add(time.Duration(i) * timeWindow / time.Duration(numCommits))
			gb.AddGen(ctx, f)
			hash := gb.CommitMsgAt(ctx, fmt.Sprintf("Commit %d", i), ts)
			task := types.MakeTestTask(ts, []string{hash})
			task.Repo = repoUrl
			require.NoError(t, db.PutTask(ctx, task))
		}
		tmp, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		repo, err := repograph.NewLocalGraph(ctx, gb.Dir(), tmp)
		require.NoError(t, err)
		require.NoError(t, repo.Update(ctx))
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
		w, err := window.New(ctx, windowSize, numCommits, repos)
		require.NoError(t, err)
		tasks, err := GetTasksFromWindow(ctx, db, w)
		require.NoError(t, err)
		require.Equal(t, expectTasks, len(tasks))
		require.True(t, sort.IsSorted(types.TaskSlice(tasks)))
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

func TestUpdateDBFromSwarmingTask(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()
	// Create task, initialize from swarming, and save.
	now := time.Now().UTC().Round(time.Microsecond)
	task := &types.Task{
		TaskKey: types.TaskKey{
			RepoState: types.RepoState{
				Repo:     "C",
				Revision: "D",
			},
			Name:        "B",
			ForcedJobId: "G",
		},
		Commits:        []string{"D", "Z"},
		Status:         types.TASK_STATUS_PENDING,
		ParentTaskIds:  []string{"E", "F"},
		SwarmingTaskId: "E",
	}
	require.NoError(t, db.AssignId(ctx, task))

	s := &types.TaskResult{
		ID:      "E", // This is the Swarming TaskId.
		Created: now.Add(time.Second),
		Status:  types.TASK_STATUS_PENDING,
		Tags: map[string][]string{
			types.SWARMING_TAG_ID:             {task.Id},
			types.SWARMING_TAG_NAME:           {"B"},
			types.SWARMING_TAG_REPO:           {"C"},
			types.SWARMING_TAG_REVISION:       {"D"},
			types.SWARMING_TAG_PARENT_TASK_ID: {"E", "F"},
			types.SWARMING_TAG_FORCED_JOB_ID:  {"G"},
		},
	}
	modified, err := task.UpdateFromTaskResult(s)
	require.NoError(t, err)
	require.True(t, modified)
	require.NoError(t, db.PutTask(ctx, task))

	// Get update from Swarming.
	s.Started = now.Add(time.Minute)
	s.Finished = now.Add(2 * time.Minute)
	s.Status = types.TASK_STATUS_FAILURE
	s.CasOutput = "aaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccdddd/32"
	s.MachineID = "H"

	modified, err = UpdateDBFromTaskResult(ctx, db, s)
	require.NoError(t, err)
	require.True(t, modified)

	updatedTask, err := db.GetTaskById(ctx, task.Id)
	require.NoError(t, err)
	assertdeep.Equal(t, updatedTask, &types.Task{
		Id: task.Id,
		TaskKey: types.TaskKey{
			RepoState: types.RepoState{
				Repo:     "C",
				Revision: "D",
			},
			Name:        "B",
			ForcedJobId: "G",
		},
		Created:        now.Add(time.Second),
		Commits:        []string{"D", "Z"},
		Started:        now.Add(time.Minute),
		Finished:       now.Add(2 * time.Minute),
		Status:         types.TASK_STATUS_FAILURE,
		SwarmingTaskId: "E",
		IsolatedOutput: "aaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccdddd/32",
		SwarmingBotId:  "H",
		ParentTaskIds:  []string{"E", "F"},
		// Use value from updatedTask so they are deep-equal.
		DbModified: updatedTask.DbModified,
	})

	modified, err = UpdateDBFromTaskResult(ctx, db, s)
	require.NoError(t, err)
	require.False(t, modified)
}

func TestUpdateDBFromSwarmingTaskTryJob(t sktest.TestingT, db TaskDB) {
	ctx := context.Background()
	// Create task, initialize from swarming, and save.
	now := time.Now().UTC().Round(time.Microsecond)
	task := &types.Task{
		TaskKey: types.TaskKey{
			RepoState: types.RepoState{
				Patch: types.Patch{
					Server:   "A",
					Issue:    "B",
					Patchset: "P",
				},
				Repo:     "C",
				Revision: "D",
			},
			Name:        "B",
			ForcedJobId: "G",
		},
		Commits:        []string{"D", "Z"},
		Status:         types.TASK_STATUS_PENDING,
		ParentTaskIds:  []string{"E", "F"},
		SwarmingTaskId: "E",
	}
	require.NoError(t, db.AssignId(ctx, task))

	s := &types.TaskResult{
		ID:      "E", // This is the Swarming TaskId.
		Created: now.Add(time.Second),
		Status:  types.TASK_STATUS_PENDING,
		Tags: map[string][]string{
			types.SWARMING_TAG_ID:             {task.Id},
			types.SWARMING_TAG_NAME:           {"B"},
			types.SWARMING_TAG_REPO:           {"C"},
			types.SWARMING_TAG_REVISION:       {"D"},
			types.SWARMING_TAG_PARENT_TASK_ID: {"E", "F"},
			types.SWARMING_TAG_FORCED_JOB_ID:  {"G"},
			types.SWARMING_TAG_SERVER:         {"A"},
			types.SWARMING_TAG_ISSUE:          {"B"},
			types.SWARMING_TAG_PATCHSET:       {"P"},
		},
	}
	modified, err := task.UpdateFromTaskResult(s)
	require.NoError(t, err)
	require.True(t, modified)

	// Make sure we can't change the server, issue, or patchset.
	s = &types.TaskResult{
		ID:      "E", // This is the Swarming TaskId.
		Created: now.Add(time.Second),
		Status:  types.TASK_STATUS_PENDING,
		Tags: map[string][]string{
			types.SWARMING_TAG_ID:             {task.Id},
			types.SWARMING_TAG_NAME:           {"B"},
			types.SWARMING_TAG_REPO:           {"C"},
			types.SWARMING_TAG_REVISION:       {"D"},
			types.SWARMING_TAG_PARENT_TASK_ID: {"E", "F"},
			types.SWARMING_TAG_FORCED_JOB_ID:  {"G"},
			types.SWARMING_TAG_SERVER:         {"BAD"},
			types.SWARMING_TAG_ISSUE:          {"B"},
			types.SWARMING_TAG_PATCHSET:       {"P"},
		},
	}
	modified, err = task.UpdateFromTaskResult(s)
	require.NotNil(t, err)
	require.False(t, modified)

	// Make sure we can't change the server, issue, or patchset.
	s = &types.TaskResult{
		ID:      "E", // This is the Swarming TaskId.
		Created: now.Add(time.Second),
		Status:  types.TASK_STATUS_PENDING,
		Tags: map[string][]string{
			types.SWARMING_TAG_ID:             {task.Id},
			types.SWARMING_TAG_NAME:           {"B"},
			types.SWARMING_TAG_REPO:           {"C"},
			types.SWARMING_TAG_REVISION:       {"D"},
			types.SWARMING_TAG_PARENT_TASK_ID: {"E", "F"},
			types.SWARMING_TAG_FORCED_JOB_ID:  {"G"},
			types.SWARMING_TAG_SERVER:         {"A"},
			types.SWARMING_TAG_ISSUE:          {"BAD"},
			types.SWARMING_TAG_PATCHSET:       {"P"},
		},
	}
	modified, err = task.UpdateFromTaskResult(s)
	require.NotNil(t, err)
	require.False(t, modified)

	// Make sure we can't change the server, issue, or patchset.
	s = &types.TaskResult{
		ID:      "E", // This is the Swarming TaskId.
		Created: now.Add(time.Second),
		Status:  types.TASK_STATUS_PENDING,
		Tags: map[string][]string{
			types.SWARMING_TAG_ID:             {task.Id},
			types.SWARMING_TAG_NAME:           {"B"},
			types.SWARMING_TAG_REPO:           {"C"},
			types.SWARMING_TAG_REVISION:       {"D"},
			types.SWARMING_TAG_PARENT_TASK_ID: {"E", "F"},
			types.SWARMING_TAG_FORCED_JOB_ID:  {"G"},
			types.SWARMING_TAG_SERVER:         {"A"},
			types.SWARMING_TAG_ISSUE:          {"B"},
			types.SWARMING_TAG_PATCHSET:       {"BAD"},
		},
	}
	modified, err = task.UpdateFromTaskResult(s)
	require.NotNil(t, err)
	require.False(t, modified)
}
