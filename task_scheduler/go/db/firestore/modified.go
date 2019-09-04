package firestore

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	fs "cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
)

// The DbModified field is set before the document is actually inserted into the
// DB. We have to account for the lag between the timestamp being set and the
// actual modification in the DB when we query for modififications, or we may
// miss modifications.
const dbModifiedLag = DEFAULT_ATTEMPTS * (PUT_MULTI_TIMEOUT + firestore.BACKOFF_WAIT*time.Duration(2^DEFAULT_ATTEMPTS))

// watchModified is a helper function used by WatchModified* which runs
// firestore.WatchQuery with a query by DbModified time and ignores the initial
// set of results.
func watchModified(ctx context.Context, coll *fs.CollectionRef, field string, cb func(*fs.DocumentSnapshot, bool) error) error {
	q := coll.Query.Where(field, ">=", time.Now().Add(-dbModifiedLag))
	return firestore.WatchQuery(ctx, q, true, func(snap *fs.DocumentSnapshot, deleted bool) error {
		if err := cb(snap, deleted); err != nil {
			return err
		}
		return nil
	})
}

// WatchModifiedTasks calls the given function whenever a Task is added,
// modified, or deleted. If the Task was deleted, the bool parameter will be
// true and the Task parameter will represent the state of the Task at the time
// of deletion.
func WatchModifiedTasks(ctx context.Context, d db.DB, cb func(*types.Task, bool) error) error {
	return watchModified(ctx, d.(*firestoreDB).tasks(), KEY_DB_MODIFIED, func(snap *fs.DocumentSnapshot, deleted bool) error {
		var t types.Task
		if err := snap.DataTo(&t); err != nil {
			return skerr.Wrapf(err, "Failed to decode Task.")
		}
		return cb(&t, deleted)
	})
}

// WatchModifiedJobs calls the given function whenever a Job is added, modified,
// or deleted. If the Job was deleted, the bool parameter will be true and the
// Job parameter will represent the state of the Job at the time of deletion.
func WatchModifiedJobs(ctx context.Context, d db.DB, cb func(*types.Job, bool) error) error {
	return watchModified(ctx, d.(*firestoreDB).jobs(), KEY_DB_MODIFIED, func(snap *fs.DocumentSnapshot, deleted bool) error {
		var j types.Job
		if err := snap.DataTo(&j); err != nil {
			return skerr.Wrapf(err, "Failed to decode Job.")
		}
		return cb(&j, deleted)
	})
}

// WatchModifiedTaskComments calls the given function whenever a TaskComment is
// added, modified, or deleted. If the TaskComment was deleted, the bool
// parameter will be true and the TaskComment parameter will represent the state
// of the TaskComment at the time of deletion.
func WatchModifiedTaskComments(ctx context.Context, d db.DB, cb func(*types.TaskComment, bool) error) error {
	return watchModified(ctx, d.(*firestoreDB).taskComments(), KEY_TIMESTAMP, func(snap *fs.DocumentSnapshot, deleted bool) error {
		var c types.TaskComment
		if err := snap.DataTo(&c); err != nil {
			return skerr.Wrapf(err, "Failed to decode TaskComment.")
		}
		// TODO(borenet): Can we remove the Deleted field?
		if deleted {
			deleted := true
			c.Deleted = &deleted
		}
		return cb(&c, deleted)
	})
}

// WatchModifiedTaskSpecComments calls the given function whenever a
// TaskSpecComment is added, modified, or deleted. If the TaskSpecComment was
// deleted, the bool parameter will be true and the TaskSpecComment parameter
// will represent the state of the TaskSpecComment at the time of deletion.
func WatchModifiedTaskSpecComments(ctx context.Context, d db.DB, cb func(*types.TaskSpecComment, bool) error) error {
	return watchModified(ctx, d.(*firestoreDB).taskSpecComments(), KEY_TIMESTAMP, func(snap *fs.DocumentSnapshot, deleted bool) error {
		var c types.TaskSpecComment
		if err := snap.DataTo(&c); err != nil {
			return skerr.Wrapf(err, "Failed to decode TaskSpecComment.")
		}
		// TODO(borenet): Can we remove the Deleted field?
		if deleted {
			deleted := true
			c.Deleted = &deleted
		}
		return cb(&c, deleted)
	})
}

// WatchModifiedCommitComments calls the given function whenever a CommitComment
// is added,  modified, or deleted. If the CommitComment was deleted, the bool
// parameter will be true and the CommitComment parameter will represent the
// state of the CommitComment at the time of deletion.
func WatchModifiedCommitComments(ctx context.Context, d db.DB, cb func(*types.CommitComment, bool) error) error {
	return watchModified(ctx, d.(*firestoreDB).commitComments(), KEY_TIMESTAMP, func(snap *fs.DocumentSnapshot, deleted bool) error {
		var c types.CommitComment
		if err := snap.DataTo(&c); err != nil {
			return skerr.Wrapf(err, "Failed to decode CommitComment.")
		}
		// TODO(borenet): Can we remove the Deleted field?
		if deleted {
			deleted := true
			c.Deleted = &deleted
		}
		return cb(&c, deleted)
	})
}

// modifiedData is a helper function for implementing db.Modified*
type modifiedData struct {
	db       *firestoreDB
	done     map[string]chan<- struct{}
	errs     map[string]error
	modified map[string][]*fs.DocumentSnapshot
	deleted  map[string][]bool
	mtx      sync.Mutex // Protects done, errs, modified, and deleted.
}

// newModifiedData returns a modifiedData instance.
func newModifiedData(d db.DB) *modifiedData {
	return &modifiedData{
		db:       d.(*firestoreDB),
		done:     map[string]chan<- struct{}{},
		errs:     map[string]error{},
		modified: map[string][]*fs.DocumentSnapshot{},
		deleted:  map[string][]bool{},
	}
}

// getModifiedData returns the DocumentSnapshots stored for the given id.
func (m *modifiedData) getModifiedData(id string) ([]*fs.DocumentSnapshot, []bool, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	mod, ok := m.modified[id]
	if !ok {
		return nil, nil, db.ErrUnknownId
	}
	del, ok := m.deleted[id]
	if !ok {
		return nil, nil, db.ErrUnknownId
	}
	m.modified[id] = []*fs.DocumentSnapshot{}
	m.deleted[id] = []bool{}
	return mod, del, nil
}

// startTrackingModifiedData spins up a goroutine to run watchModified and store
// the DocumentSnapshots it produces, returning an ID which can be passed to
// getModifiedData to retrieve the results.
func (m *modifiedData) startTrackingModifiedData(coll *fs.CollectionRef, field string) (string, error) {
	id := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		<-done
		cancel()
	}()
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.done[id] = done
	m.modified[id] = []*fs.DocumentSnapshot{}
	m.deleted[id] = []bool{}
	go func() {
		err := watchModified(ctx, coll, field, func(snap *fs.DocumentSnapshot, deleted bool) error {
			m.mtx.Lock()
			defer m.mtx.Unlock()
			m.modified[id] = append(m.modified[id], snap)
			m.deleted[id] = append(m.deleted[id], deleted)
			return nil
		})
		if err != nil {
			m.mtx.Lock()
			defer m.mtx.Unlock()
			m.errs[id] = err
		}
	}()
	return id, nil
}

// stopTrackingModifiedData stops the goroutine associated with the given ID.
func (m *modifiedData) stopTrackingModifiedData(id string) {
	m.mtx.Lock()
	ch := m.done[id]
	m.mtx.Unlock()
	ch <- struct{}{}
}

// modifiedTasks implements db.ModifiedTasks using Firestore.
type modifiedTasks struct {
	*modifiedData
}

// NewModifiedTasks returns a db.ModifiedTasks instance which uses Firestore.
func NewModifiedTasks(d db.DB) db.ModifiedTasks {
	return &modifiedTasks{newModifiedData(d)}
}

// See documentation for db.ModifiedTasks interface.
func (m *modifiedTasks) GetModifiedTasks(id string) ([]*types.Task, error) {
	mod, _, err := m.getModifiedData(id)
	if err != nil {
		return nil, err
	}
	rv := make([]*types.Task, 0, len(mod))
	for _, snap := range mod {
		var t types.Task
		if err := snap.DataTo(&t); err != nil {
			return nil, err
		}
		rv = append(rv, &t)
	}
	return rv, nil
}

// See documentation for db.ModifiedTasks interface.
func (m *modifiedTasks) GetModifiedTasksGOB(id string) (map[string][]byte, error) {
	return nil, errors.New("Not implemented")
}

// See documentation for db.ModifiedTasks interface.
func (m *modifiedTasks) StartTrackingModifiedTasks() (string, error) {
	return m.startTrackingModifiedData(m.db.tasks(), KEY_DB_MODIFIED)
}

// See documentation for db.ModifiedTasks interface.
func (m *modifiedTasks) StopTrackingModifiedTasks(id string) {
	m.stopTrackingModifiedData(id)
}

// See documentation for db.ModifiedTasks interface.
func (m *modifiedTasks) TrackModifiedTask(*types.Task) {}

// See documentation for db.ModifiedTasks interface.
func (m *modifiedTasks) TrackModifiedTasksGOB(time.Time, map[string][]byte) {}

// modifiedJobs implements db.ModifiedJobs using Firestore.
type modifiedJobs struct {
	*modifiedData
}

// NewModifiedJobs returns a db.ModifiedJobs instance which uses Firestore.
func NewModifiedJobs(d db.DB) db.ModifiedJobs {
	return &modifiedJobs{newModifiedData(d)}
}

// See documentation for db.ModifiedJobs interface.
func (m *modifiedJobs) GetModifiedJobs(id string) ([]*types.Job, error) {
	mod, _, err := m.getModifiedData(id)
	if err != nil {
		return nil, err
	}
	rv := make([]*types.Job, 0, len(mod))
	for _, snap := range mod {
		var t types.Job
		if err := snap.DataTo(&t); err != nil {
			return nil, err
		}
		rv = append(rv, &t)
	}
	return rv, nil
}

// See documentation for db.ModifiedJobs interface.
func (m *modifiedJobs) GetModifiedJobsGOB(id string) (map[string][]byte, error) {
	return nil, errors.New("Not implemented")
}

// See documentation for db.ModifiedJobs interface.
func (m *modifiedJobs) StartTrackingModifiedJobs() (string, error) {
	return m.startTrackingModifiedData(m.db.jobs(), KEY_DB_MODIFIED)
}

// See documentation for db.ModifiedJobs interface.
func (m *modifiedJobs) StopTrackingModifiedJobs(id string) {
	m.stopTrackingModifiedData(id)
}

// See documentation for db.ModifiedJobs interface.
func (m *modifiedJobs) TrackModifiedJob(*types.Job) {}

// See documentation for db.ModifiedJobs interface.
func (m *modifiedJobs) TrackModifiedJobsGOB(time.Time, map[string][]byte) {}

// modifiedComments implements db.ModifiedComments using Firestore.
type modifiedComments struct {
	commits   *modifiedData
	tasks     *modifiedData
	taskSpecs *modifiedData
}

// NewModifiedComments returns a db.ModifiedComments instance which uses Firestore.
func NewModifiedComments(d db.DB) db.ModifiedComments {
	return &modifiedComments{
		commits:   newModifiedData(d),
		tasks:     newModifiedData(d),
		taskSpecs: newModifiedData(d),
	}
}

// See documentation for db.ModifiedComments interface.
func (m *modifiedComments) GetModifiedComments(id string) ([]*types.TaskComment, []*types.TaskSpecComment, []*types.CommitComment, error) {
	ids := strings.Split(id, "#")
	if len(ids) != 3 {
		return nil, nil, nil, db.ErrUnknownId
	}
	deleted := true
	tasks, delTasks, err := m.tasks.getModifiedData(ids[0])
	if err != nil {
		return nil, nil, nil, err
	}
	rv1 := make([]*types.TaskComment, 0, len(tasks))
	for idx, snap := range tasks {
		var c types.TaskComment
		if err := snap.DataTo(&c); err != nil {
			return nil, nil, nil, err
		}
		// TODO(borenet): Can we remove the Deleted field?
		if delTasks[idx] {
			c.Deleted = &deleted
		}
		rv1 = append(rv1, &c)
	}
	sort.Sort(types.TaskCommentSlice(rv1))

	taskSpecs, delTaskSpecs, err := m.taskSpecs.getModifiedData(ids[1])
	if err != nil {
		return nil, nil, nil, err
	}
	rv2 := make([]*types.TaskSpecComment, 0, len(taskSpecs))
	for idx, snap := range taskSpecs {
		var c types.TaskSpecComment
		if err := snap.DataTo(&c); err != nil {
			return nil, nil, nil, err
		}
		// TODO(borenet): Can we remove the Deleted field?
		if delTaskSpecs[idx] {
			c.Deleted = &deleted
		}
		rv2 = append(rv2, &c)
	}
	sort.Sort(types.TaskSpecCommentSlice(rv2))

	commits, delCommits, err := m.commits.getModifiedData(ids[2])
	if err != nil {
		return nil, nil, nil, err
	}
	rv3 := make([]*types.CommitComment, 0, len(commits))
	for idx, snap := range commits {
		var c types.CommitComment
		if err := snap.DataTo(&c); err != nil {
			return nil, nil, nil, err
		}
		// TODO(borenet): Can we remove the Deleted field?
		if delCommits[idx] {
			c.Deleted = &deleted
		}
		rv3 = append(rv3, &c)
	}
	sort.Sort(types.CommitCommentSlice(rv3))

	return rv1, rv2, rv3, nil
}

// See documentation for db.ModifiedComments interface.
func (m *modifiedComments) StartTrackingModifiedComments() (string, error) {
	id1, err := m.tasks.startTrackingModifiedData(m.tasks.db.taskComments(), KEY_TIMESTAMP)
	if err != nil {
		return "", nil
	}
	id2, err := m.taskSpecs.startTrackingModifiedData(m.taskSpecs.db.taskSpecComments(), KEY_TIMESTAMP)
	if err != nil {
		m.tasks.stopTrackingModifiedData(id1)
		return "", nil
	}
	id3, err := m.commits.startTrackingModifiedData(m.commits.db.commitComments(), KEY_TIMESTAMP)
	if err != nil {
		m.tasks.stopTrackingModifiedData(id1)
		m.tasks.stopTrackingModifiedData(id2)
		return "", nil
	}
	return id1 + "#" + id2 + "#" + id3, nil
}

// See documentation for db.ModifiedComments interface.
func (m *modifiedComments) StopTrackingModifiedComments(id string) {
	ids := strings.Split(id, "#")
	if len(ids) != 3 {
		sklog.Errorf("Invalid id %q", id)
		return
	}
	m.tasks.stopTrackingModifiedData(ids[0])
	m.taskSpecs.stopTrackingModifiedData(ids[1])
	m.commits.stopTrackingModifiedData(ids[2])
}

// See documentation for db.ModifiedComments interface.
func (m *modifiedComments) TrackModifiedCommitComment(*types.CommitComment) {}

// See documentation for db.ModifiedComments interface.
func (m *modifiedComments) TrackModifiedTaskComment(*types.TaskComment) {}

// See documentation for db.ModifiedComments interface.
func (m *modifiedComments) TrackModifiedTaskSpecComment(*types.TaskSpecComment) {}

// NewModifiedData returns a db.ModifiedData instance which uses Firestore.
func NewModifiedData(d db.DB) db.ModifiedData {
	return db.NewModifiedData(NewModifiedTasks(d), NewModifiedJobs(d), NewModifiedComments(d))
}
