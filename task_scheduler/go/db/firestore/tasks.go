package firestore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/pborman/uuid"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	COLLECTION_TASKS = "tasks"
)

// Fix all timestamps for the given task.
func fixTaskTimestamps(task *db.Task) {
	task.Created = fixTimestamp(task.Created)
	task.DbModified = fixTimestamp(task.DbModified)
	task.Finished = fixTimestamp(task.Finished)
	task.Started = fixTimestamp(task.Started)
}

// tasks returns a reference to the tasks collection.
func (d *firestoreDB) tasks() *firestore.CollectionRef {
	return d.collection(COLLECTION_TASKS)
}

// See documentation for db.TaskReader interface.
func (d *firestoreDB) GetTaskById(id string) (*db.Task, error) {
	doc, err := d.tasks().Doc(id).Get(context.Background())
	if grpc.Code(err) == codes.NotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var rv db.Task
	if err := doc.DataTo(&rv); err != nil {
		return nil, err
	}
	return &rv, nil
}

// See documentation for db.TaskReader interface.
func (d *firestoreDB) GetTasksFromDateRange(start, end time.Time, repo string) ([]*db.Task, error) {
	// Adjust start and end times for Firestore resolution.
	start = fixTimestamp(start)
	end = fixTimestamp(end)
	// Adjust for time skew.
	min := start.Add(-local_db.MAX_CREATED_TIME_SKEW)
	max := end.Add(local_db.MAX_CREATED_TIME_SKEW)
	q := d.tasks().Where("Created", ">=", min).Where("Created", "<", max).OrderBy("Created", firestore.Asc)
	iter := q.Documents(context.Background())
	defer iter.Stop()
	rv := []*db.Task{}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		}
		var task db.Task
		if err := doc.DataTo(&task); err != nil {
			return nil, err
		}
		if !task.Created.Before(start) && end.After(task.Created) {
			// TODO(borenet): We can make this part of the query,
			// but it would required building a composite index.
			// It's possible that we should require a repo when
			// searching by timestamp; indexing a timestamp causes
			// the whole collection to cap out at a maximum of
			// 500 writes per second.
			if repo == "" || task.Repo == repo {
				rv = append(rv, &task)
			}
		}
	}
	return rv, nil
}

// See documentation for db.TaskDB interface.
func (d *firestoreDB) AssignId(task *db.Task) error {
	// TODO(borenet): Use firestore-assigned IDs.
	task.Id = uuid.New()
	return nil
}

// putTasks sets the contents of the given tasks in Firestore, as part of the
// given transaction. It is used by PutTask and PutTasks.
func (d *firestoreDB) putTasks(tasks []*db.Task, tx *firestore.Transaction) (rvErr error) {
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
	for idx, task := range tasks {
		ref := d.tasks().Doc(task.Id)
		doc, err := tx.Get(ref)
		if grpc.Code(err) == codes.NotFound {
			// This is expected for new tasks.
			if !isNew[idx] {
				sklog.Errorf("Task is not new but wasn't found in the DB.")
				// If the task is supposed to exist but does not, then
				// we have a problem.
				return db.ErrConcurrentUpdate
			}
		} else if err != nil {
			sklog.Errorf("isNew: %v", isNew)
			sklog.Errorf("Got error: %s", err)
			if grpc.Code(err) == codes.NotFound {
				sklog.Errorf("not found")
			}
			return err
		} else if isNew[idx] {
			// If the task is not supposed to exist but does, then
			// we have a problem.
			return db.ErrConcurrentUpdate
		}
		// If the task already exists, check the DbModified timestamp
		// to ensure that someone else didn't update it.
		if !isNew[idx] {
			var old db.Task
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

// See documentation for db.TaskDB interface.
func (d *firestoreDB) PutTask(task *db.Task) error {
	return d.PutTasks([]*db.Task{task})
}

// See documentation for db.TaskDB interface.
func (d *firestoreDB) PutTasks(tasks []*db.Task) error {
	for _, task := range tasks {
		if task.Id == "" {
			if err := d.AssignId(task); err != nil {
				return err
			}
		}
		fixTaskTimestamps(task)
	}

	ctx := context.Background()
	if err := d.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		return d.putTasks(tasks, tx)
	}); err != nil {
		return err
	}
	for _, task := range tasks {
		d.TrackModifiedTask(task)
	}
	return nil
}
