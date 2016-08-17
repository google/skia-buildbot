package db

import (
	"sort"
	"sync"
	"time"

	"github.com/satori/go.uuid"
)

// ModifiedTasks allows subscribers to keep track of Tasks that have been
// modified. It implements StartTrackingModifiedTasks and GetModifiedTasks from
// the DB interface.
type ModifiedTasks struct {
	// map[subscriber_id][Task.Id]*Task
	tasks map[string]map[string]*Task
	// After the expiration time, subscribers are automatically removed.
	expiration map[string]time.Time
	// Protects tasks and expiration.
	mtx sync.RWMutex
}

// See docs for DB interface.
func (m *ModifiedTasks) GetModifiedTasks(id string) ([]*Task, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	modifiedTasks, ok := m.tasks[id]
	if !ok {
		return nil, ErrUnknownId
	}
	rv := make([]*Task, 0, len(modifiedTasks))
	for _, t := range modifiedTasks {
		rv = append(rv, t.Copy())
	}
	m.expiration[id] = time.Now().Add(MODIFIED_BUILDS_TIMEOUT)
	m.tasks[id] = map[string]*Task{}
	sort.Sort(TaskSlice(rv))
	return rv, nil
}

// clearExpiredSubscribers periodically deletes data about any subscribers that
// haven't been seen within MODIFIED_BUILDS_TIMEOUT. Must be called as a
// goroutine. Returns when there are no remaining subscribers.
func (m *ModifiedTasks) clearExpiredSubscribers() {
	for _ = range time.Tick(time.Minute) {
		m.mtx.Lock()
		for id, t := range m.expiration {
			if time.Now().After(t) {
				delete(m.tasks, id)
				delete(m.expiration, id)
			}
		}
		anyLeft := len(m.expiration) > 0
		m.mtx.Unlock()
		if !anyLeft {
			break
		}
	}
}

// TrackModifiedTask indicates the given Task should be returned from the next
// call to GetModifiedTasks from each subscriber.
func (m *ModifiedTasks) TrackModifiedTask(t *Task) {
	m.TrackModifiedTasks([]*Task{t})
}

// TrackModifiedTasks calls TrackModifiedTask on each item.
func (m *ModifiedTasks) TrackModifiedTasks(tasks []*Task) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	for _, t := range tasks {
		// Make a single copy, since GetModifiedTasks also copies.
		t = t.Copy()
		for _, modTasks := range m.tasks {
			modTasks[t.Id] = t
		}
	}
}

// See docs for DB interface.
func (m *ModifiedTasks) StartTrackingModifiedTasks() (string, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if len(m.tasks) == 0 {
		// Initialize the data structure and start expiration goroutine.
		m.tasks = map[string]map[string]*Task{}
		m.expiration = map[string]time.Time{}
		go m.clearExpiredSubscribers()
	} else if len(m.tasks) >= MAX_MODIFIED_BUILDS_USERS {
		return "", ErrTooManyUsers
	}
	id := uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()
	m.tasks[id] = map[string]*Task{}
	m.expiration[id] = time.Now().Add(MODIFIED_BUILDS_TIMEOUT)
	return id, nil
}
