package firestore

import (
	"context"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
)

func TestModifiedTasks(t *testing.T) {
	db, cleanup := setup(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	id, err := db.StartTrackingModifiedTasks()
	assert.NoError(t, err)

	test := func(expect []*types.Task) {
		expectMap := make(map[string]*types.Task, len(expect))
		for _, e := range expect {
			expectMap[e.Id] = e
		}
		assert.NoError(t, testutils.EventuallyConsistent(10*time.Second, func() error {
			actual, err := db.GetModifiedTasks(id)
			assert.NoError(t, err)
			actualMap := make(map[string]*types.Task, len(expectMap))
			for _, a := range actual {
				// Ignore tasks not in the expected list.
				if _, ok := expectMap[a.Id]; !ok {
					continue
				}
				actualMap[a.Id] = a
			}
			if !deepequal.DeepEqual(expectMap, actualMap) {
				time.Sleep(100 * time.Millisecond)
				return testutils.TryAgainErr
			}
			return nil
		}))
	}

	// Add one task, ensure that it shows up.
	expect := []*types.Task{
		{
			Id:      "0",
			Created: time.Now(),
		},
	}
	assert.NoError(t, db.PutTasks(expect))
	test(expect)

	// Add two tasks.
	expect = []*types.Task{
		{
			Id:      "1",
			Created: time.Now(),
		},
		{
			Id:      "2",
			Created: time.Now(),
		},
	}
	assert.NoError(t, db.PutTasks(expect))
	test(expect)

	// Modify a task.
	expect[0].Name = "my-task"
	assert.NoError(t, db.PutTasks(expect[:1]))
	test(expect[:1])

	// ModifiedTasksCh removes deleted Tasks from the slice before passing
	// it through the channel. ModifiedTasks makes that a no-op, so
	// a query snapshot. Our code just removes the deleted task from the
	// results, so this should be an empty slice.
	_, err = db.(*firestoreDB).tasks().Doc(expect[1].Id).Delete(ctx)
	assert.NoError(t, err)
	test([]*types.Task{})
}
