package cache

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/memory"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

func testGetTasksForCommits(t *testing.T, c TaskCache, b *types.Task) {
	for _, commit := range b.Commits {
		found, err := c.GetTaskForCommit(types.DEFAULT_TEST_REPO, commit, b.Name)
		require.NoError(t, err)
		assertdeep.Equal(t, b, found)

		tasks, err := c.GetTasksForCommits(types.DEFAULT_TEST_REPO, []string{commit})
		require.NoError(t, err)
		assertdeep.Equal(t, b, tasks[commit][b.Name])
	}
}

func TestTaskCache(t *testing.T) {
	unittest.SmallTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := memory.NewInMemoryTaskDB()

	// Pre-load a task into the DB.
	startTime := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	t1 := types.MakeTestTask(startTime, []string{"a", "b", "c", "d"})
	require.NoError(t, d.PutTask(ctx, t1))
	d.Wait()

	// Create the cache. Ensure that the existing task is present.
	w, err := window.New(time.Hour, 0, nil)
	require.NoError(t, err)
	wait := make(chan struct{})
	c, err := NewTaskCache(ctx, d, w, func() {
		wait <- struct{}{}
	})
	require.NoError(t, err)
	<-wait
	testGetTasksForCommits(t, c, t1)

	// Bisect the first task.
	t2 := types.MakeTestTask(startTime.Add(time.Minute), []string{"c", "d"})
	t1.Commits = []string{"a", "b"}
	require.NoError(t, d.PutTasks(ctx, []*types.Task{t2, t1}))
	d.Wait()
	<-wait
	require.NoError(t, c.Update(ctx))

	// Ensure that t2 (and not t1) shows up for commits "c" and "d".
	testGetTasksForCommits(t, c, t1)
	testGetTasksForCommits(t, c, t2)

	// Insert a task on a second bot.
	t3 := types.MakeTestTask(startTime.Add(2*time.Minute), []string{"a", "b"})
	t3.Name = "Another-Task"
	require.NoError(t, d.PutTask(ctx, t3))
	d.Wait()
	<-wait
	require.NoError(t, c.Update(ctx))
	tasks, err := c.GetTasksForCommits(types.DEFAULT_TEST_REPO, []string{"b"})
	require.NoError(t, err)
	assertdeep.Equal(t, map[string]map[string]*types.Task{
		"b": {
			t1.Name: t1,
			t3.Name: t3,
		},
	}, tasks)

	// Ensure that we don't insert outdated entries.
	old := t1.Copy()
	require.False(t, util.TimeIsZero(old.DbModified))
	old.Name = "outdated"
	old.DbModified = old.DbModified.Add(-time.Hour)
	c.(*taskCache).modified[old.Id] = old
	require.NoError(t, c.Update(ctx))
	got, err := c.GetTask(old.Id)
	require.NoError(t, err)
	assertdeep.Equal(t, got, t1)
}

func TestTaskCacheKnownTaskName(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := memory.NewInMemoryTaskDB()
	w, err := window.New(time.Hour, 0, nil)
	require.NoError(t, err)
	wait := make(chan struct{})
	c, err := NewTaskCache(ctx, d, w, func() {
		wait <- struct{}{}
	})
	require.NoError(t, err)
	<-wait

	// Try jobs don't count toward KnownTaskName.
	startTime := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	t1 := types.MakeTestTask(startTime, []string{"a", "b", "c", "d"})
	t1.Server = "fake-server"
	t1.Issue = "fake-issue"
	t1.Patchset = "fake-patchset"
	require.NoError(t, d.PutTask(ctx, t1))
	d.Wait()
	<-wait
	require.NoError(t, c.Update(ctx))
	require.False(t, c.KnownTaskName(t1.Repo, t1.Name))

	// Forced jobs don't count toward KnownTaskName.
	t2 := types.MakeTestTask(startTime, []string{"a", "b", "c", "d"})
	t2.ForcedJobId = "job-id"
	require.NoError(t, d.PutTask(ctx, t2))
	d.Wait()
	<-wait
	require.NoError(t, c.Update(ctx))
	require.False(t, c.KnownTaskName(t2.Repo, t2.Name))

	// Normal task.
	t3 := types.MakeTestTask(startTime, []string{"a", "b", "c", "d"})
	require.NoError(t, d.PutTask(ctx, t3))
	d.Wait()
	<-wait
	require.NoError(t, c.Update(ctx))
	require.True(t, c.KnownTaskName(t3.Repo, t3.Name))
}

func TestTaskCacheGetTasksFromDateRange(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := memory.NewInMemoryTaskDB()

	// Pre-load a task into the DB.
	timeStart := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	t1 := types.MakeTestTask(timeStart.Add(time.Nanosecond), []string{"a", "b", "c", "d"})
	require.NoError(t, d.PutTask(ctx, t1))
	d.Wait()

	// Create the cache.
	w, err := window.New(time.Hour, 0, nil)
	require.NoError(t, err)
	wait := make(chan struct{})
	c, err := NewTaskCache(ctx, d, w, func() {
		wait <- struct{}{}
	})
	require.NoError(t, err)
	<-wait

	// Insert two more tasks. Ensure at least 1 nanosecond between task Created
	// times so that t1After != t2Before and t2After != t3Before.
	t2 := types.MakeTestTask(timeStart.Add(2*time.Nanosecond), []string{"e", "f"})
	t3 := types.MakeTestTask(timeStart.Add(3*time.Nanosecond), []string{"g", "h"})
	require.NoError(t, d.PutTasks(ctx, []*types.Task{t2, t3}))
	d.Wait()
	<-wait
	require.NoError(t, c.Update(ctx))

	// Ensure that all tasks show up in the correct time ranges, in sorted order.
	t1Before := t1.Created
	t1After := t1Before.Add(1 * time.Nanosecond)

	t2Before := t2.Created
	t2After := t2Before.Add(1 * time.Nanosecond)

	t3Before := t3.Created
	t3After := t3Before.Add(1 * time.Nanosecond)

	timeEnd := timeStart.Add(4 * time.Nanosecond)

	tasks, err := c.GetTasksFromDateRange(timeStart, t1Before)
	require.NoError(t, err)
	require.Equal(t, 0, len(tasks))

	tasks, err = c.GetTasksFromDateRange(timeStart, t1After)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{t1}, tasks)

	tasks, err = c.GetTasksFromDateRange(timeStart, t2Before)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{t1}, tasks)

	tasks, err = c.GetTasksFromDateRange(timeStart, t2After)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{t1, t2}, tasks)

	tasks, err = c.GetTasksFromDateRange(timeStart, t3Before)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{t1, t2}, tasks)

	tasks, err = c.GetTasksFromDateRange(timeStart, t3After)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{t1, t2, t3}, tasks)

	tasks, err = c.GetTasksFromDateRange(timeStart, timeEnd)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{t1, t2, t3}, tasks)

	tasks, err = c.GetTasksFromDateRange(t1Before, timeEnd)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{t1, t2, t3}, tasks)

	tasks, err = c.GetTasksFromDateRange(t1After, timeEnd)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{t2, t3}, tasks)

	tasks, err = c.GetTasksFromDateRange(t2Before, timeEnd)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{t2, t3}, tasks)

	tasks, err = c.GetTasksFromDateRange(t2After, timeEnd)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{t3}, tasks)

	tasks, err = c.GetTasksFromDateRange(t3Before, timeEnd)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{t3}, tasks)

	tasks, err = c.GetTasksFromDateRange(t3After, timeEnd)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{}, tasks)
}

func TestTaskCacheMultiRepo(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := memory.NewInMemoryTaskDB()

	// Insert several tasks with different repos.
	startTime := time.Now().Add(-30 * time.Minute)          // Arbitrary starting point.
	t1 := types.MakeTestTask(startTime, []string{"a", "b"}) // Default Repo.
	t2 := types.MakeTestTask(startTime, []string{"a", "b"})
	t2.Repo = "thats-what-you.git"
	t3 := types.MakeTestTask(startTime, []string{"b", "c"})
	t3.Repo = "never-for.git"
	require.NoError(t, d.PutTasks(ctx, []*types.Task{t1, t2, t3}))
	d.Wait()

	// Create the cache.
	w, err := window.New(time.Hour, 0, nil)
	require.NoError(t, err)
	c, err := NewTaskCache(ctx, d, w, nil)
	require.NoError(t, err)

	// Check that there's no conflict among the tasks in different repos.
	{
		tasks, err := c.GetTasksForCommits(t1.Repo, []string{"a", "b", "c"})
		require.NoError(t, err)
		assertdeep.Equal(t, map[string]map[string]*types.Task{
			"a": {
				t1.Name: t1,
			},
			"b": {
				t1.Name: t1,
			},
			"c": {},
		}, tasks)
	}

	{
		tasks, err := c.GetTasksForCommits(t2.Repo, []string{"a", "b", "c"})
		require.NoError(t, err)
		assertdeep.Equal(t, map[string]map[string]*types.Task{
			"a": {
				t1.Name: t2,
			},
			"b": {
				t1.Name: t2,
			},
			"c": {},
		}, tasks)
	}

	{
		tasks, err := c.GetTasksForCommits(t3.Repo, []string{"a", "b", "c"})
		require.NoError(t, err)
		assertdeep.Equal(t, map[string]map[string]*types.Task{
			"a": {},
			"b": {
				t1.Name: t3,
			},
			"c": {
				t1.Name: t3,
			},
		}, tasks)
	}
}

func TestTaskCacheUnfinished(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := memory.NewInMemoryTaskDB()

	// Insert a task.
	startTime := time.Now().Add(-30 * time.Minute)
	t1 := types.MakeTestTask(startTime, []string{"a"})
	require.False(t, t1.Done())
	require.NoError(t, d.PutTask(ctx, t1))
	d.Wait()

	// Add a pending task with no swarming ID to test that it won't appear
	// in UnfinishedTasks.
	fakeTask := types.MakeTestTask(startTime, []string{"b"})
	fakeTask.SwarmingTaskId = ""
	require.NoError(t, d.PutTask(ctx, fakeTask))
	d.Wait()

	// Create the cache. Ensure that the existing task is present.
	w, err := window.New(time.Hour, 0, nil)
	require.NoError(t, err)
	wait := make(chan struct{})
	c, err := NewTaskCache(ctx, d, w, func() {
		wait <- struct{}{}
	})
	require.NoError(t, err)
	<-wait
	tasks, err := c.UnfinishedTasks()
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{t1}, tasks)

	// Finish the task. Insert it, ensure that it's not unfinished.
	t1.Status = types.TASK_STATUS_SUCCESS
	require.True(t, t1.Done())
	require.NoError(t, d.PutTask(ctx, t1))
	d.Wait()
	<-wait
	require.NoError(t, c.Update(ctx))
	tasks, err = c.UnfinishedTasks()
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{}, tasks)

	// Already-finished task.
	t2 := types.MakeTestTask(time.Now(), []string{"a"})
	t2.Status = types.TASK_STATUS_MISHAP
	require.True(t, t2.Done())
	require.NoError(t, d.PutTask(ctx, t2))
	d.Wait()
	<-wait
	require.NoError(t, c.Update(ctx))
	tasks, err = c.UnfinishedTasks()
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{}, tasks)

	// An unfinished task, created after the cache was created.
	t3 := types.MakeTestTask(time.Now(), []string{"b"})
	require.False(t, t3.Done())
	require.NoError(t, d.PutTask(ctx, t3))
	d.Wait()
	<-wait
	require.NoError(t, c.Update(ctx))
	tasks, err = c.UnfinishedTasks()
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{t3}, tasks)

	// Update the task.
	t3.Commits = []string{"c", "d", "f"}
	require.False(t, t3.Done())
	require.NoError(t, d.PutTask(ctx, t3))
	d.Wait()
	<-wait
	require.NoError(t, c.Update(ctx))
	tasks, err = c.UnfinishedTasks()
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Task{t3}, tasks)
}

// assertTaskInSlice fails the test if task is not deep-equal to an element of
// slice.
func assertTaskInSlice(t *testing.T, task *types.Task, slice []*types.Task) {
	for _, other := range slice {
		if task.Id == other.Id {
			assertdeep.Equal(t, task, other)
			return
		}
	}
	t.Fatalf("Did not find task %v in %v.", task, slice)
}

// assertTasksNotCached checks that none of tasks are retrievable from c.
func assertTasksNotCached(t *testing.T, c TaskCache, tasks []*types.Task) {
	byTimeTasks, err := c.GetTasksFromDateRange(time.Date(1900, time.January, 1, 0, 0, 0, 0, time.UTC), time.Date(9999, time.January, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	unfinishedTasks, err := c.UnfinishedTasks()
	require.NoError(t, err)
	for _, task := range tasks {
		_, err := c.GetTask(task.Id)
		require.Error(t, err)
		require.True(t, db.IsNotFound(err))
		for _, commit := range task.Commits {
			found, err := c.GetTaskForCommit(types.DEFAULT_TEST_REPO, commit, task.Name)
			require.NoError(t, err)
			require.Nil(t, found)
			tasks, err := c.GetTasksForCommits(types.DEFAULT_TEST_REPO, []string{commit})
			require.NoError(t, err)
			_, ok := tasks[commit][task.Name]
			require.False(t, ok)
		}
		for _, other := range byTimeTasks {
			if task.Id == other.Id {
				t.Errorf("Found unexpected task %v in GetTasksFromDateRange.", task)
			}
		}
		for _, other := range unfinishedTasks {
			if task.Id == other.Id {
				t.Errorf("Found unexpected task %v in UnfinishedTasks.", task)
			}
		}
	}
}

func TestTaskCacheExpiration(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := memory.NewInMemoryTaskDB()

	period := 10 * time.Minute
	w, err := window.New(period, 0, nil)
	require.NoError(t, err)
	timeStart := w.EarliestStart()

	// Make a bunch of tasks with various timestamps.
	mk := func(mins int, name string, blame []string) *types.Task {
		t := types.MakeTestTask(timeStart.Add(time.Duration(mins)*time.Minute), blame)
		t.Name = name
		return t
	}

	tasks := []*types.Task{
		mk(1, "Old", []string{"a"}),                   // 0
		mk(1, "Build1", []string{"a", "b"}),           // 1
		mk(2, "Build1", []string{"c"}),                // 2
		mk(2, "Build2", []string{"c"}),                // 3
		mk(3, "Build1", []string{"d"}),                // 4
		mk(4, "Build2", []string{"d"}),                // 5
		mk(5, "Build3", []string{"a", "b", "c", "d"}), // 6
	}
	require.NoError(t, d.PutTasks(ctx, tasks))
	d.Wait()

	// Create the cache.
	wait := make(chan struct{})
	c, err := NewTaskCache(ctx, d, w, func() {
		wait <- struct{}{}
	})
	require.NoError(t, err)
	<-wait

	{
		// Check that tasks[0] and tasks[1] are in the cache.
		firstTasks, err := c.GetTasksFromDateRange(timeStart, timeStart.Add(2*time.Minute))
		require.NoError(t, err)
		require.Equal(t, 2, len(firstTasks))
		unfinishedTasks, err := c.UnfinishedTasks()
		require.NoError(t, err)
		for _, task := range []*types.Task{tasks[0], tasks[1]} {
			cachedTask, err := c.GetTask(task.Id)
			require.NoError(t, err)
			assertdeep.Equal(t, task, cachedTask)
			testGetTasksForCommits(t, c, task)
			assertTaskInSlice(t, task, firstTasks)
			assertTaskInSlice(t, task, unfinishedTasks)
			require.True(t, c.KnownTaskName(task.Repo, task.Name))
		}
	}

	// Add and update tasks.
	tasks[1].Status = types.TASK_STATUS_SUCCESS
	tasks[6].Commits = []string{"c", "d"}
	tasks = append(tasks,
		mk(7, "Build3", []string{"a", "b"}),           // 7
		mk(4, "Build4", []string{"a", "b", "c", "d"})) // 8
	// Out of order to test TaskDB.GetModifiedTasks.
	require.NoError(t, d.PutTasks(ctx, []*types.Task{tasks[6], tasks[8], tasks[1], tasks[7]}))
	d.Wait()
	<-wait

	// update, expiring tasks[0] and tasks[1].
	require.NoError(t, w.UpdateWithTime(tasks[0].Created.Add(period).Add(time.Nanosecond)))
	require.NoError(t, c.Update(ctx))

	{
		// Check that tasks[0] and tasks[1] are no longer in the cache.
		firstTasks, err := c.GetTasksFromDateRange(timeStart, timeStart.Add(2*time.Minute))
		require.NoError(t, err)
		require.Equal(t, 0, len(firstTasks))

		assertTasksNotCached(t, c, []*types.Task{tasks[0], tasks[1]})

		// tasks[0].Name is no longer known, but there are later Tasks for tasks[1].Name.
		require.False(t, c.KnownTaskName(tasks[0].Repo, tasks[0].Name))
		require.True(t, c.KnownTaskName(tasks[1].Repo, tasks[1].Name))
	}

	{
		// Check that other tasks are still cached correctly.
		allCachedTasks, err := c.GetTasksFromDateRange(timeStart, timeStart.Add(20*time.Minute))
		require.NoError(t, err)
		orderedTasks := []*types.Task{tasks[2], tasks[3], tasks[4], tasks[5], tasks[8], tasks[6], tasks[7]}
		// Tasks with same timestamp can be in either order.
		if allCachedTasks[0].Id != orderedTasks[0].Id {
			allCachedTasks[0], allCachedTasks[1] = allCachedTasks[1], allCachedTasks[0]
		}
		if allCachedTasks[3].Id != orderedTasks[3].Id {
			allCachedTasks[3], allCachedTasks[4] = allCachedTasks[4], allCachedTasks[3]
		}
		assertdeep.Equal(t, orderedTasks, allCachedTasks)

		unfinishedTasks, err := c.UnfinishedTasks()
		require.NoError(t, err)
		for _, task := range orderedTasks {
			cachedTask, err := c.GetTask(task.Id)
			require.NoError(t, err)
			assertdeep.Equal(t, task, cachedTask)
			testGetTasksForCommits(t, c, task)
			assertTaskInSlice(t, task, unfinishedTasks)
			require.True(t, c.KnownTaskName(task.Repo, task.Name))
		}
	}

	// Test entire cache expiration.
	newTasks := []*types.Task{
		mk(11, "Build3", []string{"e"}),
	}
	require.NoError(t, d.PutTasks(ctx, newTasks))
	d.Wait()
	<-wait
	require.NoError(t, w.UpdateWithTime(newTasks[0].Created.Add(period)))
	require.NoError(t, c.Update(ctx))

	{
		// Check that only new task is in the cache.
		assertTasksNotCached(t, c, tasks)

		for _, task := range newTasks {
			cachedTask, err := c.GetTask(task.Id)
			require.NoError(t, err)
			assertdeep.Equal(t, task, cachedTask)
			testGetTasksForCommits(t, c, task)
		}

		allCachedTasks, err := c.GetTasksFromDateRange(timeStart, timeStart.Add(20*time.Minute))
		require.NoError(t, err)
		assertdeep.Equal(t, newTasks, allCachedTasks)

		// Only new task is known.
		require.True(t, c.KnownTaskName(newTasks[0].Repo, newTasks[0].Name))
		for _, name := range []string{"Old", "Build1", "Build2"} {
			require.False(t, c.KnownTaskName(types.DEFAULT_TEST_REPO, name))
		}

		unfinishedTasks, err := c.UnfinishedTasks()
		require.NoError(t, err)
		assertdeep.Equal(t, newTasks, unfinishedTasks)
	}
}

func TestJobCache(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := memory.NewInMemoryJobDB()

	// Pre-load a job into the DB.
	startTime := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	j1 := types.MakeTestJob(startTime)
	require.NoError(t, d.PutJob(ctx, j1))
	d.Wait()

	// Create the cache. Ensure that the existing job is present.
	w, err := window.New(time.Hour, 0, nil)
	require.NoError(t, err)
	wait := make(chan struct{})
	c, err := NewJobCache(ctx, d, w, func() {
		wait <- struct{}{}
	})
	require.NoError(t, err)
	<-wait
	test, err := c.GetJob(j1.Id)
	require.NoError(t, err)
	assertdeep.Equal(t, j1, test)
	jobs, err := c.GetJobsByRepoState(j1.Name, j1.RepoState)
	require.NoError(t, err)
	require.Equal(t, 1, len(jobs))
	assertdeep.Equal(t, jobs[0], test)

	// Create another job. Ensure that it gets picked up.
	j2 := types.MakeTestJob(startTime.Add(time.Nanosecond))
	require.NoError(t, d.PutJob(ctx, j2))
	d.Wait()
	<-wait
	test, err = c.GetJob(j2.Id)
	require.Error(t, err)
	require.NoError(t, c.Update(ctx))
	test, err = c.GetJob(j2.Id)
	require.NoError(t, err)
	assertdeep.Equal(t, j2, test)
	jobs, err = c.GetJobsByRepoState(j2.Name, j2.RepoState)
	require.NoError(t, err)
	require.Equal(t, 2, len(jobs))
	assertdeep.Equal(t, jobs[1], test)

	// Ensure that we don't insert outdated entries.
	old := j1.Copy()
	require.False(t, util.TimeIsZero(old.DbModified))
	old.Name = "outdated"
	old.DbModified = old.DbModified.Add(-time.Hour)
	c.(*jobCache).modified[old.Id] = old
	require.NoError(t, c.Update(ctx))
	got, err := c.GetJob(old.Id)
	require.NoError(t, err)
	assertdeep.Equal(t, got, j1)
}

func testGetUnfinished(t *testing.T, expect []*types.Job, cache JobCache) {
	jobs, err := cache.UnfinishedJobs()
	require.NoError(t, err)
	sort.Sort(types.JobSlice(jobs))
	sort.Sort(types.JobSlice(expect))
	assertdeep.Equal(t, expect, jobs)
}

func TestJobCacheUnfinished(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := memory.NewInMemoryJobDB()

	// Insert a job.
	startTime := time.Now().Add(-30 * time.Minute)
	j1 := types.MakeTestJob(startTime)
	require.False(t, j1.Done())
	require.NoError(t, d.PutJob(ctx, j1))
	d.Wait()

	// Create the cache. Ensure that the existing job is present.
	w, err := window.New(time.Hour, 0, nil)
	require.NoError(t, err)
	wait := make(chan struct{})
	c, err := NewJobCache(ctx, d, w, func() {
		wait <- struct{}{}
	})
	require.NoError(t, err)
	<-wait
	testGetUnfinished(t, []*types.Job{j1}, c)

	// Finish the job. Insert it, ensure that it's not unfinished.
	j1.Status = types.JOB_STATUS_SUCCESS
	require.True(t, j1.Done())
	require.NoError(t, d.PutJob(ctx, j1))
	d.Wait()
	<-wait
	require.NoError(t, c.Update(ctx))
	testGetUnfinished(t, []*types.Job{}, c)

	// Already-finished job.
	j2 := types.MakeTestJob(time.Now())
	j2.Status = types.JOB_STATUS_MISHAP
	require.True(t, j2.Done())
	require.NoError(t, d.PutJob(ctx, j2))
	d.Wait()
	<-wait
	require.NoError(t, c.Update(ctx))
	testGetUnfinished(t, []*types.Job{}, c)

	// An unfinished job, created after the cache was created.
	j3 := types.MakeTestJob(time.Now())
	require.False(t, j3.Done())
	require.NoError(t, d.PutJob(ctx, j3))
	d.Wait()
	<-wait
	require.NoError(t, c.Update(ctx))
	testGetUnfinished(t, []*types.Job{j3}, c)

	// Update the job.
	j3.Dependencies = map[string][]string{"a": {}, "b": {}, "c": {}}
	require.False(t, j3.Done())
	require.NoError(t, d.PutJob(ctx, j3))
	d.Wait()
	<-wait
	require.NoError(t, c.Update(ctx))
	testGetUnfinished(t, []*types.Job{j3}, c)
}

// assertJobInSlice fails the test if job is not deep-equal to an element of
// slice.
func assertJobInSlice(t *testing.T, job *types.Job, slice []*types.Job) {
	for _, other := range slice {
		if job.Id == other.Id {
			assertdeep.Equal(t, job, other)
			return
		}
	}
	t.Fatalf("Did not find job %v in %v.", job, slice)
}

// assertJobsCached checks that all of jobs are retrievable from c.
func assertJobsCached(t *testing.T, c JobCache, jobs []*types.Job) {
	unfinishedJobs, err := c.UnfinishedJobs()
	require.NoError(t, err)
	for _, job := range jobs {
		cachedJob, err := c.GetJob(job.Id)
		require.NoError(t, err)
		assertdeep.Equal(t, job, cachedJob)

		if !job.Done() {
			assertJobInSlice(t, job, unfinishedJobs)
		}

		cachedJobs, err := c.GetJobsByRepoState(job.Name, job.RepoState)
		require.NoError(t, err)
		found := false
		for _, otherJob := range cachedJobs {
			if job.Id == otherJob.Id {
				assertdeep.Equal(t, job, otherJob)
				found = true
			}
		}
		require.True(t, found)

		found = false
		jobsByName, err := c.GetMatchingJobsFromDateRange([]string{job.Name}, job.Created, job.Created.Add(time.Nanosecond))
		require.NoError(t, err)
		for _, jobsForName := range jobsByName {
			for _, otherJob := range jobsForName {
				if job.Id == otherJob.Id {
					assertdeep.Equal(t, job, otherJob)
					found = true
				}
			}
		}
		require.True(t, found)
	}
}

// assertJobsNotCached checks that none of jobs are retrievable from c.
func assertJobsNotCached(t *testing.T, c JobCache, jobs []*types.Job) {
	unfinishedJobs, err := c.UnfinishedJobs()
	require.NoError(t, err)
	for _, job := range jobs {
		_, err := c.GetJob(job.Id)
		require.Error(t, err)
		require.True(t, db.IsNotFound(err))

		for _, other := range unfinishedJobs {
			if job.Id == other.Id {
				t.Errorf("Found unexpected job %v in UnfinishedJobs.", job)
			}
		}

		cachedJobs, err := c.GetJobsByRepoState(job.Name, job.RepoState)
		require.NoError(t, err)
		for _, otherJob := range cachedJobs {
			if job.Id == otherJob.Id {
				t.Fatalf("Found unexpected job %v in GetJobsByRepoState", job)
			}
		}

		found := false
		jobsByName, err := c.GetMatchingJobsFromDateRange([]string{job.Name}, time.Time{}, time.Now().Add(10*24*time.Hour))
		require.NoError(t, err)
		for _, jobsForName := range jobsByName {
			for _, otherJob := range jobsForName {
				if job.Id == otherJob.Id {
					found = true
				}
			}
		}
		require.False(t, found)
	}
}

func TestJobCacheExpiration(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := memory.NewInMemoryJobDB()

	period := 10 * time.Minute
	w, err := window.New(period, 0, nil)
	require.NoError(t, err)
	timeStart := w.EarliestStart()

	// Make a bunch of jobs with various revisions.
	mk := func(ts time.Time, isForce bool) *types.Job {
		job := types.MakeTestJob(ts)
		job.IsForce = isForce
		return job
	}

	jobs := []*types.Job{
		mk(timeStart.Add(1*time.Minute), false), // 0
		mk(timeStart.Add(1*time.Minute), false), // 1
		mk(timeStart.Add(2*time.Minute), false), // 2
		mk(timeStart.Add(2*time.Minute), true),  // 3
		mk(timeStart.Add(2*time.Minute), false), // 4
		mk(timeStart.Add(4*time.Minute), true),  // 5
		mk(timeStart.Add(4*time.Minute), true),  // 6
		mk(timeStart.Add(3*time.Minute), false), // 7
	}
	require.NoError(t, d.PutJobs(ctx, jobs))
	d.Wait()

	// Create the cache.
	wait := make(chan struct{})
	jobCacheI, err := NewJobCache(ctx, d, w, func() {
		wait <- struct{}{}
	})
	require.NoError(t, err)
	<-wait
	c := jobCacheI.(*jobCache) // To access update method.

	// Check that jobs[0] and jobs[1] are in the cache.
	assertJobsCached(t, c, []*types.Job{jobs[0], jobs[1]})

	// Add and update jobs.
	jobs[1].Status = types.JOB_STATUS_SUCCESS
	jobs = append(jobs, mk(timeStart.Add(3*time.Minute), false)) // 8
	require.NoError(t, d.PutJobs(ctx, []*types.Job{jobs[1], jobs[8]}))
	d.Wait()
	<-wait

	// update, expiring jobs[0] and jobs[1].
	require.NoError(t, w.UpdateWithTime(timeStart.Add(time.Minute).Add(period).Add(time.Nanosecond)))
	require.NoError(t, c.Update(ctx))

	// Check that jobs[0] and jobs[1] are no longer in the cache.
	assertJobsNotCached(t, c, jobs[:2])

	// Check that other jobs are still cached correctly.
	assertJobsCached(t, c, jobs[2:])

	// Test entire cache expiration.
	newJobs := []*types.Job{
		mk(timeStart.Add(5*time.Minute), false),
	}
	require.NoError(t, d.PutJobs(ctx, newJobs))
	d.Wait()
	<-wait
	require.NoError(t, w.UpdateWithTime(timeStart.Add(5*time.Minute).Add(period).Add(-time.Nanosecond)))
	require.NoError(t, c.Update(ctx))

	// Check that only new job is in the cache.
	assertJobsNotCached(t, c, jobs)
	assertJobsCached(t, c, newJobs)
}

func TestJobCacheGetMatchingJobsFromDateRange(t *testing.T) {
	unittest.SmallTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := memory.NewInMemoryJobDB()

	// Pre-load a job into the DB.
	startTime := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	j1 := types.MakeTestJob(startTime)
	j2 := types.MakeTestJob(startTime)
	j2.Name = "job2"
	require.NoError(t, d.PutJobs(ctx, []*types.Job{j1, j2}))
	d.Wait()

	// Create the cache. Ensure that the existing job is present.
	w, err := window.New(time.Hour, 0, nil)
	require.NoError(t, err)
	c, err := NewJobCache(ctx, d, w, nil)
	require.NoError(t, err)

	test := func(names []string, start, end time.Time, expect ...*types.Job) {
		expectByName := make(map[string][]*types.Job, len(expect))
		for _, job := range expect {
			expectByName[job.Name] = append(expectByName[job.Name], job)
		}
		jobs, err := c.GetMatchingJobsFromDateRange(names, start, end)
		require.NoError(t, err)
		assertdeep.Equal(t, expectByName, jobs)
	}
	test([]string{j1.Name, j2.Name}, time.Time{}, time.Now().Add(24*time.Hour), j1, j2)
	test([]string{j1.Name, j2.Name}, j1.Created, j1.Created.Add(time.Nanosecond), j1, j2)
	test([]string{j1.Name, j2.Name}, time.Time{}, j1.Created)
	test([]string{j1.Name, j2.Name}, j1.Created.Add(time.Nanosecond), time.Now().Add(24*time.Hour))
	test([]string{j1.Name}, j1.Created, j1.Created.Add(time.Nanosecond), j1)
}
