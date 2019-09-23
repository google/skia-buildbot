package firestore

import (
	"context"

	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/modified"
)

// NewModifiedTasks returns an instance of db.ModifiedTasks which uses
// ModifiedTasksCh.
func NewModifiedTasks(ctx context.Context, d db.DB) db.ModifiedTasks {
	rv := &modified.ModifiedTasksImpl{}
	go func() {
		for tasks := range ModifiedTasksCh(ctx, d) {
			rv.TrackModifiedTasks(tasks)
		}
	}()
	return rv
}

// NewModifiedJobs returns an instance of db.ModifiedJobs which uses
// ModifiedJobsCh.
func NewModifiedJobs(ctx context.Context, d db.DB) db.ModifiedJobs {
	rv := &modified.ModifiedJobsImpl{}
	go func() {
		for jobs := range ModifiedJobsCh(ctx, d) {
			rv.TrackModifiedJobs(jobs)
		}
	}()
	return rv
}

// NewModifiedComments returns an instance of db.ModifiedComments which uses
// ModifiedCommentsCh.
func NewModifiedComments(ctx context.Context, d db.DB) db.ModifiedComments {
	rv := &modified.ModifiedCommentsImpl{}
	go func() {
		for comments := range ModifiedTaskCommentsCh(ctx, d) {
			for _, c := range comments {
				rv.TrackModifiedTaskComment(c)
			}
		}
	}()
	go func() {
		for comments := range ModifiedTaskSpecCommentsCh(ctx, d) {
			for _, c := range comments {
				rv.TrackModifiedTaskSpecComment(c)
			}
		}
	}()
	go func() {
		for comments := range ModifiedCommitCommentsCh(ctx, d) {
			for _, c := range comments {
				rv.TrackModifiedCommitComment(c)
			}
		}
	}()
	return rv
}
