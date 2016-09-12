package db

import (
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func testGetTasksForCommits(t *testing.T, c TaskCache, b *Task) {
	for _, commit := range b.Commits {
		found, err := c.GetTaskForCommit(DEFAULT_TEST_REPO, commit, b.Name)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, b, found)

		tasks, err := c.GetTasksForCommits(DEFAULT_TEST_REPO, []string{commit})
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, map[string]map[string]*Task{
			commit: map[string]*Task{
				b.Name: b,
			},
		}, tasks)
	}
}

func TestTaskCache(t *testing.T) {
	db := NewInMemoryTaskDB()
	defer testutils.AssertCloses(t, db)

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

func TestTaskCacheMultiRepo(t *testing.T) {
	db := NewInMemoryTaskDB()
	defer testutils.AssertCloses(t, db)

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
	db := NewInMemoryTaskDB()
	defer testutils.AssertCloses(t, db)

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
	db := NewInMemoryTaskDB()
	defer testutils.AssertCloses(t, db)

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

func TestJobCache(t *testing.T) {
	db := NewInMemoryJobDB()
	defer testutils.AssertCloses(t, db)

	// Pre-load a job into the DB.
	startTime := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	j1 := makeJob(startTime)
	assert.NoError(t, db.PutJob(j1))

	// Create the cache. Ensure that the existing job is present.
	c, err := NewJobCache(db, time.Hour)
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
	db := NewInMemoryJobDB()
	defer testutils.AssertCloses(t, db)

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
	cache, err := NewJobCache(db, time.Hour)
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
	db := NewInMemoryJobDB()
	defer testutils.AssertCloses(t, db)

	// Pre-load a job into the DB.
	startTime := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	j1 := makeJob(startTime)
	assert.NoError(t, db.PutJob(j1))

	// Create the cache. Ensure that the existing job is present.
	c, err := NewJobCache(db, time.Hour)
	assert.NoError(t, err)
	testGetUnfinished(t, []*Job{j1}, c)

	// Pretend the DB connection is lost.
	db.StopTrackingModifiedJobs(c.(*jobCache).queryId)

	// Make an update.
	j2 := makeJob(startTime.Add(time.Minute))
	j1.Dependencies = []string{"someTask"}
	assert.NoError(t, db.PutJobs([]*Job{j2, j1}))

	// Ensure cache gets reset.
	assert.NoError(t, c.Update())
	testGetUnfinished(t, []*Job{j1, j2}, c)
}

func TestJobCacheUnfinished(t *testing.T) {
	db := NewInMemoryJobDB()
	defer testutils.AssertCloses(t, db)

	// Insert a job.
	startTime := time.Now().Add(-30 * time.Minute)
	j1 := makeJob(startTime)
	assert.False(t, j1.Done())
	assert.NoError(t, db.PutJob(j1))

	// Create the cache. Ensure that the existing job is present.
	c, err := NewJobCache(db, time.Hour)
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
	j3.Dependencies = []string{"a", "b", "c"}
	assert.False(t, j3.Done())
	assert.NoError(t, db.PutJob(j3))
	assert.NoError(t, c.Update())
	testGetUnfinished(t, []*Job{j3}, c)
}
