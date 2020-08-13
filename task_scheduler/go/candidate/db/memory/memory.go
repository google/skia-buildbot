package memory

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/task_scheduler/go/candidate"
	"go.skia.org/infra/task_scheduler/go/candidate/db"
)

func NewInMemoryTaskCandidateDB() db.TaskCandidateDB {
	return &memoryDB{
		candidates: map[string]*candidate.TaskCandidate{},
		active:     map[string]*candidate.TaskCandidate{},
		byJob:      map[string]map[string]*candidate.TaskCandidate{},
	}
}

type memoryDB struct {
	mtx        sync.RWMutex
	candidates map[string]*candidate.TaskCandidate
	active     map[string]*candidate.TaskCandidate
	byJob      map[string]map[string]*candidate.TaskCandidate
}

func (m *memoryDB) GetActive(ctx context.Context) ([]*candidate.TaskCandidate, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	rv := make([]*candidate.TaskCandidate, 0, len(m.active))
	for _, c := range m.active {
		rv = append(rv, c)
	}
	return rv, nil
}

func (m *memoryDB) GetCandidatesForJobs(ctx context.Context, jobs []string) (map[string][]*candidate.TaskCandidate, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	rv := make(map[string][]*candidate.TaskCandidate, len(jobs))
	for _, jobId := range jobs {
		cs := make([]*candidate.TaskCandidate, 0, len(m.byJob[jobId]))
		for _, c := range m.byJob[jobId] {
			cs = append(cs, c)
		}
		rv[jobId] = cs
	}
	return rv, nil
}

func (m *memoryDB) Put(ctx context.Context, cs []*candidate.TaskCandidate) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	for _, c := range cs {
		m.candidates[c.Id] = c
		if c.Active() {
			m.active[c.Id] = c
		} else {
			delete(m.active, c.Id)
		}
		for _, job := range c.Jobs {
			subMap, ok := m.byJob[job.Id]
			if !ok {
				subMap = map[string]*candidate.TaskCandidate{}
				m.byJob[job.Id] = subMap
			}
			subMap[c.Id] = c
		}
	}
	return nil
}

func (m *memoryDB) GetRange(ctx context.Context, start, end time.Time) ([]*candidate.TaskCandidate, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	rv := []*candidate.TaskCandidate{}
	for _, c := range m.candidates {
		if c.Timestamp.Before(start) || !end.After(c.Timestamp) {
			continue
		}
		rv = append(rv, c)
	}
	return rv, nil
}
