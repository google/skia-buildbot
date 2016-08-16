package db

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func testGetTasksForCommits(t *testing.T, c *TaskCache, b *Task) {
	for _, commit := range b.Commits {
		found, err := c.GetTaskForCommit(b.Name, commit)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, b, found)

		tasks, err := c.GetTasksForCommits([]string{commit})
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, map[string]map[string]*Task{
			commit: map[string]*Task{
				b.Name: b,
			},
		}, tasks)
	}
}

func TestDBCache(t *testing.T) {
	db := NewInMemoryDB()
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
	tasks, err := c.GetTasksForCommits([]string{"b"})
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, map[string]map[string]*Task{
		"b": map[string]*Task{
			t1.Name: t1,
			t3.Name: t3,
		},
	}, tasks)
}
