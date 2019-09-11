package firestore

import (
	"context"
	"sync"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/modified"
	"go.skia.org/infra/task_scheduler/go/types"
)

// The DbModified field is set before the document is actually inserted into the
// DB. We have to account for the lag between the timestamp being set and the
// actual modification in the DB when we query for modifications, or we may miss
// modifications.
const dbModifiedLag = DEFAULT_ATTEMPTS * (PUT_MULTI_TIMEOUT + firestore.BACKOFF_WAIT*time.Duration(2^DEFAULT_ATTEMPTS))

// watchModified is a helper function used by WatchModified* which runs
// firestore.WatchQuery with a query by DbModified time and ignores the initial
// set of results. Runs until the given context is cancelled.
func watchModified(ctx context.Context, coll *fs.CollectionRef, field string, cb func(*fs.QuerySnapshot) error) {
	q := coll.Query.Where(field, ">=", time.Now().Add(-dbModifiedLag))
	for {
		if err := ctx.Err(); err != nil {
			sklog.Errorf("%s while watching query.", err)
			return
		}
		first := true
		err := firestore.IterateQuerySnapshots(ctx, q, func(snap *fs.QuerySnapshot) error {
			if first {
				first = false
				return nil
			}
			return cb(snap)
		})
		if err != nil {
			sklog.Errorf("Failed watching query: %s", err)
			// Don't spin at a crazy rate.
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// WatchModifiedTasks calls the given function whenever a Task is added,
// modified, or deleted. If the Task was deleted, the bool parameter will be
// true and the Task parameter will represent the state of the Task at the time
// of deletion.
func WatchModifiedTasks(ctx context.Context, d db.DB, cb func([]*types.Task) error) {
	watchModified(ctx, d.(*firestoreDB).tasks(), KEY_DB_MODIFIED, func(snap *fs.QuerySnapshot) error {
		tasks := make([]*types.Task, 0, len(snap.Changes))
		for _, ch := range snap.Changes {
			var t types.Task
			if err := ch.Doc.DataTo(&t); err != nil {
				return skerr.Wrapf(err, "Failed to decode Task.")
			}
			tasks = append(tasks, &t)
		}
		return cb(tasks)
	})
}

// WatchModifiedJobs calls the given function whenever a Job is added, modified,
// or deleted. If the Job was deleted, the bool parameter will be true and the
// Job parameter will represent the state of the Job at the time of deletion.
func WatchModifiedJobs(ctx context.Context, d db.DB, cb func([]*types.Job) error) {
	watchModified(ctx, d.(*firestoreDB).jobs(), KEY_DB_MODIFIED, func(snap *fs.QuerySnapshot) error {
		jobs := make([]*types.Job, 0, len(snap.Changes))
		for _, ch := range snap.Changes {
			var j types.Job
			if err := ch.Doc.DataTo(&j); err != nil {
				return skerr.Wrapf(err, "Failed to decode Job.")
			}
			jobs = append(jobs, &j)
		}
		return cb(jobs)
	})
}

// WatchModifiedTaskComments calls the given function whenever a TaskComment is
// added, modified, or deleted. If the TaskComment was deleted, the bool
// parameter will be true and the TaskComment parameter will represent the state
// of the TaskComment at the time of deletion.
func WatchModifiedTaskComments(ctx context.Context, d db.DB, cb func([]*types.TaskComment) error) {
	watchModified(ctx, d.(*firestoreDB).taskComments(), KEY_TIMESTAMP, func(snap *fs.QuerySnapshot) error {
		tcs := make([]*types.TaskComment, 0, len(snap.Changes))
		for _, ch := range snap.Changes {
			var c types.TaskComment
			if err := ch.Doc.DataTo(&c); err != nil {
				return skerr.Wrapf(err, "Failed to decode TaskComment.")
			}
			// TODO(borenet): Can we remove the Deleted field?
			if ch.Kind == fs.DocumentRemoved {
				deleted := true
				c.Deleted = &deleted
			}
			tcs = append(tcs, &c)
		}
		return cb(tcs)
	})
}

// WatchModifiedTaskSpecComments calls the given function whenever a
// TaskSpecComment is added, modified, or deleted. If the TaskSpecComment was
// deleted, the bool parameter will be true and the TaskSpecComment parameter
// will represent the state of the TaskSpecComment at the time of deletion.
func WatchModifiedTaskSpecComments(ctx context.Context, d db.DB, cb func([]*types.TaskSpecComment) error) {
	watchModified(ctx, d.(*firestoreDB).taskSpecComments(), KEY_TIMESTAMP, func(snap *fs.QuerySnapshot) error {
		tscs := make([]*types.TaskSpecComment, 0, len(snap.Changes))
		for _, ch := range snap.Changes {
			var c types.TaskSpecComment
			if err := ch.Doc.DataTo(&c); err != nil {
				return skerr.Wrapf(err, "Failed to decode TaskSpecComment.")
			}
			// TODO(borenet): Can we remove the Deleted field?
			if ch.Kind == fs.DocumentRemoved {
				deleted := true
				c.Deleted = &deleted
			}
			tscs = append(tscs, &c)
		}
		return cb(tscs)
	})
}

// WatchModifiedCommitComments calls the given function whenever a CommitComment
// is added,  modified, or deleted. If the CommitComment was deleted, the bool
// parameter will be true and the CommitComment parameter will represent the
// state of the CommitComment at the time of deletion.
func WatchModifiedCommitComments(ctx context.Context, d db.DB, cb func([]*types.CommitComment) error) {
	watchModified(ctx, d.(*firestoreDB).commitComments(), KEY_TIMESTAMP, func(snap *fs.QuerySnapshot) error {
		ccs := make([]*types.CommitComment, 0, len(snap.Changes))
		for _, ch := range snap.Changes {
			var c types.CommitComment
			if err := ch.Doc.DataTo(&c); err != nil {
				return skerr.Wrapf(err, "Failed to decode CommitComment.")
			}
			// TODO(borenet): Can we remove the Deleted field?
			if ch.Kind == fs.DocumentRemoved {
				deleted := true
				c.Deleted = &deleted
			}
			ccs = append(ccs, &c)
		}
		return cb(ccs)
	})
}

// modifiedTasks is an implementation of db.ModifiedTasks which uses Firestore.
type modifiedTasks struct {
	db.ModifiedTasks
	db         db.DB
	started    bool
	startedMtx sync.Mutex
}

// NewModifiedTasks returns a db.ModifiedTasks instance.
func NewModifiedTasks(ctx context.Context, d db.DB) db.ModifiedTasks {
	return &modifiedTasks{
		ModifiedTasks: &modified.ModifiedTasksImpl{},
		db:            d,
	}
}

// See documentation for db.ModifiedTasks interface.
func (m *modifiedTasks) StartTrackingModifiedTasks() (string, error) {
	m.startedMtx.Lock()
	if !m.started {
		go WatchModifiedTasks(context.Background(), m.db, func(tasks []*types.Task) error {
			m.ModifiedTasks.TrackModifiedTasks(tasks)
			return nil
		})
	}
	m.startedMtx.Unlock()
	return m.ModifiedTasks.StartTrackingModifiedTasks()
}

// See documentation for db.ModifiedTasks interface.
func (m *modifiedTasks) TrackModifiedTask(*types.Task) {}

// See documentation for db.ModifiedTasks interface.
func (m *modifiedTasks) TrackModifiedTasks([]*types.Task) {}

// modifiedJobs implements db.ModifiedJobs using Firestore.
type modifiedJobs struct {
	db.ModifiedJobs
	db         db.DB
	started    bool
	startedMtx sync.Mutex
}

// NewModifiedJobs returns a db.ModifiedJobs instance which uses Firestore.
func NewModifiedJobs(ctx context.Context, d db.DB) db.ModifiedJobs {
	return &modifiedJobs{
		ModifiedJobs: &modified.ModifiedJobsImpl{},
		db:           d,
	}
}

// See documentation for db.ModifiedJobs interface.
func (m *modifiedJobs) StartTrackingModifiedJobs() (string, error) {
	m.startedMtx.Lock()
	if !m.started {
		go WatchModifiedJobs(context.Background(), m.db, func(jobs []*types.Job) error {
			m.ModifiedJobs.TrackModifiedJobs(jobs)
			return nil
		})
	}
	m.startedMtx.Unlock()
	return m.ModifiedJobs.StartTrackingModifiedJobs()
}

// See documentation for db.ModifiedJobs interface.
func (m *modifiedJobs) TrackModifiedJob(*types.Job) {}

// See documentation for db.ModifiedJobs interface.
func (m *modifiedJobs) TrackModifiedJobs([]*types.Job) {}

// modifiedComments implements db.ModifiedComments using Firestore.
type modifiedComments struct {
	db.ModifiedComments
	db         db.DB
	started    bool
	startedMtx sync.Mutex
}

// NewModifiedComments returns a db.ModifiedComments instance which uses Firestore.
func NewModifiedComments(ctx context.Context, d db.DB) db.ModifiedComments {
	return &modifiedComments{
		ModifiedComments: &modified.ModifiedCommentsImpl{},
		db:               d,
	}
}

// See documentation for db.ModifiedComments interface.
func (m *modifiedComments) StartTrackingModifiedComments() (string, error) {
	m.startedMtx.Lock()
	if !m.started {
		go WatchModifiedTaskComments(context.Background(), m.db, func(cs []*types.TaskComment) error {
			for _, c := range cs {
				m.ModifiedComments.TrackModifiedTaskComment(c)
			}
			return nil
		})
		go WatchModifiedTaskSpecComments(context.Background(), m.db, func(cs []*types.TaskSpecComment) error {
			for _, c := range cs {
				m.ModifiedComments.TrackModifiedTaskSpecComment(c)
			}
			return nil
		})
		go WatchModifiedCommitComments(context.Background(), m.db, func(cs []*types.CommitComment) error {
			for _, c := range cs {
				m.ModifiedComments.TrackModifiedCommitComment(c)
			}
			return nil
		})
	}
	m.startedMtx.Unlock()
	return m.ModifiedComments.StartTrackingModifiedComments()
}

// See documentation for db.ModifiedComments interface.
func (m *modifiedComments) TrackModifiedCommitComment(*types.CommitComment) {}

// See documentation for db.ModifiedComments interface.
func (m *modifiedComments) TrackModifiedTaskComment(*types.TaskComment) {}

// See documentation for db.ModifiedComments interface.
func (m *modifiedComments) TrackModifiedTaskSpecComment(*types.TaskSpecComment) {}

// NewModifiedData returns a db.ModifiedData instance which uses Firestore.
func NewModifiedData(ctx context.Context, d db.DB) db.ModifiedData {
	return db.NewModifiedData(NewModifiedTasks(ctx, d), NewModifiedJobs(ctx, d), NewModifiedComments(ctx, d))
}
