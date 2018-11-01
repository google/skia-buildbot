package bigtable

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/task_scheduler/go/db"
)

const (
	COLUMN_FAMILY_TASK = "TASK"
	COLUMN_TASK        = "TASK"
)

var (
	// Fully-qualified BigTable column name.
	COLUMN_TASK_FULL = fmt.Sprintf("%s:%s", COLUMN_FAMILY_TASK, COLUMN_TASK)
)

// makeRowKeyTask returns a row key for the given Task. Assumes that the Task
// has a valid commit hash and repo name.
func makeRowKeyTask(task *db.Task, uuid string) string {
	return fmt.Sprintf("%s-%s-%s", shortCommit(task.Revision), common.REPO_PROJECT_MAPPING[task.Repo], uuid)
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

// mutationForTask returns the row key and mutation for the given task.
func mutationForTask(task *db.Task, ts time.Time) (string, *bigtable.Mutation, error) {
	// Encode the Task.
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(task); err != nil {
		return "", nil, fmt.Errorf("Failed to gob-encode task: %s", err)
	}
	// Create the mutation.
	mt := bigtable.NewMutation()
	mt.Set(COLUMN_FAMILY_TASK, COLUMN_TASK, bigtable.Time(ts), buf.Bytes())
	rk := task.Id
	if rk == "" {
		return "", nil, fmt.Errorf("Task has no ID")
	}
	return rk, mt, nil
}

// PutTask inserts or updates the given task in BigTable.
func (t *tasksTable) PutTask(ctx context.Context, task *db.Task, ts time.Time) (rvErr error) {
	prevModified := task.DbModified
	task.DbModified = ts
	defer func() {
		if rvErr != nil {
			task.DbModified = prevModified
		}
	}()

	rk, mt, err := mutationForTask(task, ts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, INSERT_TIMEOUT)
	defer cancel()
	task.DbModified = ts
	return t.table.Apply(ctx, rk, mt)
}

// PutTasks inserts or updates the given tasks in BigTable.
func (t *tasksTable) PutTasks(ctx context.Context, tasks []*db.Task, ts time.Time) (rvErr error) {
	prevModified := make([]time.Time, 0, len(tasks))
	for _, task := range tasks {
		prevModified = append(prevModified, task.DbModified)
		task.DbModified = ts
	}
	defer func() {
		// TODO(borenet): It's possible that some of the tasks were
		// successfully updated. We should look at the individual errors
		// returned by ApplyBulk.
		if rvErr != nil {
			for idx, task := range tasks {
				task.DbModified = prevModified[idx]
			}
		}
	}()

	rks := make([]string, 0, len(tasks))
	mts := make([]*bigtable.Mutation, 0, len(tasks))
	for _, task := range tasks {
		rk, mt, err := mutationForTask(task, ts)
		if err != nil {
			return err
		}
		rks = append(rks, rk)
		mts = append(mts, mt)
	}
	ctx, cancel := context.WithTimeout(ctx, INSERT_TIMEOUT)
	defer cancel()
	return combineErrors(t.table.ApplyBulk(ctx, rks, mts))
}
