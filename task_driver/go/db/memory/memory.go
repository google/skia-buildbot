package memory

/*
	Package memory provides an in-memory implementation of db.DB.
*/

import (
	"context"
	"sync"

	"go.opencensus.io/trace"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/db"
	"go.skia.org/infra/task_driver/go/td"
)

// memoryDB is an in-memory implementation of db.DB.
type memoryDB struct {
	backingFile string
	mtx         sync.Mutex
	taskDrivers map[string]*db.TaskDriverRun
}

// See documentation for db.DB interface.
func (d *memoryDB) Close() error {
	// Close() is a no-op for memoryDB.
	return nil
}

// Write the contents of memoryDB to disk. Assumes the caller holds d.mtx.
func (d *memoryDB) write() error {
	return util.WriteGobFile(d.backingFile, d.taskDrivers)
}

// See documentation for db.DB interface.
func (d *memoryDB) GetTaskDriver(ctx context.Context, id string) (*db.TaskDriverRun, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.taskDrivers[id], nil
}

// See documentation for db.DB interface.
func (d *memoryDB) UpdateTaskDriver(ctx context.Context, id string, msg *td.Message) error {
	ctx, span := trace.StartSpan(ctx, "memory_UpdateTaskDriver")
	defer span.End()
	d.mtx.Lock()
	defer d.mtx.Unlock()
	old := d.taskDrivers[id]
	t := old
	if t == nil {
		t = &db.TaskDriverRun{
			TaskId: id,
		}
	}
	cpy := t.Copy()
	if err := cpy.UpdateFromMessage(msg); err != nil {
		return err
	}
	d.taskDrivers[id] = cpy
	if err := d.write(); err != nil {
		// Undo the changes.
		if old == nil {
			delete(d.taskDrivers, id)
		} else {
			d.taskDrivers[id] = old
		}
		return err
	}
	return nil
}

// Return an in-memory DB instance.
func NewInMemoryDB(backingFile string) (db.DB, error) {
	data := map[string]*db.TaskDriverRun{}
	if err := util.MaybeReadGobFile(backingFile, &data); err != nil {
		return nil, err
	}
	return &memoryDB{
		backingFile: backingFile,
		taskDrivers: data,
	}, nil
}
