package firestore

import (
	"context"
	"time"

	fs "cloud.google.com/go/firestore"
	"github.com/cenkalti/backoff"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/sklog"
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
func modifiedCh(ctx context.Context, coll *fs.CollectionRef, field string) <-chan *fs.QuerySnapshot {
	// Note: we could specify the wait intervals for the exponential
	// backoff, but the defaults seem like a reasonable place to start.
	backoffWait := backoff.NewExponentialBackOff()
	outCh := make(chan *fs.QuerySnapshot)
	go func() {
		defer close(outCh)
		lastSnapTime := now.Now(ctx)

		// Loop and resume watching if the input channel is closed.
		for {
			// It's important that we recreate the query inside the
			// loop, otherwise we may restart iteration with an old
			// timestamp and generate a bunch of old results.
			q := coll.Query.Where(field, ">=", lastSnapTime.Add(-dbModifiedLag))
			for qsnap := range firestore.QuerySnapshotChannel(ctx, q) {
				lastSnapTime = qsnap.ReadTime
				backoffWait.Reset()
				outCh <- qsnap
			}

			// Respect context cancellation.
			if err := ctx.Err(); err != nil {
				sklog.Warningf("%s while watching query.", err)
				return
			}

			// If we reached this point, the QuerySnapshotIterator
			// has stopped for some reason. Restart it after a brief
			// wait in case it's repeatedly failing.
			time.Sleep(backoffWait.NextBackOff())
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
		for snap := range modifiedCh(ctx, d.tasks(), KEY_DB_MODIFIED) {
			tasks := make([]*types.Task, 0, len(snap.Changes))
			for _, ch := range snap.Changes {
				// We don't support deletion of tasks, but this
				// is where we'd handle it if we did.
				if ch.Kind != fs.DocumentRemoved {
					var t types.Task
					if err := ch.Doc.DataTo(&t); err != nil {
						sklog.Errorf("Failed to decode Task: %+v", ch.Doc.Data())
						continue
					}
					tasks = append(tasks, &t)
				}
			}
			outCh <- tasks
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
		for snap := range modifiedCh(ctx, d.jobs(), KEY_DB_MODIFIED) {
			jobs := make([]*types.Job, 0, len(snap.Changes))
			for _, ch := range snap.Changes {
				// We don't support deletion of jobs, but this
				// is where we'd handle it if we did.
				if ch.Kind != fs.DocumentRemoved {
					var j types.Job
					if err := ch.Doc.DataTo(&j); err != nil {
						sklog.Errorf("Failed to decode Job: %+v", ch.Doc.Data())
						continue
					}
					jobs = append(jobs, &j)
				}
			}
			outCh <- jobs
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
		for snap := range modifiedCh(ctx, d.taskComments(), KEY_TIMESTAMP) {
			cs := make([]*types.TaskComment, 0, len(snap.Changes))
			for _, ch := range snap.Changes {
				var c types.TaskComment
				if err := ch.Doc.DataTo(&c); err != nil {
					sklog.Errorf("Failed to decode TaskComment: %+v", ch.Doc.Data())
					continue
				}
				// If the comment was removed, set the Deleted
				// field.
				if ch.Kind == fs.DocumentRemoved {
					deleted := true
					c.Deleted = &deleted
				}
				cs = append(cs, &c)
			}
			outCh <- cs
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
		for snap := range modifiedCh(ctx, d.taskSpecComments(), KEY_TIMESTAMP) {
			cs := make([]*types.TaskSpecComment, 0, len(snap.Changes))
			for _, ch := range snap.Changes {
				var c types.TaskSpecComment
				if err := ch.Doc.DataTo(&c); err != nil {
					sklog.Errorf("Failed to decode TaskSpecComment: %+v", ch.Doc.Data())
					continue
				}
				// If the comment was removed, set the Deleted
				// field.
				if ch.Kind == fs.DocumentRemoved {
					deleted := true
					c.Deleted = &deleted
				}
				cs = append(cs, &c)
			}
			outCh <- cs
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
		for snap := range modifiedCh(ctx, d.commitComments(), KEY_TIMESTAMP) {
			cs := make([]*types.CommitComment, 0, len(snap.Changes))
			for _, ch := range snap.Changes {
				var c types.CommitComment
				if err := ch.Doc.DataTo(&c); err != nil {
					sklog.Errorf("Failed to decode CommitComment: %+v", ch.Doc.Data())
					continue
				}
				// If the comment was removed, set the Deleted
				// field.
				if ch.Kind == fs.DocumentRemoved {
					deleted := true
					c.Deleted = &deleted
				}
				cs = append(cs, &c)
			}
			outCh <- cs
		}
	}()
	return outCh
}
