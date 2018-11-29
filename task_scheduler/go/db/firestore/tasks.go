package firestore

import (
	"context"
	"fmt"
	"sort"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	COLLECTION_TASKS = "tasks"
)

// Fix all timestamps for the given task.
func fixTaskTimestamps(task *types.Task) {
	task.Created = fixTimestamp(task.Created)
	task.DbModified = fixTimestamp(task.DbModified)
	task.Finished = fixTimestamp(task.Finished)
	task.Started = fixTimestamp(task.Started)
}

// tasks returns a reference to the tasks collection.
func (d *firestoreDB) tasks() *fs.CollectionRef {
	return d.client.Collection(COLLECTION_TASKS)
}

// See documentation for types.TaskReader interface.
func (d *firestoreDB) GetTaskById(id string) (*types.Task, error) {
	doc, err := firestore.Get(d.tasks().Doc(id), DEFAULT_ATTEMPTS, GET_SINGLE_TIMEOUT)
	if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var rv types.Task
	if err := doc.DataTo(&rv); err != nil {
		return nil, err
	}
	return &rv, nil
}

// See documentation for types.TaskReader interface.
func (d *firestoreDB) GetTasksFromDateRange(start, end time.Time, repo string) ([]*types.Task, error) {
	var tasks [][]*types.Task
	init := func(numGoroutines int) {
		tasks = make([][]*types.Task, numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			estResults := estResultSize(end.Sub(start) / time.Duration(numGoroutines))
			tasks[i] = make([]*types.Task, 0, estResults)
		}
	}
	elem := func(idx int, doc *fs.DocumentSnapshot) error {
		var task types.Task
		if err := doc.DataTo(&task); err != nil {
			return err
		}
		if repo == "" || task.Repo == repo {
			tasks[idx] = append(tasks[idx], &task)
		}
		return nil
	}
	if err := d.dateRangeHelper(d.tasks(), start, end, init, elem); err != nil {
		return nil, err
	}
	totalResults := 0
	for _, taskList := range tasks {
		totalResults += len(taskList)
	}
	rv := make([]*types.Task, 0, totalResults)
	for _, taskList := range tasks {
		rv = append(rv, taskList...)
	}
	sort.Sort(types.TaskSlice(rv))
	return rv, nil
}

// See documentation for types.TaskDB interface.
func (d *firestoreDB) AssignId(task *types.Task) error {
	task.Id = d.tasks().NewDoc().ID
	return nil
}

// putTasks sets the contents of the given tasks in Firestore, as part of the
// given transaction. It is used by PutTask and PutTasks.
func (d *firestoreDB) putTasks(tasks []*types.Task, tx *fs.Transaction) (rvErr error) {
	// Set the modification time of the tasks.
	now := fixTimestamp(time.Now())
	isNew := make([]bool, len(tasks))
	prevModified := make([]time.Time, len(tasks))
	for idx, task := range tasks {
		if util.TimeIsZero(task.Created) {
			return fmt.Errorf("Created not set. Task %s created time is %s. %v", task.Id, task.Created, task)
		}
		isNew[idx] = util.TimeIsZero(task.DbModified)
		prevModified[idx] = task.DbModified
		if !now.After(task.DbModified) {
			return fmt.Errorf("Task modification time is in the future: %s (current time is %s)", task.DbModified, now)
		}
		task.DbModified = now
	}
	defer func() {
		if rvErr != nil {
			for idx, task := range tasks {
				task.DbModified = prevModified[idx]
			}
		}
	}()

	// Find the previous versions of the tasks. Ensure that they weren't
	// updated concurrently.
	refs := make([]*fs.DocumentRef, 0, len(tasks))
	for _, task := range tasks {
		refs = append(refs, d.tasks().Doc(task.Id))
	}
	docs, err := tx.GetAll(refs)
	if err != nil {
		return err
	}
	for idx, doc := range docs {
		if !doc.Exists() {
			// This is expected for new tasks.
			if !isNew[idx] {
				sklog.Errorf("Task is not new but wasn't found in the DB.")
				// If the task is supposed to exist but does not, then
				// we have a problem.
				return db.ErrConcurrentUpdate
			}
		} else if isNew[idx] {
			// If the task is not supposed to exist but does, then
			// we have a problem.
			sklog.Errorf("Task has no DbModified timestamp but already exists in the DB!")
			return db.ErrConcurrentUpdate
		}
		// If the task already exists, check the DbModified timestamp
		// to ensure that someone else didn't update it.
		if !isNew[idx] {
			var old types.Task
			if err := doc.DataTo(&old); err != nil {
				return err
			}
			if old.DbModified != prevModified[idx] {
				return db.ErrConcurrentUpdate
			}
		}
	}

	// Set the new contents of the tasks.
	for _, task := range tasks {
		ref := d.tasks().Doc(task.Id)
		if err := tx.Set(ref, task); err != nil {
			return err
		}
	}
	return nil
}

// See documentation for types.TaskDB interface.
func (d *firestoreDB) PutTask(task *types.Task) error {
	return d.PutTasks([]*types.Task{task})
}

// See documentation for types.TaskDB interface.
func (d *firestoreDB) PutTasks(tasks []*types.Task) error {
	if len(tasks) > MAX_TRANSACTION_DOCS/2 {
		sklog.Warningf("Inserting %d tasks; Firestore maximum per transaction is %d", len(tasks), MAX_TRANSACTION_DOCS)
	}
	for _, task := range tasks {
		if task.Id == "" {
			if err := d.AssignId(task); err != nil {
				return err
			}
		}
		fixTaskTimestamps(task)
	}

	if err := firestore.RunTransaction(d.client, DEFAULT_ATTEMPTS, PUT_MULTI_TIMEOUT, func(ctx context.Context, tx *fs.Transaction) error {
		return d.putTasks(tasks, tx)
	}); err != nil {
		return err
	}
	for _, task := range tasks {
		d.TrackModifiedTask(task)
	}
	return nil
}
