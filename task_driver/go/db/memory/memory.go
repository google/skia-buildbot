package memory

/*
	Package memory provides an in-memory implementation of db.DB.
*/

import (
	"encoding/gob"
	"sync"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/db"
)

// memoryDB is an in-memory implementation of db.DB.
type memoryDB struct {
	backingFile string
	mtx         sync.Mutex
	taskDrivers map[string]*db.TaskDriverRun
}

// Write the contents of memoryDB to disk. Assumes the caller holds d.mtx.
func (d *memoryDB) write() error {
	return util.WriteGobFile(d.backingFile, d.taskDrivers)
}

// See documentation for db.DB interface.
func (d *memoryDB) InsertTaskDriver(t *db.TaskDriverRun) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	d.taskDrivers[t.TaskId] = t
	return d.write()
}

// See documentation for db.DB interface.
func (d *memoryDB) GetTaskDriver(id string) (*db.TaskDriverRun, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.taskDrivers[id], nil
}

// See documentation for db.DB interface.
func (d *memoryDB) UpdateTaskDriver(id string, fn func(*db.TaskDriverRun) error) error {
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
	if err := fn(cpy); err != nil {
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
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
	data := map[string]*db.TaskDriverRun{}
	if err := util.MaybeReadGobFile(backingFile, &data); err != nil {
		return nil, err
	}
	return &memoryDB{
		backingFile: backingFile,
		taskDrivers: data,
	}, nil
}
