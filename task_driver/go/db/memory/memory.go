package memory

/*
	Package memory provides an in-memory implementation of db.DB.
*/

import (
	"sync"

	"go.skia.org/infra/task_driver/go/db"
)

// memoryDB is an in-memory implementation of db.DB.
type memoryDB struct {
	mtx         sync.Mutex
	taskDrivers map[string]*db.TaskDriverRun
}

// See documentation for db.DB interface.
func (d *memoryDB) InsertTaskDriver(t *db.TaskDriverRun) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	d.taskDrivers[t.TaskId] = t
	return nil
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
	t := d.taskDrivers[id]
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
	return nil
}

// Return an in-memory DB instance.
func NewInMemoryDB() db.DB {
	return &memoryDB{
		taskDrivers: map[string]*db.TaskDriverRun{},
	}
}
