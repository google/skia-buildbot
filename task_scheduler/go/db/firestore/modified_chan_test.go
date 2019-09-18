package firestore

import (
	"context"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/task_scheduler/go/types"
)

var deleted = true

func TestModifiedTasksCh(t *testing.T) {
	db, cleanup := setup(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	taskCh := ModifiedTasksCh(ctx, db)

	// Initial snapshot contains all results, ie. empty right now.
	<-taskCh

	// Add one task, ensure that it shows up.
	expect := []*types.Task{
		{
			Id:      "0",
			Created: time.Now(),
		},
	}
	assert.NoError(t, db.PutTasks(expect))
	got := <-taskCh
	deepequal.AssertDeepEqual(t, expect, got)

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
	got = <-taskCh
	deepequal.AssertDeepEqual(t, expect, got)

	// Modify a task.
	expect[0].Name = "my-task"
	assert.NoError(t, db.PutTasks(expect[:1]))
	got = <-taskCh
	deepequal.AssertDeepEqual(t, expect[:1], got)

	// Delete a task. This isn't actually supported, but we should still see
	// a query snapshot. Our code just removes the deleted task from the
	// results, so this should be an empty slice.
	_, err := db.(*firestoreDB).tasks().Doc(expect[1].Id).Delete(ctx)
	assert.NoError(t, err)
	got = <-taskCh
	deepequal.AssertDeepEqual(t, []*types.Task{}, got)

	// Assert that the channel gets closed when we cancel the context.
	cancel()
	_, stillOpen := <-taskCh
	assert.False(t, stillOpen)
}

func TestModifiedJobsCh(t *testing.T) {
	db, cleanup := setup(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobCh := ModifiedJobsCh(ctx, db)

	// Initial snapshot contains all results, ie. empty right now.
	<-jobCh

	// Add one job, ensure that it shows up.
	expect := []*types.Job{
		{
			Id:      "0",
			Created: time.Now(),
		},
	}
	assert.NoError(t, db.PutJobs(expect))
	got := <-jobCh
	deepequal.AssertDeepEqual(t, expect, got)

	// Add two jobs.
	expect = []*types.Job{
		{
			Id:      "1",
			Created: time.Now(),
		},
		{
			Id:      "2",
			Created: time.Now(),
		},
	}
	assert.NoError(t, db.PutJobs(expect))
	got = <-jobCh
	deepequal.AssertDeepEqual(t, expect, got)

	// Modify a job.
	expect[0].Name = "my-job"
	assert.NoError(t, db.PutJobs(expect[:1]))
	got = <-jobCh
	deepequal.AssertDeepEqual(t, expect[:1], got)

	// Delete a job. This isn't actually supported, but we should still see
	// a query snapshot. Our code just removes the deleted job from the
	// results, so this should be an empty slice.
	_, err := db.(*firestoreDB).jobs().Doc(expect[1].Id).Delete(ctx)
	assert.NoError(t, err)
	got = <-jobCh
	deepequal.AssertDeepEqual(t, []*types.Job{}, got)

	// Assert that the channel gets closed when we cancel the context.
	cancel()
	_, stillOpen := <-jobCh
	assert.False(t, stillOpen)
}

func TestModifiedTaskCommentsCh(t *testing.T) {
	db, cleanup := setup(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	taskCommentCh := ModifiedTaskCommentsCh(ctx, db)

	// Initial snapshot contains all results, ie. empty right now.
	<-taskCommentCh

	// Add one taskComment, ensure that it shows up.
	expect := []*types.TaskComment{
		{
			Repo:      "my-repo",
			Revision:  "abc",
			Name:      "taskname",
			Timestamp: time.Now(),
		},
	}
	assert.NoError(t, db.PutTaskComment(expect[0]))
	got := <-taskCommentCh
	deepequal.AssertDeepEqual(t, expect, got)

	// Add two taskComments.
	expect = []*types.TaskComment{
		{
			Repo:      "my-repo",
			Revision:  "def",
			Name:      "taskname",
			Timestamp: time.Now(),
		},
		{
			Repo:      "my-repo",
			Revision:  "123",
			Name:      "taskname",
			Timestamp: time.Now(),
		},
	}
	assert.NoError(t, db.PutTaskComment(expect[0]))
	got = <-taskCommentCh
	deepequal.AssertDeepEqual(t, expect[:1], got)
	assert.NoError(t, db.PutTaskComment(expect[1]))
	got = <-taskCommentCh
	deepequal.AssertDeepEqual(t, expect[1:], got)

	// We can't modify TaskComments.

	// Delete a taskComment.
	assert.NoError(t, db.DeleteTaskComment(expect[1]))
	got = <-taskCommentCh
	assert.Equal(t, expect[1].Deleted, &deleted)
	deepequal.AssertDeepEqual(t, expect[1:], got)

	// Assert that the channel gets closed when we cancel the context.
	cancel()
	_, stillOpen := <-taskCommentCh
	assert.False(t, stillOpen)
}

func TestModifiedTaskSpecCommentsCh(t *testing.T) {
	db, cleanup := setup(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	taskSpecCommentCh := ModifiedTaskSpecCommentsCh(ctx, db)

	// Initial snapshot contains all results, ie. empty right now.
	<-taskSpecCommentCh

	// Add one taskSpecComment, ensure that it shows up.
	expect := []*types.TaskSpecComment{
		{
			Repo:      "my-repo",
			Name:      "taskname",
			Timestamp: time.Now(),
		},
	}
	assert.NoError(t, db.PutTaskSpecComment(expect[0]))
	got := <-taskSpecCommentCh
	deepequal.AssertDeepEqual(t, expect, got)

	// Add two taskSpecComments.
	expect = []*types.TaskSpecComment{
		{
			Repo:      "my-repo",
			Name:      "taskname2",
			Timestamp: time.Now(),
		},
		{
			Repo:      "my-repo",
			Name:      "taskname3",
			Timestamp: time.Now(),
		},
	}
	assert.NoError(t, db.PutTaskSpecComment(expect[0]))
	got = <-taskSpecCommentCh
	deepequal.AssertDeepEqual(t, expect[:1], got)
	assert.NoError(t, db.PutTaskSpecComment(expect[1]))
	got = <-taskSpecCommentCh
	deepequal.AssertDeepEqual(t, expect[1:], got)

	// We can't modify TaskSpecComments.

	// Delete a taskSpecComment.
	assert.NoError(t, db.DeleteTaskSpecComment(expect[1]))
	got = <-taskSpecCommentCh
	assert.Equal(t, expect[1].Deleted, &deleted)
	deepequal.AssertDeepEqual(t, expect[1:], got)

	// Assert that the channel gets closed when we cancel the context.
	cancel()
	_, stillOpen := <-taskSpecCommentCh
	assert.False(t, stillOpen)
}

func TestModifiedCommitCommentsCh(t *testing.T) {
	db, cleanup := setup(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	commitCommentCh := ModifiedCommitCommentsCh(ctx, db)

	// Initial snapshot contains all results, ie. empty right now.
	<-commitCommentCh

	// Add one commitComment, ensure that it shows up.
	expect := []*types.CommitComment{
		{
			Repo:      "my-repo",
			Revision:  "abc",
			Timestamp: time.Now(),
		},
	}
	assert.NoError(t, db.PutCommitComment(expect[0]))
	got := <-commitCommentCh
	deepequal.AssertDeepEqual(t, expect, got)

	// Add two commitComments.
	expect = []*types.CommitComment{
		{
			Repo:      "my-repo",
			Revision:  "def",
			Timestamp: time.Now(),
		},
		{
			Repo:      "my-repo",
			Revision:  "123",
			Timestamp: time.Now(),
		},
	}
	assert.NoError(t, db.PutCommitComment(expect[0]))
	got = <-commitCommentCh
	deepequal.AssertDeepEqual(t, expect[:1], got)
	assert.NoError(t, db.PutCommitComment(expect[1]))
	got = <-commitCommentCh
	deepequal.AssertDeepEqual(t, expect[1:], got)

	// We can't modify CommitComments.

	// Delete a commitComment.
	assert.NoError(t, db.DeleteCommitComment(expect[1]))
	got = <-commitCommentCh
	assert.Equal(t, expect[1].Deleted, &deleted)
	deepequal.AssertDeepEqual(t, expect[1:], got)

	// Assert that the channel gets closed when we cancel the context.
	cancel()
	_, stillOpen := <-commitCommentCh
	assert.False(t, stillOpen)
}
