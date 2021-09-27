package db

import (
	"context"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/task_scheduler/go/types"
)

var deleted = true

func TestModifiedTasksCh(t sktest.TestingT, db DB) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	taskCh := db.ModifiedTasksCh(ctx)

	// Initial snapshot contains all results, ie. empty right now.
	<-taskCh

	// Add one task, ensure that it shows up.
	expect := []*types.Task{
		{
			Id:      "0",
			Created: time.Now(),
		},
	}
	require.NoError(t, db.PutTasks(ctx, expect))
	got := <-taskCh
	assertdeep.Equal(t, expect, got)

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
	require.NoError(t, db.PutTasks(ctx, expect))
	got = <-taskCh
	assertdeep.Equal(t, expect, got)

	// Modify a task.
	expect[0].Name = "my-task"
	require.NoError(t, db.PutTasks(ctx, expect[:1]))
	got = <-taskCh
	assertdeep.Equal(t, expect[:1], got)

	// Create a second channel. Modify an entry after we receive it and
	// ensure that we don't see that modification elsewhere.
	dupCh := db.ModifiedTasksCh(ctx)
	<-dupCh // Ignore first snapshot.
	expect[0].Name = "renamed again"
	require.NoError(t, db.PutTasks(ctx, expect[:1]))
	expect[0].Name = "changed my mind"
	got1 := <-taskCh
	got2 := <-dupCh
	assertdeep.Equal(t, got1, got2)
	require.Equal(t, "renamed again", got1[0].Name)
	got1[0].Name = "changed my mind again"
	require.Equal(t, "renamed again", got2[0].Name)

	// Assert that the channel gets closed when we cancel the context.
	cancel()
	_, stillOpen := <-taskCh
	require.False(t, stillOpen)
}

func TestModifiedJobsCh(t sktest.TestingT, db DB) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobCh := db.ModifiedJobsCh(ctx)

	// Initial snapshot contains all results, ie. empty right now.
	<-jobCh

	// Add one job, ensure that it shows up.
	expect := []*types.Job{
		{
			Id:      "0",
			Created: time.Now(),
		},
	}
	require.NoError(t, db.PutJobs(ctx, expect))
	got := <-jobCh
	assertdeep.Equal(t, expect, got)

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
	require.NoError(t, db.PutJobs(ctx, expect))
	got = <-jobCh
	assertdeep.Equal(t, expect, got)

	// Modify a job.
	expect[0].Name = "my-job"
	require.NoError(t, db.PutJobs(ctx, expect[:1]))
	got = <-jobCh
	assertdeep.Equal(t, expect[:1], got)

	// Create a second channel. Modify an entry after we receive it and
	// ensure that we don't see that modification elsewhere.
	dupCh := db.ModifiedJobsCh(ctx)
	<-dupCh // Ignore first snapshot.
	expect[0].Name = "renamed again"
	require.NoError(t, db.PutJobs(ctx, expect[:1]))
	expect[0].Name = "changed my mind"
	got1 := <-jobCh
	got2 := <-dupCh
	assertdeep.Equal(t, got1, got2)
	require.Equal(t, "renamed again", got1[0].Name)
	got1[0].Name = "changed my mind again"
	require.Equal(t, "renamed again", got2[0].Name)

	// Assert that the channel gets closed when we cancel the context.
	cancel()
	_, stillOpen := <-jobCh
	require.False(t, stillOpen)
}

func TestModifiedTaskCommentsCh(t sktest.TestingT, db DB) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	taskCommentCh := db.ModifiedTaskCommentsCh(ctx)

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
	require.NoError(t, db.PutTaskComment(ctx, expect[0]))
	got := <-taskCommentCh
	assertdeep.Equal(t, expect, got)

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
	require.NoError(t, db.PutTaskComment(ctx, expect[0]))
	got = <-taskCommentCh
	assertdeep.Equal(t, expect[:1], got)
	require.NoError(t, db.PutTaskComment(ctx, expect[1]))
	got = <-taskCommentCh
	assertdeep.Equal(t, expect[1:], got)

	// We can't modify TaskComments.

	// Delete a taskComment.
	require.NoError(t, db.DeleteTaskComment(ctx, expect[1]))
	got = <-taskCommentCh
	require.Equal(t, expect[1].Deleted, &deleted)
	assertdeep.Equal(t, expect[1:], got)

	// Create a second channel. Modify an entry after we receive it and
	// ensure that we don't see that modification elsewhere.
	dupCh := db.ModifiedTaskCommentsCh(ctx)
	<-dupCh // Ignore first snapshot.
	expect[0].Name = "renamed again"
	require.NoError(t, db.PutTaskComment(ctx, expect[0]))
	expect[0].Name = "changed my mind"
	got1 := <-taskCommentCh
	got2 := <-dupCh
	assertdeep.Equal(t, got1, got2)
	require.Equal(t, "renamed again", got1[0].Name)
	got1[0].Name = "changed my mind again"
	require.Equal(t, "renamed again", got2[0].Name)

	// Assert that the channel gets closed when we cancel the context.
	cancel()
	_, stillOpen := <-taskCommentCh
	require.False(t, stillOpen)
}

func TestModifiedTaskSpecCommentsCh(t sktest.TestingT, db DB) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	taskSpecCommentCh := db.ModifiedTaskSpecCommentsCh(ctx)

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
	require.NoError(t, db.PutTaskSpecComment(ctx, expect[0]))
	got := <-taskSpecCommentCh
	assertdeep.Equal(t, expect, got)

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
	require.NoError(t, db.PutTaskSpecComment(ctx, expect[0]))
	got = <-taskSpecCommentCh
	assertdeep.Equal(t, expect[:1], got)
	require.NoError(t, db.PutTaskSpecComment(ctx, expect[1]))
	got = <-taskSpecCommentCh
	assertdeep.Equal(t, expect[1:], got)

	// We can't modify TaskSpecComments.

	// Delete a taskSpecComment.
	require.NoError(t, db.DeleteTaskSpecComment(ctx, expect[1]))
	got = <-taskSpecCommentCh
	require.Equal(t, expect[1].Deleted, &deleted)
	assertdeep.Equal(t, expect[1:], got)

	// Create a second channel. Modify an entry after we receive it and
	// ensure that we don't see that modification elsewhere.
	dupCh := db.ModifiedTaskSpecCommentsCh(ctx)
	<-dupCh // Ignore first snapshot.
	expect[0].Name = "renamed again"
	require.NoError(t, db.PutTaskSpecComment(ctx, expect[0]))
	expect[0].Name = "changed my mind"
	got1 := <-taskSpecCommentCh
	got2 := <-dupCh
	assertdeep.Equal(t, got1, got2)
	require.Equal(t, "renamed again", got1[0].Name)
	got1[0].Name = "changed my mind again"
	require.Equal(t, "renamed again", got2[0].Name)

	// Assert that the channel gets closed when we cancel the context.
	cancel()
	_, stillOpen := <-taskSpecCommentCh
	require.False(t, stillOpen)
}

func TestModifiedCommitCommentsCh(t sktest.TestingT, db DB) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	commitCommentCh := db.ModifiedCommitCommentsCh(ctx)

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
	require.NoError(t, db.PutCommitComment(ctx, expect[0]))
	got := <-commitCommentCh
	assertdeep.Equal(t, expect, got)

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
	require.NoError(t, db.PutCommitComment(ctx, expect[0]))
	got = <-commitCommentCh
	assertdeep.Equal(t, expect[:1], got)
	require.NoError(t, db.PutCommitComment(ctx, expect[1]))
	got = <-commitCommentCh
	assertdeep.Equal(t, expect[1:], got)

	// We can't modify CommitComments.

	// Delete a commitComment.
	require.NoError(t, db.DeleteCommitComment(ctx, expect[1]))
	got = <-commitCommentCh
	require.Equal(t, expect[1].Deleted, &deleted)
	assertdeep.Equal(t, expect[1:], got)

	// Create a second channel. Modify an entry after we receive it and
	// ensure that we don't see that modification elsewhere.
	dupCh := db.ModifiedCommitCommentsCh(ctx)
	<-dupCh // Ignore first snapshot.
	expect[0].Revision = "renamed again"
	require.NoError(t, db.PutCommitComment(ctx, expect[0]))
	expect[0].Revision = "changed my mind"
	got1 := <-commitCommentCh
	got2 := <-dupCh
	assertdeep.Equal(t, got1, got2)
	require.Equal(t, "renamed again", got1[0].Revision)
	got1[0].Revision = "changed my mind again"
	require.Equal(t, "renamed again", got2[0].Revision)

	// Assert that the channel gets closed when we cancel the context.
	cancel()
	_, stillOpen := <-commitCommentCh
	require.False(t, stillOpen)
}
