package db

import (
	"bytes"
	"encoding/gob"
	"sort"
	"sync"
	"time"

	"github.com/satori/go.uuid"
	"go.skia.org/infra/go/sklog"
)

// modifiedData allows subscribers to keep track of DB entries that have been
// modified. It is designed to be used with wrappers in order to store a desired
// type of data.
type modifiedData struct {
	// map[subscriber_id][entry_id]gob
	data map[string]map[string][]byte
	// After the expiration time, subscribers are automatically removed.
	expiration map[string]time.Time
	// Protects data and expiration.
	mtx sync.RWMutex
}

func (m *modifiedData) GetModifiedEntries(id string) (map[string][]byte, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if _, ok := m.expiration[id]; !ok {
		return nil, ErrUnknownId
	}
	rv := m.data[id]
	m.expiration[id] = time.Now().Add(MODIFIED_DATA_TIMEOUT)
	delete(m.data, id)
	return rv, nil
}

// clearExpiredSubscribers periodically deletes data about any subscribers that
// haven't been seen within MODIFIED_TASKS_TIMEOUT. Must be called as a
// goroutine. Returns when there are no remaining subscribers.
func (m *modifiedData) clearExpiredSubscribers() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		m.mtx.Lock()
		for id, t := range m.expiration {
			if time.Now().After(t) {
				sklog.Warningf("Deleting expired subscriber with id %s; expiration time %s.", id, t)
				delete(m.data, id)
				delete(m.expiration, id)
			}
		}
		anyLeft := len(m.expiration) > 0
		if !anyLeft {
			m.data = nil
			m.expiration = nil
		}
		m.mtx.Unlock()
		if !anyLeft {
			break
		}
	}
	ticker.Stop()
}

// TrackModifiedEntry indicates the given data should be returned from the next
// call to GetModifiedEntries from each subscriber.
func (m *modifiedData) TrackModifiedEntry(id string, d []byte) {
	m.TrackModifiedEntries(map[string][]byte{id: d})
}

// TrackModifiedEntries is a batch version of TrackModifiedEntry. Values of
// gobs must not be modified after this call.
func (m *modifiedData) TrackModifiedEntries(gobs map[string][]byte) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	for subId := range m.expiration {
		sub, ok := m.data[subId]
		if !ok {
			sub = make(map[string][]byte, len(gobs))
			m.data[subId] = sub
		}
		for entryId, gob := range gobs {
			sub[entryId] = gob
		}
	}
}

// See docs for TaskReader.StartTrackingModifiedTasks or
// JobReader.StartTrackingModifiedJobs.
func (m *modifiedData) StartTrackingModifiedEntries() (string, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if m.expiration == nil {
		// Initialize the data structure and start expiration goroutine.
		m.data = map[string]map[string][]byte{}
		m.expiration = map[string]time.Time{}
		go m.clearExpiredSubscribers()
	} else if len(m.expiration) >= MAX_MODIFIED_DATA_USERS {
		return "", ErrTooManyUsers
	}
	id := uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()
	m.expiration[id] = time.Now().Add(MODIFIED_DATA_TIMEOUT)
	return id, nil
}

// See docs for TaskReader.StopTrackingModifiedTasks or
// JobReader.StopTrackingModifiedJobs.
func (m *modifiedData) StopTrackingModifiedEntries(id string) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	delete(m.data, id)
	delete(m.expiration, id)
}

type ModifiedTasks struct {
	m modifiedData
}

// See docs for TaskReader interface.
func (m *ModifiedTasks) GetModifiedTasks(id string) ([]*Task, error) {
	tasks, err := m.m.GetModifiedEntries(id)
	if err != nil {
		return nil, err
	}
	d := TaskDecoder{}
	for _, g := range tasks {
		if !d.Process(g) {
			break
		}
	}
	rv, err := d.Result()
	if err != nil {
		return nil, err
	}
	sort.Sort(TaskSlice(rv))
	return rv, nil
}

// See docs for TaskReader interface.
func (m *ModifiedTasks) GetModifiedTasksGOB(id string) (map[string][]byte, error) {
	return m.m.GetModifiedEntries(id)
}

// TrackModifiedTask indicates the given Task should be returned from the next
// call to GetModifiedTasks from each subscriber.
func (m *ModifiedTasks) TrackModifiedTask(t *Task) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(t); err != nil {
		sklog.Fatal(err)
	}
	m.m.TrackModifiedEntries(map[string][]byte{t.Id: buf.Bytes()})
}

// TrackModifiedTasksGOB is a batch, GOB version of TrackModifiedTask. Given a
// map from Task.Id to GOB-encoded task, it is equivalent to GOB-decoding each
// value of gobs as a Task and calling TrackModifiedTask on each one. Values of
// gobs must not be modified after this call.
func (m *ModifiedTasks) TrackModifiedTasksGOB(gobs map[string][]byte) {
	m.m.TrackModifiedEntries(gobs)
}

// See docs for TaskReader interface.
func (m *ModifiedTasks) StartTrackingModifiedTasks() (string, error) {
	return m.m.StartTrackingModifiedEntries()
}

// See docs for TaskReader interface.
func (m *ModifiedTasks) StopTrackingModifiedTasks(id string) {
	m.m.StopTrackingModifiedEntries(id)
}

type ModifiedJobs struct {
	m modifiedData
}

// See docs for JobReader interface.
func (m *ModifiedJobs) GetModifiedJobs(id string) ([]*Job, error) {
	jobs, err := m.m.GetModifiedEntries(id)
	if err != nil {
		return nil, err
	}
	d := JobDecoder{}
	for _, g := range jobs {
		if !d.Process(g) {
			break
		}
	}
	rv, err := d.Result()
	if err != nil {
		return nil, err
	}
	sort.Sort(JobSlice(rv))
	return rv, nil
}

// See docs for JobReader interface.
func (m *ModifiedJobs) GetModifiedJobsGOB(id string) (map[string][]byte, error) {
	return m.m.GetModifiedEntries(id)
}

// TrackModifiedJob indicates the given Job should be returned from the next
// call to GetModifiedJobs from each subscriber.
func (m *ModifiedJobs) TrackModifiedJob(j *Job) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(j); err != nil {
		sklog.Fatal(err)
	}
	m.m.TrackModifiedEntries(map[string][]byte{j.Id: buf.Bytes()})
}

// TrackModifiedJobsGOB is a batch, GOB version of TrackModifiedJob. Given a
// map from Job.Id to GOB-encoded task, it is equivalent to GOB-decoding each
// value of gobs as a Job and calling TrackModifiedJob on each one. Values of
// gobs must not be modified after this call.
func (m *ModifiedJobs) TrackModifiedJobsGOB(gobs map[string][]byte) {
	m.m.TrackModifiedEntries(gobs)
}

// See docs for JobReader interface.
func (m *ModifiedJobs) StartTrackingModifiedJobs() (string, error) {
	return m.m.StartTrackingModifiedEntries()
}

// See docs for JobReader interface.
func (m *ModifiedJobs) StopTrackingModifiedJobs(id string) {
	m.m.StopTrackingModifiedEntries(id)
}
