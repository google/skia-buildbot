package bigtable

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/bigtable"
	multierror "github.com/hashicorp/go-multierror"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
)

const (
	COLUMN_FAMILY_TASK = "TASK"
	COLUMN_TASK        = "TASK"

	DB_MODIFIED_FORMAT = "20060102T150405_000000000Z"
)

var (
	// Fully-qualified BigTable column name.
	COLUMN_TASK_FULL = fmt.Sprintf("%s:%s", COLUMN_FAMILY_TASK, COLUMN_TASK)
)

// makeRowKeyTask returns a row key for the given Task. Assumes that the Task
// has a valid commit hash and repo name.
func makeRowKeyTask(task *db.Task, uuid string) string {
	return fmt.Sprintf("%s-%s-%s", ShortCommit(task.Revision), common.REPO_PROJECT_MAPPING[task.Repo], uuid)
}

// tasksTable interacts with the BigTable table for tasks.
type tasksTable struct {
	table *bigtable.Table
}

// GetTaskById returns the task with the given ID.
func (t *tasksTable) GetTaskById(ctx context.Context, id string) (*db.Task, error) {
	tasks, err := t.GetTasksWithPrefixes(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, db.ErrNotFound
	}
	if len(tasks) != 1 {
		return nil, fmt.Errorf("Expected exactly one task with ID %s but got %d", id, len(tasks))
	}
	return tasks[0], nil
}

// GetTasksWithPrefixes returns all tasks with row keys having the given
// prefixes.
func (t *tasksTable) GetTasksWithPrefixes(ctx context.Context, prefixes []string) ([]*db.Task, error) {
	rs := make([]bigtable.RowRange, 0, len(prefixes))
	for _, prefix := range prefixes {
		rs = append(rs, bigtable.PrefixRange(prefix))
	}
	var decodeErr error
	ctx, cancel := context.WithTimeout(ctx, QUERY_TIMEOUT)
	defer cancel()
	rv := make([]*db.Task, 0, len(prefixes))
	if err := t.table.ReadRows(ctx, bigtable.RowRangeList(rs), func(row bigtable.Row) bool {
		for _, ri := range row[COLUMN_FAMILY_TASK] {
			if ri.Column == COLUMN_TASK_FULL {
				var task *db.Task
				decodeErr = gob.NewDecoder(bytes.NewReader(ri.Value)).Decode(&task)
				if decodeErr != nil {
					return false
				}
				rv = append(rv, task)
				// We only store one task per row, so return here.
				return true
			}
		}
		return true
	}, bigtable.RowFilter(bigtable.LatestNFilter(1))); err != nil {
		return nil, fmt.Errorf("Failed to retrieve data from BigTable: %s", err)
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("Failed to gob-decode task: %s", decodeErr)
	}
	return rv, nil
}

// putTask inserts or updates the given task in BigTable.
func (t *tasksTable) putTask(ctx context.Context, task *db.Task, isNew bool) (rvErr error) {
	// Validation.
	if task.Id == "" {
		return errors.New("Task.Id is required.")
	}
	if isNew && !util.TimeIsZero(task.DbModified) {
		return errors.New("InsertTask must only be called for new tasks, but this one has a DbModified timestamp.")
	} else if !isNew && util.TimeIsZero(task.DbModified) {
		// TODO(borenet): We should error out if we have a non-new task
		// without a DbModified timestamp, but since db.DB doesn't
		// distinguish between Insert and Update, we have to assume that
		// this is actually a new task and AssignId was called outside
		// of the adapter package.
		isNew = true
	}

	// Set the modification timestamp.
	nowTs := bigtable.Now()
	prevModified := task.DbModified
	if prevModified.After(nowTs.Time()) {
		// Ensure that the modification time increases even if we update
		// faster than the timestamp resolution.
		nowTs = bigtable.Time(prevModified.Add(time.Millisecond)).TruncateToMilliseconds()
	}
	task.DbModified = nowTs.Time()
	defer func() {
		if rvErr != nil {
			task.DbModified = prevModified
		}
	}()

	// Encode the Task.
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(task); err != nil {
		return fmt.Errorf("Failed to gob-encode task: %s", err)
	}

	// Create the mutation.
	mt := bigtable.NewMutation()
	mt.Set(COLUMN_FAMILY_TASK, COLUMN_TASK, nowTs, buf.Bytes())
	mt.Set(COLUMN_FAMILY_DB_MODIFIED, COLUMN_DB_MODIFIED, nowTs, []byte(task.DbModified.Format(DB_MODIFIED_FORMAT)))
	if isNew {
		// Only insert the task if there's no existing value.
		f := bigtable.ColumnFilter(COLUMN_DB_MODIFIED)
		mt = bigtable.NewCondMutation(f, nil, mt)
	} else {
		// Only insert the task if the existing value has the expected
		// timestamp; if it doesn't, someone else modified the task.
		f := bigtable.ChainFilters(
			bigtable.ColumnFilter(COLUMN_DB_MODIFIED),
			bigtable.ValueFilter(prevModified.Format(DB_MODIFIED_FORMAT)),
		)
		mt = bigtable.NewCondMutation(f, mt, nil)
	}

	// Apply the mutation.
	ctx, cancel := context.WithTimeout(ctx, INSERT_TIMEOUT)
	defer cancel()
	var matched bool
	if err := t.table.Apply(ctx, task.Id, mt, bigtable.GetCondMutationResult(&matched)); err != nil {
		return err
	}
	// We expect no match for new tasks, but do expect one for existing tasks.
	if matched == isNew {
		return db.ErrConcurrentUpdate
	}
	return nil
}

// InsertTask inserts a new task into BigTable.
func (t *tasksTable) InsertTask(ctx context.Context, task *db.Task) error {
	return t.putTask(ctx, task, true)
}

// UpdateTask updates a task in BigTable.
func (t *tasksTable) UpdateTask(ctx context.Context, task *db.Task) error {
	return t.putTask(ctx, task, false)
}

// putTasks inserts or updates the given tasks in BigTable.
func (t *tasksTable) putTasks(ctx context.Context, tasks []*db.Task, fn func(context.Context, *db.Task) error) error {
	var mtx sync.Mutex
	var errs error
	var wg sync.WaitGroup
	for _, task := range tasks {
		go func(task *db.Task) {
			if err := fn(ctx, task); err != nil {
				mtx.Lock()
				defer mtx.Unlock()
				errs = multierror.Append(errs, err)
			}
		}(task)
	}
	wg.Wait()
	return errs
}

// InsertTasks inserts the given tasks into BigTable.
func (t *tasksTable) InsertTasks(ctx context.Context, tasks []*db.Task) error {
	return t.putTasks(ctx, tasks, t.InsertTask)
}

// UpdateTasks updates the given tasks in BigTable.
func (t *tasksTable) UpdateTasks(ctx context.Context, tasks []*db.Task) error {
	return t.putTasks(ctx, tasks, t.UpdateTask)
}
