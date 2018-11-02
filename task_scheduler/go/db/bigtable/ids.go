package bigtable

import (
	"context"
	"encoding/binary"
	"fmt"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
)

const (
	// Row keys in the ID table.
	ID_ROW_KEY_JOBS  = "jobs"
	ID_ROW_KEY_TASKS = "tasks"

	COLUMN_FAMILY_ID = "ID"
	COLUMN_ID        = "ID"
)

// idTable is a table used for managing IDs.
type idTable struct {
	table *bigtable.Table
}

// getUUID returns N UUIDs for the given type of entity.
func (t *idTable) getUUIDs(ctx context.Context, key string, n int) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, INSERT_TIMEOUT)
	defer cancel()
	mt := bigtable.NewReadModifyWrite()
	mt.Increment(COLUMN_FAMILY_ID, COLUMN_ID, int64(n))
	row, err := t.table.ApplyReadModifyWrite(ctx, key, mt)
	if err != nil {
		return nil, err
	}
	end := binary.BigEndian.Uint64(row[COLUMN_FAMILY_ID][0].Value)
	start := end - uint64(n)
	rv := make([]string, 0, n)
	for i := start + 1; i <= end; i++ {
		rv = append(rv, fmt.Sprintf(local_db.SEQUENCE_NUMBER_FORMAT, i))
	}
	return rv, nil
}

// AssignJobId assigns an ID for the given job.
func (t *idTable) AssignJobId(ctx context.Context, job *db.Job) error {
	return t.AssignJobIds(ctx, []*db.Job{job})
}

// AssignJobIds assigns IDs for the given jobs.
func (t *idTable) AssignJobIds(ctx context.Context, jobs []*db.Job) error {
	uuids, err := t.getUUIDs(ctx, ID_ROW_KEY_JOBS, len(jobs))
	if err != nil {
		return err
	}
	for idx, job := range jobs {
		job.Id = makeRowKeyJob(job, uuids[idx])
	}
	return nil
}

// AssignTaskId assigns an ID for the given task.
func (t *idTable) AssignTaskId(ctx context.Context, task *db.Task) error {
	return t.AssignTaskIds(ctx, []*db.Task{task})
}

// AssignTaskIds assigns IDs for the given tasks.
func (t *idTable) AssignTaskIds(ctx context.Context, tasks []*db.Task) error {
	uuids, err := t.getUUIDs(ctx, ID_ROW_KEY_TASKS, len(tasks))
	if err != nil {
		return err
	}
	for idx, task := range tasks {
		task.Id = makeRowKeyTask(task, uuids[idx])
	}
	return nil
}
