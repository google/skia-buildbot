package db

import (
	"bytes"
	"encoding/gob"
	"sort"
	"sync"
	"time"

	"github.com/satori/go.uuid"
	"github.com/skia-dev/glog"
)

// ModifiedTasks allows subscribers to keep track of Tasks that have been
// modified. It implements StartTrackingModifiedTasks and GetModifiedTasks from
// the DB interface.
type ModifiedTasks struct {
	// map[subscriber_id][task_id]task_gob
	tasks map[string]map[string][]byte
	// After the expiration time, subscribers are automatically removed.
	expiration map[string]time.Time
	// Protects tasks and expiration.
	mtx sync.RWMutex
}

// See docs for DB interface.
func (m *ModifiedTasks) GetModifiedTasks(id string) ([]*Task, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if _, ok := m.expiration[id]; !ok {
		return nil, ErrUnknownId
	}
	d := TaskDecoder{}
	for _, g := range m.tasks[id] {
		if !d.Process(g) {
			break
		}
	}
	rv, err := d.Result()
	if err != nil {
		return nil, err
	}
	m.expiration[id] = time.Now().Add(MODIFIED_TASKS_TIMEOUT)
	delete(m.tasks, id)
	sort.Sort(TaskSlice(rv))
	return rv, nil
}

// clearExpiredSubscribers periodically deletes data about any subscribers that
// haven't been seen within MODIFIED_TASKS_TIMEOUT. Must be called as a
// goroutine. Returns when there are no remaining subscribers.
func (m *ModifiedTasks) clearExpiredSubscribers() {
	ticker := time.NewTicker(time.Minute)
	for _ = range ticker.C {
		m.mtx.Lock()
		for id, t := range m.expiration {
			if time.Now().After(t) {
				delete(m.tasks, id)
				delete(m.expiration, id)
			}
		}
		anyLeft := len(m.expiration) > 0
		if !anyLeft {
			m.tasks = nil
			m.expiration = nil
		}
		m.mtx.Unlock()
		if !anyLeft {
			break
		}
	}
	ticker.Stop()
}

// TrackModifiedTask indicates the given Task should be returned from the next
// call to GetModifiedTasks from each subscriber.
func (m *ModifiedTasks) TrackModifiedTask(t *Task) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(t); err != nil {
		glog.Fatal(err)
	}
	m.TrackModifiedTasksGOB(map[string][]byte{t.Id: buf.Bytes()})
}

// TrackModifiedTasksGOB is a batch, GOB version of TrackModifiedTask. Given a
// map from Task.Id to GOB-encoded task, it is equivalent to GOB-decoding each
// value of gobs as a Task and calling TrackModifiedTask on each one. Values of
// gobs must not be modified after this call.
func (m *ModifiedTasks) TrackModifiedTasksGOB(gobs map[string][]byte) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	for subId, _ := range m.expiration {
		sub, ok := m.tasks[subId]
		if !ok {
			sub = make(map[string][]byte, len(gobs))
			m.tasks[subId] = sub
		}
		for taskId, gob := range gobs {
			sub[taskId] = gob
		}
	}
}

// See docs for DB interface.
func (m *ModifiedTasks) StartTrackingModifiedTasks() (string, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if m.expiration == nil {
		// Initialize the data structure and start expiration goroutine.
		m.tasks = map[string]map[string][]byte{}
		m.expiration = map[string]time.Time{}
		go m.clearExpiredSubscribers()
	} else if len(m.expiration) >= MAX_MODIFIED_TASKS_USERS {
		return "", ErrTooManyUsers
	}
	id := uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()
	m.expiration[id] = time.Now().Add(MODIFIED_TASKS_TIMEOUT)
	return id, nil
}

// See docs for DB interface.
func (m *ModifiedTasks) StopTrackingModifiedTasks(id string) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	delete(m.tasks, id)
	delete(m.expiration, id)
}
