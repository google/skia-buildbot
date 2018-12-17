package memory

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/modified"
	"go.skia.org/infra/task_scheduler/go/types"
)

type inMemoryTaskDB struct {
	tasks    map[string]*types.Task
	tasksMtx sync.RWMutex
	db.ModifiedTasks
}

// See docs for TaskDB interface. Does not take any locks.
func (d *inMemoryTaskDB) AssignId(t *types.Task) error {
	if t.Id != "" {
		return fmt.Errorf("Task Id already assigned: %v", t.Id)
	}
	t.Id = uuid.New().String()
	return nil
}

// See docs for TaskDB interface.
func (d *inMemoryTaskDB) GetTaskById(id string) (*types.Task, error) {
	d.tasksMtx.RLock()
	defer d.tasksMtx.RUnlock()
	if task := d.tasks[id]; task != nil {
		return task.Copy(), nil
	}
	return nil, nil
}

// See docs for TaskDB interface.
func (d *inMemoryTaskDB) GetTasksFromDateRange(start, end time.Time, repo string) ([]*types.Task, error) {
	d.tasksMtx.RLock()
	defer d.tasksMtx.RUnlock()

	rv := []*types.Task{}
	// TODO(borenet): Binary search.
	for _, b := range d.tasks {
		if (b.Created.Equal(start) || b.Created.After(start)) && b.Created.Before(end) {
			if repo == "" || b.Repo == repo {
				rv = append(rv, b.Copy())
			}
		}
	}
	sort.Sort(types.TaskSlice(rv))
	return rv, nil
}

// See docs for TaskDB interface.
func (d *inMemoryTaskDB) PutTask(task *types.Task) error {
	return d.PutTasks([]*types.Task{task})
}

// See docs for TaskDB interface.
func (d *inMemoryTaskDB) PutTasks(tasks []*types.Task) error {
	d.tasksMtx.Lock()
	defer d.tasksMtx.Unlock()

	// Validate.
	for _, task := range tasks {
		if util.TimeIsZero(task.Created) {
			return fmt.Errorf("Created not set. Task %s created time is %s. %v", task.Id, task.Created, task)
		}

		if existing := d.tasks[task.Id]; existing != nil {
			if !existing.DbModified.Equal(task.DbModified) {
				sklog.Warningf("Cached Task has been modified in the DB. Current:\n%v\nCached:\n%v", existing, task)
				return db.ErrConcurrentUpdate
			}
		}
	}

	// Insert.
	for _, task := range tasks {
		if task.Id == "" {
			if err := d.AssignId(task); err != nil {
				// Should never happen.
				return err
			}
		}

		task.DbModified = time.Now()

		// TODO(borenet): Keep tasks in a sorted slice.
		d.tasks[task.Id] = task.Copy()
		d.TrackModifiedTask(task)
	}
	return nil
}

// NewInMemoryTaskDB returns an extremely simple, inefficient, in-memory TaskDB
// implementation.
func NewInMemoryTaskDB(modTasks db.ModifiedTasks) db.TaskDB {
	if modTasks == nil {
		modTasks = &modified.ModifiedTasksImpl{}
	}
	db := &inMemoryTaskDB{
		tasks:         map[string]*types.Task{},
		ModifiedTasks: modTasks,
	}
	return db
}

type inMemoryJobDB struct {
	jobs    map[string]*types.Job
	jobsMtx sync.RWMutex
	db.ModifiedJobs
}

func (d *inMemoryJobDB) assignId(j *types.Job) error {
	if j.Id != "" {
		return fmt.Errorf("Job Id already assigned: %v", j.Id)
	}
	j.Id = uuid.New().String()
	return nil
}

// See docs for JobDB interface.
func (d *inMemoryJobDB) GetJobById(id string) (*types.Job, error) {
	d.jobsMtx.RLock()
	defer d.jobsMtx.RUnlock()
	if job := d.jobs[id]; job != nil {
		return job.Copy(), nil
	}
	return nil, nil
}

// See docs for JobDB interface.
func (d *inMemoryJobDB) GetJobsFromDateRange(start, end time.Time) ([]*types.Job, error) {
	d.jobsMtx.RLock()
	defer d.jobsMtx.RUnlock()

	rv := []*types.Job{}
	// TODO(borenet): Binary search.
	for _, b := range d.jobs {
		if (b.Created.Equal(start) || b.Created.After(start)) && b.Created.Before(end) {
			rv = append(rv, b.Copy())
		}
	}
	sort.Sort(types.JobSlice(rv))
	return rv, nil
}

// See docs for JobDB interface.
func (d *inMemoryJobDB) PutJob(job *types.Job) error {
	return d.PutJobs([]*types.Job{job})
}

// See docs for JobDB interface.
func (d *inMemoryJobDB) PutJobs(jobs []*types.Job) error {
	d.jobsMtx.Lock()
	defer d.jobsMtx.Unlock()

	// Validate.
	for _, job := range jobs {
		if util.TimeIsZero(job.Created) {
			return fmt.Errorf("Created not set. Job %s created time is %s. %v", job.Id, job.Created, job)
		}

		if existing := d.jobs[job.Id]; existing != nil {
			if !existing.DbModified.Equal(job.DbModified) {
				sklog.Warningf("Cached Job has been modified in the DB. Current:\n%v\nCached:\n%v", existing, job)
				return db.ErrConcurrentUpdate
			}
		}
	}

	// Insert.
	for _, job := range jobs {
		if job.Id == "" {
			if err := d.assignId(job); err != nil {
				// Should never happen.
				return err
			}
		}
		job.DbModified = time.Now()

		// TODO(borenet): Keep jobs in a sorted slice.
		d.jobs[job.Id] = job.Copy()
		d.TrackModifiedJob(job)
	}
	return nil
}

// NewInMemoryJobDB returns an extremely simple, inefficient, in-memory JobDB
// implementation.
func NewInMemoryJobDB(modJobs db.ModifiedJobs) db.JobDB {
	if modJobs == nil {
		modJobs = &modified.ModifiedJobsImpl{}
	}
	db := &inMemoryJobDB{
		jobs:         map[string]*types.Job{},
		ModifiedJobs: modJobs,
	}
	return db
}

// NewInMemoryDB returns an extremely simple, inefficient, in-memory DB
// implementation.
func NewInMemoryDB(mod db.ModifiedData) db.DB {
	if mod == nil {
		mod = modified.NewModifiedData()
	}
	return db.NewDB(NewInMemoryTaskDB(mod), NewInMemoryJobDB(mod), &CommentBox{ModifiedComments: mod})
}
