package firestore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/types"
)

/*
	Tests for WatchModified*. The ModifiedData implementation is tested in
	the shared tests from the db package.
*/

func setupWatch(t *testing.T) (*firestoreDB, func()) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	d, err := NewDB(context.Background(), c, nil)
	assert.NoError(t, err)
	return d.(*firestoreDB), cleanup
}

func TestWatchModifiedTasks(t *testing.T) {
	d, cleanup := setupWatch(t)
	defer cleanup()

	type taskData struct {
		task    *types.Task
		deleted bool
	}
	ch := make(chan taskData)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		err := WatchModifiedTasks(ctx, d, func(task *types.Task, deleted bool) error {
			ch <- taskData{
				task:    task,
				deleted: deleted,
			}
			return nil
		})
		assert.Equal(t, context.Canceled, err)
	}()

	test := func(expect []taskData) {
		// The callback might be called in any order. We may receive
		// multiple callbacks for the same task, if it was modified
		// multiple times. Ensure that every Task we get from
		// WatchModifiedTasks is in the expected slice, and ensure that
		// we got a Task for every expectation.
		found := make([]bool, len(expect))
		for range expect {
			got := <-ch
			foundExpectation := false
			for idx, e := range expect {
				if deepequal.DeepEqual(e, got) {
					foundExpectation = true
					found[idx] = true
					break
				}
			}
			assert.True(t, foundExpectation)
		}
		for _, f := range found {
			assert.True(t, f)
		}
	}
	testOne := func(expect *types.Task, deleted bool) {
		test([]taskData{
			taskData{
				task:    expect,
				deleted: deleted,
			},
		})
	}

	t0 := types.MakeTestTask(time.Now(), []string{"a"})
	assert.NoError(t, d.PutTask(t0))
	testOne(t0, false)

	t0.Status = types.TASK_STATUS_SUCCESS
	assert.NoError(t, d.PutTask(t0))
	testOne(t0, false)

	// We don't have an API for deleting tasks, but pretend we do.
	_, err := d.tasks().Doc(t0.Id).Delete(ctx)
	assert.NoError(t, err)
	testOne(t0, true)

	// Re-insert the task, along with another.
	t0.Id = ""
	t0.DbModified = time.Time{}
	t1 := types.MakeTestTask(time.Now(), []string{"b"})
	assert.NoError(t, d.PutTasks([]*types.Task{t0, t1}))
	test([]taskData{
		taskData{
			task:    t0,
			deleted: false,
		},
		taskData{
			task:    t1,
			deleted: false,
		},
	})

	// Modify the same task multiple times. Retain the originals so that
	// we can use deepequal.
	t1.Commits = []string{"c"}
	assert.NoError(t, d.PutTask(t1))
	t1Cpy1 := t1.Copy()
	t1Cpy1.SwarmingTaskId = "abc123"
	assert.NoError(t, d.PutTask(t1Cpy1))
	t1Cpy2 := t1Cpy1.Copy()
	t1Cpy2.Status = types.TASK_STATUS_FAILURE
	assert.NoError(t, d.PutTask(t1Cpy2))
	_, err = d.tasks().Doc(t1.Id).Delete(ctx)
	assert.NoError(t, err)
	test([]taskData{
		taskData{
			task:    t1,
			deleted: false,
		},
		taskData{
			task:    t1Cpy1,
			deleted: false,
		},
		taskData{
			task:    t1Cpy2,
			deleted: false,
		},
		taskData{
			task:    t1Cpy2,
			deleted: true,
		},
	})
}
