package db

import (
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

func TestDBCache(t *testing.T) {
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

func TestDBCacheMultiRepo(t *testing.T) {
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

func TestDBCacheReset(t *testing.T) {
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

func TestCacheUnfinished(t *testing.T) {
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
