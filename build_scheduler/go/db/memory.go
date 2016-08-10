package db

import (
	"sort"
	"sync"
	"time"

	"github.com/satori/go.uuid"
	"github.com/skia-dev/glog"
)

type TaskSlice []*Task

func (s TaskSlice) Len() int { return len(s) }

func (s TaskSlice) Less(i, j int) bool {
	ts1, err := s[i].Created()
	if err != nil {
		glog.Errorf("Failed to parse CreatedTimestamp: %v", s[i])
	}
	ts2, err := s[j].Created()
	if err != nil {
		glog.Errorf("Failed to parse CreatedTimestamp: %v", s[j])
	}
	return ts1.Before(ts2)
}

func (s TaskSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type inMemoryDB struct {
	tasks     map[string]*Task
	tasksMtx  sync.RWMutex
	modTasks  map[string]map[string]*Task
	modExpire map[string]time.Time
	modMtx    sync.RWMutex
}

// See docs for DB interface.
func (db *inMemoryDB) Close() error {
	return nil
}

// See docs for DB interface.
func (db *inMemoryDB) GetTasksFromDateRange(start, end time.Time) ([]*Task, error) {
	db.tasksMtx.RLock()
	defer db.tasksMtx.RUnlock()

	rv := []*Task{}
	// TODO(borenet): Binary search.
	for _, b := range db.tasks {
		created, err := b.Created()
		if err != nil {
			return nil, err
		}
		if (created.Equal(start) || created.After(start)) && created.Before(end) {
			rv = append(rv, b.Copy())
		}
	}
	sort.Sort(TaskSlice(rv))
	return rv, nil
}

// See docs for DB interface.
func (db *inMemoryDB) GetModifiedTasks(id string) ([]*Task, error) {
	db.modMtx.Lock()
	defer db.modMtx.Unlock()
	modifiedTasks, ok := db.modTasks[id]
	if !ok {
		return nil, ErrUnknownId
	}
	rv := make([]*Task, 0, len(modifiedTasks))
	for _, b := range modifiedTasks {
		rv = append(rv, b.Copy())
	}
	db.modExpire[id] = time.Now().Add(MODIFIED_BUILDS_TIMEOUT)
	db.modTasks[id] = map[string]*Task{}
	sort.Sort(TaskSlice(rv))
	return rv, nil
}

func (db *inMemoryDB) clearExpiredModifiedUsers() {
	db.modMtx.Lock()
	defer db.modMtx.Unlock()
	for id, t := range db.modExpire {
		if time.Now().After(t) {
			delete(db.modTasks, id)
			delete(db.modExpire, id)
		}
	}
}

func (db *inMemoryDB) modify(b *Task) {
	db.modMtx.Lock()
	defer db.modMtx.Unlock()
	for _, modTasks := range db.modTasks {
		modTasks[b.Id] = b.Copy()
	}
}

// See docs for DB interface.
func (db *inMemoryDB) PutTask(task *Task) error {
	db.tasksMtx.Lock()
	defer db.tasksMtx.Unlock()

	// TODO(borenet): Keep tasks in a sorted slice.
	db.tasks[task.Id] = task
	db.modify(task)
	return nil
}

// See docs for DB interface.
func (db *inMemoryDB) PutTasks(tasks []*Task) error {
	for _, t := range tasks {
		if err := db.PutTask(t); err != nil {
			return err
		}
	}
	return nil
}

// See docs for DB interface.
func (db *inMemoryDB) StartTrackingModifiedTasks() (string, error) {
	db.modMtx.Lock()
	defer db.modMtx.Unlock()
	if len(db.modTasks) >= MAX_MODIFIED_BUILDS_USERS {
		return "", ErrTooManyUsers
	}
	id := uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()
	db.modTasks[id] = map[string]*Task{}
	db.modExpire[id] = time.Now().Add(MODIFIED_BUILDS_TIMEOUT)
	return id, nil
}

// NewInMemoryDB returns an extremely simple, inefficient, in-memory DB implementation.
func NewInMemoryDB() DB {
	db := &inMemoryDB{
		tasks:     map[string]*Task{},
		modTasks:  map[string]map[string]*Task{},
		modExpire: map[string]time.Time{},
	}
	go func() {
		for _ = range time.Tick(time.Minute) {
			db.clearExpiredModifiedUsers()
		}
	}()
	return db
}
