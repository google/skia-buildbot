package firestore

import (
	"context"
	"slices"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/task_scheduler/go/types"
)

// The DbModified field is set before the document is actually inserted into the
// DB. We have to account for the lag between the timestamp being set and the
// actual modification in the DB when we query for modifications, or we may miss
// modifications.
const dbModifiedLag = DEFAULT_ATTEMPTS * (PUT_MULTI_TIMEOUT + firestore.BACKOFF_WAIT*time.Duration(2^DEFAULT_ATTEMPTS))

// modifiedCh is a helper function used by Modified* which runs
// firestore.QuerySnapshotChannel with a query by DbModified time. Starts a
// goroutine that runs until the given context is cancelled, at which point the
// returned channel is closed.
func modifiedCh[T any](ctx context.Context, coll *fs.CollectionRef, field string) <-chan firestore.QuerySnapshotChanges[T] {
	outCh := make(chan firestore.QuerySnapshotChanges[T])
	lastSnapTime := now.Now(ctx)
	makeQuery := func() fs.Query {
		return coll.Query.Where(field, ">=", lastSnapTime.Add(-dbModifiedLag))
	}
	go func() {
		defer close(outCh)
		for changes := range firestore.QuerySnapshotChannel[T](ctx, makeQuery) {
			lastSnapTime = changes.Timestamp
			outCh <- changes
		}
	}()
	return outCh
}

// ModifiedTasksCh passes slices of Tasks along the returned channel as they are
// modified in the DB.
func (d *firestoreDB) ModifiedTasksCh(ctx context.Context) <-chan []*types.Task {
	outCh := make(chan []*types.Task)
	go func() {
		defer close(outCh)
		for changes := range modifiedCh[types.Task](ctx, d.tasks(), KEY_DB_MODIFIED) {
			outCh <- slices.Concat(changes.Added, changes.Modified)
		}
	}()
	return outCh
}

// ModifiedJobsCh passes slices of Jobs along the returned channel as they are
// modified in the DB.
func (d *firestoreDB) ModifiedJobsCh(ctx context.Context) <-chan []*types.Job {
	outCh := make(chan []*types.Job)
	go func() {
		defer close(outCh)
		for changes := range modifiedCh[types.Job](ctx, d.jobs(), KEY_DB_MODIFIED) {
			outCh <- slices.Concat(changes.Added, changes.Modified)
		}
	}()
	return outCh
}

// ModifiedTaskCommentsCh passes slices of TaskComments along the returned channel
// as they are modified in the DB.
func (d *firestoreDB) ModifiedTaskCommentsCh(ctx context.Context) <-chan []*types.TaskComment {
	outCh := make(chan []*types.TaskComment)
	go func() {
		defer close(outCh)
		for changes := range modifiedCh[types.TaskComment](ctx, d.taskComments(), KEY_TIMESTAMP) {
			for _, c := range changes.Removed {
				deleted := true
				c.Deleted = &deleted
			}
			outCh <- slices.Concat(changes.Added, changes.Modified, changes.Removed)
		}
	}()
	return outCh
}

// ModifiedTaskSpecCommentsCh passes slices of TaskSpecComments along the returned
// channel as they are modified in the DB.
func (d *firestoreDB) ModifiedTaskSpecCommentsCh(ctx context.Context) <-chan []*types.TaskSpecComment {
	outCh := make(chan []*types.TaskSpecComment)
	go func() {
		defer close(outCh)
		for changes := range modifiedCh[types.TaskSpecComment](ctx, d.taskSpecComments(), KEY_TIMESTAMP) {
			for _, c := range changes.Removed {
				deleted := true
				c.Deleted = &deleted
			}
			outCh <- slices.Concat(changes.Added, changes.Modified, changes.Removed)
		}
	}()
	return outCh
}

// ModifiedCommitCommentsCh passes slices of CommitComments along the returned
// channel as they are modified in the DB.
func (d *firestoreDB) ModifiedCommitCommentsCh(ctx context.Context) <-chan []*types.CommitComment {
	outCh := make(chan []*types.CommitComment)
	go func() {
		defer close(outCh)
		for changes := range modifiedCh[types.CommitComment](ctx, d.commitComments(), KEY_TIMESTAMP) {
			for _, c := range changes.Removed {
				deleted := true
				c.Deleted = &deleted
			}
			outCh <- slices.Concat(changes.Added, changes.Modified, changes.Removed)
		}
	}()
	return outCh
}
