package db

import (
	"fmt"
	"io/ioutil"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
)

func testGetTasksForCommits(t *testing.T, c TaskCache, b *Task) {
	for _, commit := range b.Commits {
		found, err := c.GetTaskForCommit(DEFAULT_TEST_REPO, commit, b.Name)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, b, found)

		tasks, err := c.GetTasksForCommits(DEFAULT_TEST_REPO, []string{commit})
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, b, tasks[commit][b.Name])
	}
}

func TestTaskCache(t *testing.T) {
	testutils.SmallTest(t)
	db := NewInMemoryTaskDB()

	// Pre-load a task into the DB.
	startTime := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	t1 := makeTask(startTime, []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutTask(t1))

	// Create the cache. Ensure that the existing task is present.
	c, err := NewTaskCache(db, time.Hour)
	assert.NoError(t, err)
	testGetTasksForCommits(t, c, t1)

	// Bisect the first task.
	t2 := makeTask(startTime.Add(time.Minute), []string{"c", "d"})
	t1.Commits = []string{"a", "b"}
	assert.NoError(t, db.PutTasks([]*Task{t2, t1}))
	assert.NoError(t, c.Update())

	// Ensure that t2 (and not t1) shows up for commits "c" and "d".
	testGetTasksForCommits(t, c, t1)
	testGetTasksForCommits(t, c, t2)

	// Insert a task on a second bot.
	t3 := makeTask(startTime.Add(2*time.Minute), []string{"a", "b"})
	t3.Name = "Another-Task"
	assert.NoError(t, db.PutTask(t3))
	assert.NoError(t, c.Update())
	tasks, err := c.GetTasksForCommits(DEFAULT_TEST_REPO, []string{"b"})
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, map[string]map[string]*Task{
		"b": map[string]*Task{
			t1.Name: t1,
			t3.Name: t3,
		},
	}, tasks)
}

func TestTaskCacheKnownTaskName(t *testing.T) {
	testutils.SmallTest(t)
	db := NewInMemoryTaskDB()
	c, err := NewTaskCache(db, time.Hour)
	assert.NoError(t, err)

	// Try jobs don't count toward KnownTaskName.
	startTime := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	t1 := makeTask(startTime, []string{"a", "b", "c", "d"})
	t1.Server = "fake-server"
	t1.Issue = "fake-issue"
	t1.Patchset = "fake-patchset"
	assert.NoError(t, db.PutTask(t1))
	assert.NoError(t, c.Update())
	assert.False(t, c.KnownTaskName(t1.Repo, t1.Name))

	// Forced jobs don't count toward KnownTaskName.
	t2 := makeTask(startTime, []string{"a", "b", "c", "d"})
	t2.ForcedJobId = "job-id"
	assert.NoError(t, db.PutTask(t2))
	assert.NoError(t, c.Update())
	assert.False(t, c.KnownTaskName(t2.Repo, t2.Name))

	// Normal task.
	t3 := makeTask(startTime, []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutTask(t3))
	assert.NoError(t, c.Update())
	assert.True(t, c.KnownTaskName(t3.Repo, t3.Name))
}

func TestTaskCacheGetTasksFromDateRange(t *testing.T) {
	testutils.SmallTest(t)
	db := NewInMemoryTaskDB()

	// Pre-load a task into the DB.
	timeStart := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	t1 := makeTask(timeStart.Add(time.Nanosecond), []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutTask(t1))

	// Create the cache.
	c, err := NewTaskCache(db, time.Hour)
	assert.NoError(t, err)

	// Insert two more tasks. Ensure at least 1 nanosecond between task Created
	// times so that t1After != t2Before and t2After != t3Before.
	t2 := makeTask(timeStart.Add(2*time.Nanosecond), []string{"e", "f"})
	t3 := makeTask(timeStart.Add(3*time.Nanosecond), []string{"g", "h"})
	assert.NoError(t, db.PutTasks([]*Task{t2, t3}))
	assert.NoError(t, c.Update())

	// Ensure that all tasks show up in the correct time ranges, in sorted order.
	t1Before := t1.Created
	t1After := t1Before.Add(1 * time.Nanosecond)

	t2Before := t2.Created
	t2After := t2Before.Add(1 * time.Nanosecond)

	t3Before := t3.Created
	t3After := t3Before.Add(1 * time.Nanosecond)

	timeEnd := timeStart.Add(4 * time.Nanosecond)

	tasks, err := c.GetTasksFromDateRange(timeStart, t1Before)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))

	tasks, err = c.GetTasksFromDateRange(timeStart, t1After)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1}, tasks)

	tasks, err = c.GetTasksFromDateRange(timeStart, t2Before)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1}, tasks)

	tasks, err = c.GetTasksFromDateRange(timeStart, t2After)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1, t2}, tasks)

	tasks, err = c.GetTasksFromDateRange(timeStart, t3Before)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1, t2}, tasks)

	tasks, err = c.GetTasksFromDateRange(timeStart, t3After)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1, t2, t3}, tasks)

	tasks, err = c.GetTasksFromDateRange(timeStart, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1, t2, t3}, tasks)

	tasks, err = c.GetTasksFromDateRange(t1Before, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1, t2, t3}, tasks)

	tasks, err = c.GetTasksFromDateRange(t1After, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t2, t3}, tasks)

	tasks, err = c.GetTasksFromDateRange(t2Before, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t2, t3}, tasks)

	tasks, err = c.GetTasksFromDateRange(t2After, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t3}, tasks)

	tasks, err = c.GetTasksFromDateRange(t3Before, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t3}, tasks)

	tasks, err = c.GetTasksFromDateRange(t3After, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{}, tasks)
}

func TestTaskCacheMultiRepo(t *testing.T) {
	testutils.SmallTest(t)
	db := NewInMemoryTaskDB()

	// Insert several tasks with different repos.
	startTime := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	t1 := makeTask(startTime, []string{"a", "b"})  // Default Repo.
	t2 := makeTask(startTime, []string{"a", "b"})
	t2.Repo = "thats-what-you.git"
	t3 := makeTask(startTime, []string{"b", "c"})
	t3.Repo = "never-for.git"
	assert.NoError(t, db.PutTasks([]*Task{t1, t2, t3}))

	// Create the cache.
	c, err := NewTaskCache(db, time.Hour)
	assert.NoError(t, err)

	// Check that there's no conflict among the tasks in different repos.
	{
		tasks, err := c.GetTasksForCommits(t1.Repo, []string{"a", "b", "c"})
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, map[string]map[string]*Task{
			"a": map[string]*Task{
				t1.Name: t1,
			},
			"b": map[string]*Task{
				t1.Name: t1,
			},
			"c": map[string]*Task{},
		}, tasks)
	}

	{
		tasks, err := c.GetTasksForCommits(t2.Repo, []string{"a", "b", "c"})
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, map[string]map[string]*Task{
			"a": map[string]*Task{
				t1.Name: t2,
			},
			"b": map[string]*Task{
				t1.Name: t2,
			},
			"c": map[string]*Task{},
		}, tasks)
	}

	{
		tasks, err := c.GetTasksForCommits(t3.Repo, []string{"a", "b", "c"})
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, map[string]map[string]*Task{
			"a": map[string]*Task{},
			"b": map[string]*Task{
				t1.Name: t3,
			},
			"c": map[string]*Task{
				t1.Name: t3,
			},
		}, tasks)
	}
}

func TestTaskCacheReset(t *testing.T) {
	testutils.SmallTest(t)
	db := NewInMemoryTaskDB()

	// Pre-load a task into the DB.
	startTime := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	t1 := makeTask(startTime, []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutTask(t1))

	// Create the cache. Ensure that the existing task is present.
	c, err := NewTaskCache(db, time.Hour)
	assert.NoError(t, err)
	testGetTasksForCommits(t, c, t1)

	// Pretend the DB connection is lost.
	db.StopTrackingModifiedTasks(c.(*taskCache).queryId)

	// Make an update.
	t2 := makeTask(startTime.Add(time.Minute), []string{"c", "d"})
	t1.Commits = []string{"a", "b"}
	assert.NoError(t, db.PutTasks([]*Task{t2, t1}))

	// Ensure cache gets reset.
	assert.NoError(t, c.Update())
	testGetTasksForCommits(t, c, t1)
	testGetTasksForCommits(t, c, t2)
}

func TestTaskCacheUnfinished(t *testing.T) {
	testutils.SmallTest(t)
	db := NewInMemoryTaskDB()

	// Insert a task.
	startTime := time.Now().Add(-30 * time.Minute)
	t1 := makeTask(startTime, []string{"a"})
	assert.False(t, t1.Done())
	assert.NoError(t, db.PutTask(t1))

	// Create the cache. Ensure that the existing task is present.
	c, err := NewTaskCache(db, time.Hour)
	assert.NoError(t, err)
	tasks, err := c.UnfinishedTasks()
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t1}, tasks)

	// Finish the task. Insert it, ensure that it's not unfinished.
	t1.Status = TASK_STATUS_SUCCESS
	assert.True(t, t1.Done())
	assert.NoError(t, db.PutTask(t1))
	assert.NoError(t, c.Update())
	tasks, err = c.UnfinishedTasks()
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{}, tasks)

	// Already-finished task.
	t2 := makeTask(time.Now(), []string{"a"})
	t2.Status = TASK_STATUS_MISHAP
	assert.True(t, t2.Done())
	assert.NoError(t, db.PutTask(t2))
	assert.NoError(t, c.Update())
	tasks, err = c.UnfinishedTasks()
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{}, tasks)

	// An unfinished task, created after the cache was created.
	t3 := makeTask(time.Now(), []string{"b"})
	assert.False(t, t3.Done())
	assert.NoError(t, db.PutTask(t3))
	assert.NoError(t, c.Update())
	tasks, err = c.UnfinishedTasks()
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t3}, tasks)

	// Update the task.
	t3.Commits = []string{"c", "d", "f"}
	assert.False(t, t3.Done())
	assert.NoError(t, db.PutTask(t3))
	assert.NoError(t, c.Update())
	tasks, err = c.UnfinishedTasks()
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Task{t3}, tasks)
}

// assertTaskInSlice fails the test if task is not deep-equal to an element of
// slice.
func assertTaskInSlice(t *testing.T, task *Task, slice []*Task) {
	for _, other := range slice {
		if task.Id == other.Id {
			testutils.AssertDeepEqual(t, task, other)
			return
		}
	}
	t.Fatalf("Did not find task %v in %s.", task, slice)
}

// assertTasksNotCached checks that none of tasks are retrievable from c.
func assertTasksNotCached(t *testing.T, c TaskCache, tasks []*Task) {
	byTimeTasks, err := c.GetTasksFromDateRange(time.Date(1900, time.January, 1, 0, 0, 0, 0, time.UTC), time.Date(9999, time.January, 1, 0, 0, 0, 0, time.UTC))
	assert.NoError(t, err)

	unfinishedTasks, err := c.UnfinishedTasks()
	assert.NoError(t, err)
	for _, task := range tasks {
		_, err := c.GetTask(task.Id)
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
		for _, commit := range task.Commits {
			found, err := c.GetTaskForCommit(DEFAULT_TEST_REPO, commit, task.Name)
			assert.NoError(t, err)
			assert.Nil(t, found)
			tasks, err := c.GetTasksForCommits(DEFAULT_TEST_REPO, []string{commit})
			assert.NoError(t, err)
			_, ok := tasks[commit][task.Name]
			assert.False(t, ok)
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
	testutils.SmallTest(t)
	db := NewInMemoryTaskDB()

	period := 10 * time.Minute
	timeStart := time.Now().Add(-period)

	// Make a bunch of tasks with various timestamps.
	mk := func(mins int, name string, blame []string) *Task {
		t := makeTask(timeStart.Add(time.Duration(mins)*time.Minute), blame)
		t.Name = name
		return t
	}

	tasks := []*Task{
		mk(1, "Old", []string{"a"}),                   // 0
		mk(1, "Build1", []string{"a", "b"}),           // 1
		mk(2, "Build1", []string{"c"}),                // 2
		mk(2, "Build2", []string{"c"}),                // 3
		mk(3, "Build1", []string{"d"}),                // 4
		mk(4, "Build2", []string{"d"}),                // 5
		mk(5, "Build3", []string{"a", "b", "c", "d"}), // 6
	}
	assert.NoError(t, db.PutTasks(tasks))

	// Create the cache.
	taskCacheI, err := NewTaskCache(db, period)
	assert.NoError(t, err)
	c := taskCacheI.(*taskCache) // To access update method.

	{
		// Check that tasks[0] and tasks[1] are in the cache.
		firstTasks, err := c.GetTasksFromDateRange(timeStart, timeStart.Add(2*time.Minute))
		assert.NoError(t, err)
		assert.Equal(t, 2, len(firstTasks))
		unfinishedTasks, err := c.UnfinishedTasks()
		assert.NoError(t, err)
		for _, task := range []*Task{tasks[0], tasks[1]} {
			cachedTask, err := c.GetTask(task.Id)
			assert.NoError(t, err)
			testutils.AssertDeepEqual(t, task, cachedTask)
			testGetTasksForCommits(t, c, task)
			assertTaskInSlice(t, task, firstTasks)
			assertTaskInSlice(t, task, unfinishedTasks)
			assert.True(t, c.KnownTaskName(task.Repo, task.Name))
		}
	}

	// Add and update tasks.
	tasks[1].Status = TASK_STATUS_SUCCESS
	tasks[6].Commits = []string{"c", "d"}
	tasks = append(tasks,
		mk(7, "Build3", []string{"a", "b"}),           // 7
		mk(4, "Build4", []string{"a", "b", "c", "d"})) // 8
	// Out of order to test TaskDB.GetModifiedTasks.
	assert.NoError(t, db.PutTasks([]*Task{tasks[6], tasks[8], tasks[1], tasks[7]}))

	// update, expiring tasks[0] and tasks[1].
	assert.NoError(t, c.update(tasks[0].Created.Add(period).Add(time.Nanosecond)))

	{
		// Check that tasks[0] and tasks[1] are no longer in the cache.
		firstTasks, err := c.GetTasksFromDateRange(timeStart, timeStart.Add(2*time.Minute))
		assert.NoError(t, err)
		assert.Equal(t, 0, len(firstTasks))

		assertTasksNotCached(t, c, []*Task{tasks[0], tasks[1]})

		// tasks[0].Name is no longer known, but there are later Tasks for tasks[1].Name.
		assert.False(t, c.KnownTaskName(tasks[0].Repo, tasks[0].Name))
		assert.True(t, c.KnownTaskName(tasks[1].Repo, tasks[1].Name))
	}

	{
		// Check that other tasks are still cached correctly.
		allCachedTasks, err := c.GetTasksFromDateRange(timeStart, timeStart.Add(20*time.Minute))
		assert.NoError(t, err)
		orderedTasks := []*Task{tasks[2], tasks[3], tasks[4], tasks[5], tasks[8], tasks[6], tasks[7]}
		// Tasks with same timestamp can be in either order.
		if allCachedTasks[0].Id != orderedTasks[0].Id {
			allCachedTasks[0], allCachedTasks[1] = allCachedTasks[1], allCachedTasks[0]
		}
		if allCachedTasks[3].Id != orderedTasks[3].Id {
			allCachedTasks[3], allCachedTasks[4] = allCachedTasks[4], allCachedTasks[3]
		}
		testutils.AssertDeepEqual(t, orderedTasks, allCachedTasks)

		unfinishedTasks, err := c.UnfinishedTasks()
		assert.NoError(t, err)
		for _, task := range orderedTasks {
			cachedTask, err := c.GetTask(task.Id)
			assert.NoError(t, err)
			testutils.AssertDeepEqual(t, task, cachedTask)
			testGetTasksForCommits(t, c, task)
			assertTaskInSlice(t, task, unfinishedTasks)
			assert.True(t, c.KnownTaskName(task.Repo, task.Name))
		}
	}

	// Test entire cache expiration.
	newTasks := []*Task{
		mk(11, "Build3", []string{"e"}),
	}
	assert.NoError(t, db.PutTasks(newTasks))
	assert.NoError(t, c.update(newTasks[0].Created.Add(period)))

	{
		// Check that only new task is in the cache.
		assertTasksNotCached(t, c, tasks)

		for _, task := range newTasks {
			cachedTask, err := c.GetTask(task.Id)
			assert.NoError(t, err)
			testutils.AssertDeepEqual(t, task, cachedTask)
			testGetTasksForCommits(t, c, task)
		}

		allCachedTasks, err := c.GetTasksFromDateRange(timeStart, timeStart.Add(20*time.Minute))
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, newTasks, allCachedTasks)

		// Only new task is known.
		assert.True(t, c.KnownTaskName(newTasks[0].Repo, newTasks[0].Name))
		for _, name := range []string{"Old", "Build1", "Build2"} {
			assert.False(t, c.KnownTaskName(DEFAULT_TEST_REPO, name))
		}

		unfinishedTasks, err := c.UnfinishedTasks()
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, newTasks, unfinishedTasks)
	}
}

func TestJobCache(t *testing.T) {
	testutils.SmallTest(t)
	db := NewInMemoryJobDB()

	// Pre-load a job into the DB.
	startTime := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	j1 := makeJob(startTime)
	assert.NoError(t, db.PutJob(j1))

	// Create the cache. Ensure that the existing job is present.
	c, err := NewJobCache(db, time.Hour, DummyGetRevisionTimestamp(j1.Created.Add(-1*time.Minute)))
	assert.NoError(t, err)
	test, err := c.GetJob(j1.Id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, j1, test)

	// Create another job. Ensure that it gets picked up.
	j2 := makeJob(startTime.Add(time.Nanosecond))
	assert.NoError(t, db.PutJob(j2))
	test, err = c.GetJob(j2.Id)
	assert.Error(t, err)
	assert.NoError(t, c.Update())
	test, err = c.GetJob(j2.Id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, j2, test)
}

func TestJobCacheTriggeredForCommit(t *testing.T) {
	testutils.SmallTest(t)
	db := NewInMemoryJobDB()

	// Insert several jobs with different repos.
	startTime := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	j1 := makeJob(startTime)                       // Default Repo.
	j1.Revision = "a"
	j2 := makeJob(startTime)
	j2.Repo = "thats-what-you.git"
	j2.Revision = "b"
	j3 := makeJob(startTime)
	j3.Repo = "never-for.git"
	j3.Revision = "c"
	assert.NoError(t, db.PutJobs([]*Job{j1, j2, j3}))

	// Create the cache.
	cache, err := NewJobCache(db, time.Hour, DummyGetRevisionTimestamp(j1.Created.Add(-1*time.Minute)))
	assert.NoError(t, err)
	b, err := cache.ScheduledJobsForCommit(j1.Repo, j1.Revision)
	assert.NoError(t, err)
	assert.True(t, b)
	b, err = cache.ScheduledJobsForCommit(j2.Repo, j2.Revision)
	assert.NoError(t, err)
	assert.True(t, b)
	b, err = cache.ScheduledJobsForCommit(j3.Repo, j3.Revision)
	assert.NoError(t, err)
	assert.True(t, b)
	b, err = cache.ScheduledJobsForCommit(j2.Repo, j3.Revision)
	assert.NoError(t, err)
	assert.False(t, b)
}

func testGetUnfinished(t *testing.T, expect []*Job, cache JobCache) {
	jobs, err := cache.UnfinishedJobs()
	assert.NoError(t, err)
	sort.Sort(JobSlice(jobs))
	sort.Sort(JobSlice(expect))
	testutils.AssertDeepEqual(t, expect, jobs)
}

func TestJobCacheReset(t *testing.T) {
	testutils.SmallTest(t)
	db := NewInMemoryJobDB()

	// Pre-load a job into the DB.
	startTime := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	j1 := makeJob(startTime)
	assert.NoError(t, db.PutJob(j1))

	// Create the cache. Ensure that the existing job is present.
	c, err := NewJobCache(db, time.Hour, DummyGetRevisionTimestamp(j1.Created.Add(-1*time.Minute)))
	assert.NoError(t, err)
	testGetUnfinished(t, []*Job{j1}, c)

	// Pretend the DB connection is lost.
	db.StopTrackingModifiedJobs(c.(*jobCache).queryId)

	// Make an update.
	j2 := makeJob(startTime.Add(time.Minute))
	j1.Dependencies = map[string][]string{"someTask": []string{}}
	assert.NoError(t, db.PutJobs([]*Job{j2, j1}))

	// Ensure cache gets reset.
	assert.NoError(t, c.Update())
	testGetUnfinished(t, []*Job{j1, j2}, c)
}

func TestJobCacheUnfinished(t *testing.T) {
	testutils.SmallTest(t)
	db := NewInMemoryJobDB()

	// Insert a job.
	startTime := time.Now().Add(-30 * time.Minute)
	j1 := makeJob(startTime)
	assert.False(t, j1.Done())
	assert.NoError(t, db.PutJob(j1))

	// Create the cache. Ensure that the existing job is present.
	c, err := NewJobCache(db, time.Hour, DummyGetRevisionTimestamp(j1.Created.Add(-1*time.Minute)))
	assert.NoError(t, err)
	testGetUnfinished(t, []*Job{j1}, c)

	// Finish the job. Insert it, ensure that it's not unfinished.
	j1.Status = JOB_STATUS_SUCCESS
	assert.True(t, j1.Done())
	assert.NoError(t, db.PutJob(j1))
	assert.NoError(t, c.Update())
	testGetUnfinished(t, []*Job{}, c)

	// Already-finished job.
	j2 := makeJob(time.Now())
	j2.Status = JOB_STATUS_MISHAP
	assert.True(t, j2.Done())
	assert.NoError(t, db.PutJob(j2))
	assert.NoError(t, c.Update())
	testGetUnfinished(t, []*Job{}, c)

	// An unfinished job, created after the cache was created.
	j3 := makeJob(time.Now())
	assert.False(t, j3.Done())
	assert.NoError(t, db.PutJob(j3))
	assert.NoError(t, c.Update())
	testGetUnfinished(t, []*Job{j3}, c)

	// Update the job.
	j3.Dependencies = map[string][]string{"a": []string{}, "b": []string{}, "c": []string{}}
	assert.False(t, j3.Done())
	assert.NoError(t, db.PutJob(j3))
	assert.NoError(t, c.Update())
	testGetUnfinished(t, []*Job{j3}, c)
}

// assertJobInSlice fails the test if job is not deep-equal to an element of
// slice.
func assertJobInSlice(t *testing.T, job *Job, slice []*Job) {
	for _, other := range slice {
		if job.Id == other.Id {
			testutils.AssertDeepEqual(t, job, other)
			return
		}
	}
	t.Fatalf("Did not find job %v in %v.", job, slice)
}

// assertJobsCached checks that all of jobs are retrievable from c.
func assertJobsCached(t *testing.T, c JobCache, jobs []*Job) {
	unfinishedJobs, err := c.UnfinishedJobs()
	assert.NoError(t, err)
	for _, job := range jobs {
		cachedJob, err := c.GetJob(job.Id)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, job, cachedJob)

		if !job.Done() {
			assertJobInSlice(t, job, unfinishedJobs)
		}

		if !job.IsForce {
			val, err := c.ScheduledJobsForCommit(job.Repo, job.Revision)
			assert.NoError(t, err)
			assert.True(t, val)
		}
	}
}

// assertJobsNotCached checks that none of jobs are retrievable from c.
func assertJobsNotCached(t *testing.T, c JobCache, jobs []*Job) {
	unfinishedJobs, err := c.UnfinishedJobs()
	assert.NoError(t, err)
	for _, job := range jobs {
		_, err := c.GetJob(job.Id)
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))

		for _, other := range unfinishedJobs {
			if job.Id == other.Id {
				t.Errorf("Found unexpected job %v in UnfinishedJobs.", job)
			}
		}

		val, err := c.ScheduledJobsForCommit(job.Repo, job.Revision)
		assert.NoError(t, err)
		assert.False(t, val)
	}
}

func TestJobCacheExpiration(t *testing.T) {
	testutils.SmallTest(t)
	db := NewInMemoryJobDB()

	period := 10 * time.Minute
	timeStart := time.Now().Add(-period)

	getRevisionTimestamp := func(repo, rev string) (time.Time, error) {
		assert.Equal(t, DEFAULT_TEST_REPO, repo)
		switch rev {
		case "a":
			return timeStart.Add(1 * time.Minute), nil
		case "b":
			return timeStart.Add(2 * time.Minute), nil
		case "c":
			return timeStart.Add(3 * time.Minute), nil
		case "d":
			return timeStart.Add(4 * time.Minute), nil
		case "e":
			return timeStart.Add(5 * time.Minute), nil
		default:
			assert.FailNow(t, "Unknown revision %q", rev)
			return time.Time{}, fmt.Errorf("Can't get here.")
		}
	}

	// Make a bunch of jobs with various revisions.
	make := func(rev string, isForce bool) *Job {
		job := makeJob(timeStart.Add(7 * time.Minute))
		job.Revision = rev
		job.IsForce = isForce
		return job
	}

	jobs := []*Job{
		make("a", false), // 0
		make("a", false), // 1
		make("b", false), // 2
		make("b", true),  // 3
		make("b", false), // 4
		make("d", true),  // 5
		make("d", true),  // 6
		make("c", false), // 7
	}
	assert.NoError(t, db.PutJobs(jobs))

	// Create the cache.
	jobCacheI, err := NewJobCache(db, period, getRevisionTimestamp)
	assert.NoError(t, err)
	c := jobCacheI.(*jobCache) // To access update method.

	// Check that jobs[0] and jobs[1] are in the cache.
	assertJobsCached(t, c, []*Job{jobs[0], jobs[1]})

	// Add and update jobs.
	jobs[1].Status = JOB_STATUS_SUCCESS
	jobs = append(jobs, make("c", false)) // 8
	assert.NoError(t, db.PutJobs([]*Job{jobs[1], jobs[8]}))

	// update, expiring jobs[0] and jobs[1].
	assert.NoError(t, c.update(timeStart.Add(time.Minute).Add(period).Add(time.Nanosecond)))

	// Check that jobs[0] and jobs[1] are no longer in the cache.
	assertJobsNotCached(t, c, jobs[:2])

	// Check that other jobs are still cached correctly.
	assertJobsCached(t, c, jobs[2:])

	// Test entire cache expiration.
	newJobs := []*Job{
		make("e", false),
	}
	assert.NoError(t, db.PutJobs(newJobs))
	assert.NoError(t, c.update(timeStart.Add(5*time.Minute).Add(period).Add(-time.Nanosecond)))

	// Check that only new job is in the cache.
	assertJobsNotCached(t, c, jobs)
	assertJobsCached(t, c, newJobs)
}

func TestJobCacheGetRevisionTimestampError(t *testing.T) {
	testutils.SmallTest(t)
	db := NewInMemoryJobDB()

	period := 10 * time.Minute
	timeStart := time.Now().Add(-period)

	enableTransientError := true

	getRevisionTimestamp := func(repo, rev string) (time.Time, error) {
		if enableTransientError {
			return time.Time{}, fmt.Errorf("Transient error")
		} else {
			if rev == "a" {
				return timeStart.Add(1 * time.Minute), nil
			} else {
				return timeStart.Add(4 * time.Minute), nil
			}
		}
	}

	make := func(created time.Time, rev string) *Job {
		job := makeJob(created)
		job.Revision = rev
		return job
	}

	// Make jobs with different Created timestamps.
	jobs := []*Job{
		make(timeStart.Add(2*time.Minute), "a"), // 0
		make(timeStart.Add(3*time.Minute), "a"), // 1
		make(timeStart.Add(2*time.Minute), "b"), // 2
		make(timeStart.Add(3*time.Minute), "b"), // 3
	}
	assert.NoError(t, db.PutJobs(jobs))

	// Create the cache.
	jobCacheI, err := NewJobCache(db, period, getRevisionTimestamp)
	assert.NoError(t, err)
	c := jobCacheI.(*jobCache) // To access update method.

	// Check that all jobs are in the cache.
	assertJobsCached(t, c, jobs)

	// update and expire jobs before timeStart.Add(1 * time.Minute); since
	// getRevisionTimestamp returns an error, this shouldn't expire any jobs.
	assert.NoError(t, c.update(timeStart.Add(1*time.Minute).Add(period).Add(time.Nanosecond)))

	// Check that all jobs are in the cache.
	assertJobsCached(t, c, jobs)

	// Transient error is resolved; jobs for revision "a" should be expired.
	enableTransientError = false
	assert.NoError(t, c.update(timeStart.Add(1*time.Minute).Add(period).Add(time.Nanosecond)))

	assertJobsNotCached(t, c, jobs[:2])
	assertJobsCached(t, c, jobs[2:])

	// Created time is ignored if getRevisionTimestamp does not error.
	assert.NoError(t, c.update(timeStart.Add(2*time.Minute).Add(period).Add(time.Nanosecond)))

	assertJobsNotCached(t, c, jobs[:2])
	assertJobsCached(t, c, jobs[2:])

	// If error persists, jobs are expired by Created time.
	enableTransientError = true
	assert.NoError(t, c.update(timeStart.Add(2*time.Minute).Add(period).Add(time.Nanosecond)))

	assertJobsNotCached(t, c, jobs[:2])
	// jobs[2] should also not be cached, but it has the same revision as jobs[3],
	// so ScheduledJobsForCommit returns true.
	{
		_, err := c.GetJob(jobs[2].Id)
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))

		unfinishedJobs, err := c.UnfinishedJobs()
		assert.NoError(t, err)
		for _, other := range unfinishedJobs {
			if jobs[2].Id == other.Id {
				t.Errorf("Found unexpected job %v in UnfinishedJobs.", other)
			}
		}
	}
	assertJobsCached(t, c, jobs[3:])
}

func TestGitRepoGetRevisionTimestamp(t *testing.T) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)
	g := git_testutils.GitInit(t)
	defer g.Cleanup()

	git_testutils.GitSetup(g)
	now := time.Now()
	g.AddGen("a.txt")
	g.CommitMsgAt("Extra commit 1", now.Add(3*time.Second))
	g.AddGen("a.txt")
	g.CommitMsgAt("Extra commit 2", now.Add(17*time.Hour))

	workdir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, workdir)
	repo, err := repograph.NewGraph(g.Dir(), workdir)
	assert.NoError(t, err)

	grt := GitRepoGetRevisionTimestamp(map[string]*repograph.Graph{
		"a.git": repo,
	})

	var firstCommit *repograph.Commit
	err = repo.RecurseAllBranches(func(c *repograph.Commit) (bool, error) {
		ts, err := grt("a.git", c.Hash)
		assert.NoError(t, err)
		assert.True(t, c.Timestamp.Equal(ts))
		firstCommit = c
		return true, nil
	})
	assert.NoError(t, err)

	_, err = grt("invalid.git", firstCommit.Hash)
	assert.EqualError(t, err, "Unknown repo invalid.git")

	_, err = grt("a.git", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	assert.EqualError(t, err, "Unknown commit a.git@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
}
