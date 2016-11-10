package db

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/util"

	"github.com/satori/go.uuid"
	"github.com/skia-dev/glog"
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
	t.Id = uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()
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
func (db *inMemoryTaskDB) GetTasksFromDateRange(start, end time.Time) ([]*Task, error) {
	db.tasksMtx.RLock()
	defer db.tasksMtx.RUnlock()

	rv := []*Task{}
	// TODO(borenet): Binary search.
	for _, b := range db.tasks {
		if (b.Created.Equal(start) || b.Created.After(start)) && b.Created.Before(end) {
			rv = append(rv, b.Copy())
		}
	}
	sort.Sort(TaskSlice(rv))
	return rv, nil
}

// See docs for TaskDB interface.
func (db *inMemoryTaskDB) PutTask(task *Task) error {
	db.tasksMtx.Lock()
	defer db.tasksMtx.Unlock()

	if util.TimeIsZero(task.Created) {
		return fmt.Errorf("Created not set. Task %s created time is %s. %v", task.Id, task.Created, task)
	}

	if task.Id == "" {
		if err := db.AssignId(task); err != nil {
			return err
		}
	} else if existing := db.tasks[task.Id]; existing != nil {
		if !existing.DbModified.Equal(task.DbModified) {
			glog.Warningf("Cached Task has been modified in the DB. Current:\n%v\nCached:\n%v", existing, task)
			return ErrConcurrentUpdate
		}
	}
	task.DbModified = time.Now()

	// TODO(borenet): Keep tasks in a sorted slice.
	db.tasks[task.Id] = task
	db.TrackModifiedTask(task)
	return nil
}

// See docs for TaskDB interface.
func (db *inMemoryTaskDB) PutTasks(tasks []*Task) error {
	for _, t := range tasks {
		if err := db.PutTask(t); err != nil {
			return err
		}
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
	j.Id = uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()
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
	db.jobsMtx.Lock()
	defer db.jobsMtx.Unlock()

	if util.TimeIsZero(job.Created) {
		return fmt.Errorf("Created not set. Job %s created time is %s. %v", job.Id, job.Created, job)
	}

	if job.Id == "" {
		if err := db.assignId(job); err != nil {
			return err
		}
	} else if existing := db.jobs[job.Id]; existing != nil {
		if !existing.DbModified.Equal(job.DbModified) {
			glog.Warningf("Cached Job has been modified in the DB. Current:\n%v\nCached:\n%v", existing, job)
			return ErrConcurrentUpdate
		}
	}
	job.DbModified = time.Now()

	// TODO(borenet): Keep jobs in a sorted slice.
	db.jobs[job.Id] = job
	db.TrackModifiedJob(job)
	return nil
}

// See docs for JobDB interface.
func (db *inMemoryJobDB) PutJobs(jobs []*Job) error {
	for _, j := range jobs {
		if err := db.PutJob(j); err != nil {
			return err
		}
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
