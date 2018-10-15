package db

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/pborman/uuid"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

type inMemoryTaskDB struct {
	tasks    map[string]*Task
	tasksMtx sync.RWMutex
	ModifiedTasks
}

// See docs for TaskDB interface. Does not take any locks.
func (db *inMemoryTaskDB) AssignId(t *Task) error {
	if t.Id != "" {
		return fmt.Errorf("Task Id already assigned: %v", t.Id)
	}
	t.Id = uuid.New()
	return nil
}

// See docs for TaskDB interface.
func (db *inMemoryTaskDB) GetTaskById(id string) (*Task, error) {
	db.tasksMtx.RLock()
	defer db.tasksMtx.RUnlock()
	if task := db.tasks[id]; task != nil {
		return task.Copy(), nil
	}
	return nil, nil
}

// See docs for TaskDB interface.
func (db *inMemoryTaskDB) GetTasksFromDateRange(start, end time.Time, repo string) ([]*Task, error) {
	db.tasksMtx.RLock()
	defer db.tasksMtx.RUnlock()

	rv := []*Task{}
	// TODO(borenet): Binary search.
	for _, b := range db.tasks {
		if (b.Created.Equal(start) || b.Created.After(start)) && b.Created.Before(end) {
			if repo == "" || b.Repo == repo {
				rv = append(rv, b.Copy())
			}
		}
	}
	sort.Sort(TaskSlice(rv))
	return rv, nil
}

// See docs for TaskDB interface.
func (db *inMemoryTaskDB) PutTask(task *Task) error {
	return db.PutTasks([]*Task{task})
}

// See docs for TaskDB interface.
func (db *inMemoryTaskDB) PutTasks(tasks []*Task) error {
	db.tasksMtx.Lock()
	defer db.tasksMtx.Unlock()

	// Validate.
	for _, task := range tasks {
		if util.TimeIsZero(task.Created) {
			return fmt.Errorf("Created not set. Task %s created time is %s. %v", task.Id, task.Created, task)
		}

		if existing := db.tasks[task.Id]; existing != nil {
			if !existing.DbModified.Equal(task.DbModified) {
				sklog.Warningf("Cached Task has been modified in the DB. Current:\n%v\nCached:\n%v", existing, task)
				return ErrConcurrentUpdate
			}
		}
	}

	// Insert.
	for _, task := range tasks {
		if task.Id == "" {
			if err := db.AssignId(task); err != nil {
				// Should never happen.
				return err
			}
		}

		task.DbModified = time.Now()

		// TODO(borenet): Keep tasks in a sorted slice.
		db.tasks[task.Id] = task.Copy()
		db.TrackModifiedTask(task)
	}
	return nil
}

// NewInMemoryTaskDB returns an extremely simple, inefficient, in-memory TaskDB
// implementation.
func NewInMemoryTaskDB() TaskDB {
	db := &inMemoryTaskDB{
		tasks: map[string]*Task{},
	}
	return db
}

type inMemoryJobDB struct {
	jobs    map[string]*Job
	jobsMtx sync.RWMutex
	ModifiedJobs
}

func (db *inMemoryJobDB) assignId(j *Job) error {
	if j.Id != "" {
		return fmt.Errorf("Job Id already assigned: %v", j.Id)
	}
	j.Id = uuid.New()
	return nil
}

// See docs for JobDB interface.
func (db *inMemoryJobDB) GetJobById(id string) (*Job, error) {
	db.jobsMtx.RLock()
	defer db.jobsMtx.RUnlock()
	if job := db.jobs[id]; job != nil {
		return job.Copy(), nil
	}
	return nil, nil
}

// See docs for JobDB interface.
func (db *inMemoryJobDB) GetJobsFromDateRange(start, end time.Time) ([]*Job, error) {
	db.jobsMtx.RLock()
	defer db.jobsMtx.RUnlock()

	rv := []*Job{}
	// TODO(borenet): Binary search.
	for _, b := range db.jobs {
		if (b.Created.Equal(start) || b.Created.After(start)) && b.Created.Before(end) {
			rv = append(rv, b.Copy())
		}
	}
	sort.Sort(JobSlice(rv))
	return rv, nil
}

// See docs for JobDB interface.
func (db *inMemoryJobDB) PutJob(job *Job) error {
	return db.PutJobs([]*Job{job})
}

// See docs for JobDB interface.
func (db *inMemoryJobDB) PutJobs(jobs []*Job) error {
	db.jobsMtx.Lock()
	defer db.jobsMtx.Unlock()

	// Validate.
	for _, job := range jobs {
		if util.TimeIsZero(job.Created) {
			return fmt.Errorf("Created not set. Job %s created time is %s. %v", job.Id, job.Created, job)
		}

		if existing := db.jobs[job.Id]; existing != nil {
			if !existing.DbModified.Equal(job.DbModified) {
				sklog.Warningf("Cached Job has been modified in the DB. Current:\n%v\nCached:\n%v", existing, job)
				return ErrConcurrentUpdate
			}
		}
	}

	// Insert.
	for _, job := range jobs {
		if job.Id == "" {
			if err := db.assignId(job); err != nil {
				// Should never happen.
				return err
			}
		}
		job.DbModified = time.Now()

		// TODO(borenet): Keep jobs in a sorted slice.
		db.jobs[job.Id] = job.Copy()
		db.TrackModifiedJob(job)
	}
	return nil
}

// NewInMemoryJobDB returns an extremely simple, inefficient, in-memory JobDB
// implementation.
func NewInMemoryJobDB() JobDB {
	db := &inMemoryJobDB{
		jobs: map[string]*Job{},
	}
	return db
}

// NewInMemoryDB returns an extremely simple, inefficient, in-memory DB
// implementation.
func NewInMemoryDB() DB {
	return NewDB(NewInMemoryTaskDB(), NewInMemoryJobDB(), &CommentBox{})
}
