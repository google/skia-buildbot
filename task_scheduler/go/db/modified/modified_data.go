package modified

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
)

// NewModifiedData returns a db.ModifiedData instance which tracks modifications
// in memory.
func NewModifiedData() db.ModifiedData {
	return db.NewModifiedData(&ModifiedTasksImpl{}, &ModifiedJobsImpl{}, &ModifiedCommentsImpl{})
}

// modifiedData allows subscribers to keep track of DB entries that have been
// modified. It is designed to be used with wrappers in order to store a desired
// type of data.
type modifiedData struct {
	// map[subscriber_id][entry_id]entry
	data map[string]map[string]interface{}
	// After the expiration time, subscribers are automatically removed.
	expiration map[string]time.Time
	// Protects data and expiration.
	mtx sync.RWMutex
}

func (m *modifiedData) GetModifiedEntries(id string) (map[string]interface{}, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if _, ok := m.expiration[id]; !ok {
		return nil, db.ErrUnknownId
	}
	rv := m.data[id]
	m.expiration[id] = time.Now().Add(db.MODIFIED_DATA_TIMEOUT)
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
func (m *modifiedData) TrackModifiedEntry(id string, d interface{}) {
	m.TrackModifiedEntries(map[string]interface{}{id: d})
}

// TrackModifiedEntries indicates that the given data should be returned from
// the next call to GetModifiedEntries from each subscriber. Values must not be
// modified after this call.
func (m *modifiedData) TrackModifiedEntries(entries map[string]interface{}) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	for subId := range m.expiration {
		sub, ok := m.data[subId]
		if !ok {
			sub = make(map[string]interface{}, len(entries))
			m.data[subId] = sub
		}
		for entryId, entry := range entries {
			sub[entryId] = entry
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
		m.data = map[string]map[string]interface{}{}
		m.expiration = map[string]time.Time{}
		go m.clearExpiredSubscribers()
	} else if len(m.expiration) >= db.MAX_MODIFIED_DATA_USERS {
		return "", db.ErrTooManyUsers
	}
	id := uuid.New().String()
	m.expiration[id] = time.Now().Add(db.MODIFIED_DATA_TIMEOUT)
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

// ModifiedTasksImpl is an implementation of the ModifiedTasks interface.
type ModifiedTasksImpl struct {
	m modifiedData
}

// See docs for ModifiedTasks interface.
func (m *ModifiedTasksImpl) GetModifiedTasks(id string) ([]*types.Task, error) {
	tasks, err := m.m.GetModifiedEntries(id)
	if err != nil {
		return nil, err
	}
	rv := make([]*types.Task, 0, len(tasks))
	for _, t := range tasks {
		rv = append(rv, t.(*types.Task).Copy())
	}
	sort.Sort(types.TaskSlice(rv))
	return rv, nil
}

// See docs for ModifiedTasks interface.
func (m *ModifiedTasksImpl) TrackModifiedTask(t *types.Task) {
	m.TrackModifiedTasks([]*types.Task{t})
}

// See docs for ModifiedTasks interface.
func (m *ModifiedTasksImpl) TrackModifiedTasks(tasks []*types.Task) {
	entries := make(map[string]interface{}, len(tasks))
	for _, t := range tasks {
		entries[t.Id] = t.Copy()
	}
	m.m.TrackModifiedEntries(entries)
}

// See docs for ModifiedTasks interface.
func (m *ModifiedTasksImpl) StartTrackingModifiedTasks() (string, error) {
	return m.m.StartTrackingModifiedEntries()
}

// See docs for ModifiedTasks interface.
func (m *ModifiedTasksImpl) StopTrackingModifiedTasks(id string) {
	m.m.StopTrackingModifiedEntries(id)
}

type ModifiedJobsImpl struct {
	m modifiedData
}

// See docs for ModifiedJobs interface.
func (m *ModifiedJobsImpl) GetModifiedJobs(id string) ([]*types.Job, error) {
	jobs, err := m.m.GetModifiedEntries(id)
	if err != nil {
		return nil, err
	}
	rv := make([]*types.Job, 0, len(jobs))
	for _, j := range jobs {
		rv = append(rv, j.(*types.Job).Copy())
	}
	sort.Sort(types.JobSlice(rv))
	return rv, nil
}

// See docs for ModifiedJobs interface.
func (m *ModifiedJobsImpl) TrackModifiedJob(j *types.Job) {
	m.TrackModifiedJobs([]*types.Job{j})
}

// See docs for ModifiedJobs interface.
func (m *ModifiedJobsImpl) TrackModifiedJobs(jobs []*types.Job) {
	entries := make(map[string]interface{}, len(jobs))
	for _, j := range jobs {
		entries[j.Id] = j.Copy()
	}
	m.m.TrackModifiedEntries(entries)
}

// See docs for ModifiedJobs interface.
func (m *ModifiedJobsImpl) StartTrackingModifiedJobs() (string, error) {
	return m.m.StartTrackingModifiedEntries()
}

// See docs for ModifiedJobs interface.
func (m *ModifiedJobsImpl) StopTrackingModifiedJobs(id string) {
	m.m.StopTrackingModifiedEntries(id)
}

type ModifiedCommentsImpl struct {
	tasks     modifiedData
	taskSpecs modifiedData
	commits   modifiedData
}

// See docs for ModifiedComments interface.
func (m *ModifiedCommentsImpl) GetModifiedComments(id string) ([]*types.TaskComment, []*types.TaskSpecComment, []*types.CommitComment, error) {
	ids := strings.Split(id, "#")
	if len(ids) != 3 {
		return nil, nil, nil, db.ErrUnknownId
	}
	tasks, err := m.tasks.GetModifiedEntries(ids[0])
	if err != nil {
		return nil, nil, nil, err
	}
	rv1 := make([]*types.TaskComment, 0, len(tasks))
	for _, c := range tasks {
		rv1 = append(rv1, c.(*types.TaskComment).Copy())
	}
	sort.Sort(types.TaskCommentSlice(rv1))

	taskSpecs, err := m.taskSpecs.GetModifiedEntries(ids[1])
	if err != nil {
		return nil, nil, nil, err
	}
	rv2 := make([]*types.TaskSpecComment, 0, len(taskSpecs))
	for _, c := range taskSpecs {
		rv2 = append(rv2, c.(*types.TaskSpecComment).Copy())
	}
	sort.Sort(types.TaskSpecCommentSlice(rv2))

	commits, err := m.commits.GetModifiedEntries(ids[2])
	if err != nil {
		return nil, nil, nil, err
	}
	rv3 := make([]*types.CommitComment, 0, len(commits))
	for _, c := range commits {
		rv3 = append(rv3, c.(*types.CommitComment).Copy())
	}
	sort.Sort(types.CommitCommentSlice(rv3))

	return rv1, rv2, rv3, nil
}

// See docs for ModifiedComments interface.
func (m *ModifiedCommentsImpl) TrackModifiedTaskComment(c *types.TaskComment) {
	m.tasks.TrackModifiedEntries(map[string]interface{}{c.Id(): c.Copy()})
}

// See docs for ModifiedComments interface.
func (m *ModifiedCommentsImpl) TrackModifiedTaskSpecComment(c *types.TaskSpecComment) {
	m.taskSpecs.TrackModifiedEntries(map[string]interface{}{c.Id(): c.Copy()})
}

// See docs for ModifiedComments interface.
func (m *ModifiedCommentsImpl) TrackModifiedCommitComment(c *types.CommitComment) {
	m.commits.TrackModifiedEntries(map[string]interface{}{c.Id(): c.Copy()})
}

// See docs for ModifiedComments interface.
func (m *ModifiedCommentsImpl) StartTrackingModifiedComments() (string, error) {
	id1, err := m.tasks.StartTrackingModifiedEntries()
	if err != nil {
		return "", err
	}
	id2, err := m.taskSpecs.StartTrackingModifiedEntries()
	if err != nil {
		m.tasks.StopTrackingModifiedEntries(id1)
		return "", err
	}
	id3, err := m.commits.StartTrackingModifiedEntries()
	if err != nil {
		m.tasks.StopTrackingModifiedEntries(id1)
		m.taskSpecs.StopTrackingModifiedEntries(id2)
		return "", err
	}
	return id1 + "#" + id2 + "#" + id3, nil
}

// See docs for ModifiedComments interface.
func (m *ModifiedCommentsImpl) StopTrackingModifiedComments(id string) {
	ids := strings.Split(id, "#")
	if len(ids) != 3 {
		sklog.Errorf("Invalid id %q", id)
		return
	}
	m.tasks.StopTrackingModifiedEntries(ids[0])
	m.taskSpecs.StopTrackingModifiedEntries(ids[1])
	m.commits.StopTrackingModifiedEntries(ids[2])
}

var _ db.ModifiedTasks = &ModifiedTasksImpl{}
var _ db.ModifiedJobs = &ModifiedJobsImpl{}
var _ db.ModifiedComments = &ModifiedCommentsImpl{}
