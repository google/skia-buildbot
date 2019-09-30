package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/types"
)

type inMemoryTaskDB struct {
	tasks      map[string]*types.Task
	tasksMtx   sync.RWMutex
	modTasksCh map[chan<- []*types.Task]bool
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
	if len(tasks) > firestore.MAX_TRANSACTION_DOCS {
		sklog.Errorf("Inserting %d tasks, which is more than the Firestore maximum of %d; consider switching to PutTasksInChunks.", len(tasks), firestore.MAX_TRANSACTION_DOCS)
	}
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
	now := time.Now()
	for _, task := range tasks {
		if task.Id == "" {
			if err := d.AssignId(task); err != nil {
				// Should never happen.
				return err
			}
		}

		// We can't use the same DbModified timestamp for two updates,
		// or we risk losing updates. Increment the timestamp if
		// necessary.
		if !now.After(task.DbModified) {
			task.DbModified = task.DbModified.Add(time.Nanosecond)
		} else {
			task.DbModified = now
		}

		// TODO(borenet): Keep tasks in a sorted slice.
		d.tasks[task.Id] = task.Copy()
	}
	// Send the modified tasks to any listeners.
	for ch := range d.modTasksCh {
		// Don't block, in case a listener has forgotten about us.
		go func(ch chan<- []*types.Task) {
			tasksCpy := make([]*types.Task, 0, len(tasks))
			for _, t := range tasks {
				tasksCpy = append(tasksCpy, t.Copy())
			}
			ch <- tasksCpy
		}(ch)
	}
	return nil
}

// See docs for TaskDB interface.
func (d *inMemoryTaskDB) PutTasksInChunks(tasks []*types.Task) error {
	return util.ChunkIter(len(tasks), firestore.MAX_TRANSACTION_DOCS, func(i, j int) error {
		return d.PutTasks(tasks[i:j])
	})
}

// See docs for TaskDB interface.
func (d *inMemoryTaskDB) ModifiedTasksCh(ctx context.Context) <-chan []*types.Task {
	d.tasksMtx.Lock()
	defer d.tasksMtx.Unlock()
	rv := make(chan []*types.Task)
	d.modTasksCh[rv] = true
	done := ctx.Done()
	if done != nil {
		go func() {
			<-done
			d.tasksMtx.Lock()
			defer d.tasksMtx.Unlock()
			delete(d.modTasksCh, rv)
			close(rv)
		}()
	}
	go func() {
		rv <- []*types.Task{}
	}()
	return rv
}

// NewInMemoryTaskDB returns an extremely simple, inefficient, in-memory TaskDB
// implementation.
func NewInMemoryTaskDB() db.TaskDB {
	return &inMemoryTaskDB{
		tasks:      map[string]*types.Task{},
		modTasksCh: map[chan<- []*types.Task]bool{},
	}
}

type inMemoryJobDB struct {
	jobs      map[string]*types.Job
	jobsMtx   sync.RWMutex
	modJobsCh map[chan<- []*types.Job]bool
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
func (d *inMemoryJobDB) GetJobsFromDateRange(start, end time.Time, repo string) ([]*types.Job, error) {
	d.jobsMtx.RLock()
	defer d.jobsMtx.RUnlock()

	rv := []*types.Job{}
	// TODO(borenet): Binary search.
	for _, b := range d.jobs {
		if repo != "" && b.Repo != repo {
			continue
		}
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
	if len(jobs) > firestore.MAX_TRANSACTION_DOCS {
		sklog.Errorf("Inserting %d jobs, which is more than the Firestore maximum of %d; consider switching to PutJobsInChunks.", len(jobs), firestore.MAX_TRANSACTION_DOCS)
	}
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
	now := time.Now()
	for _, job := range jobs {
		if job.Id == "" {
			if err := d.assignId(job); err != nil {
				// Should never happen.
				return err
			}
		}

		// We can't use the same DbModified timestamp for two updates,
		// or we risk losing updates. Increment the timestamp if
		// necessary.
		if job.DbModified == now {
			job.DbModified = job.DbModified.Add(time.Nanosecond)
		} else {
			job.DbModified = now
		}

		// TODO(borenet): Keep jobs in a sorted slice.
		d.jobs[job.Id] = job.Copy()
	}
	for ch := range d.modJobsCh {
		// Don't block, in case a listener has forgotten about us.
		go func(ch chan<- []*types.Job) {
			jobsCpy := make([]*types.Job, 0, len(jobs))
			for _, j := range jobs {
				jobsCpy = append(jobsCpy, j.Copy())
			}
			ch <- jobsCpy
		}(ch)
	}
	return nil
}

// See docs for JobDB interface.
func (d *inMemoryJobDB) PutJobsInChunks(jobs []*types.Job) error {
	return util.ChunkIter(len(jobs), firestore.MAX_TRANSACTION_DOCS, func(i, j int) error {
		return d.PutJobs(jobs[i:j])
	})
}

// See docs for JobDB interface.
func (d *inMemoryJobDB) ModifiedJobsCh(ctx context.Context) <-chan []*types.Job {
	d.jobsMtx.Lock()
	defer d.jobsMtx.Unlock()
	rv := make(chan []*types.Job)
	d.modJobsCh[rv] = true
	done := ctx.Done()
	if done != nil {
		go func() {
			<-done
			d.jobsMtx.Lock()
			defer d.jobsMtx.Unlock()
			delete(d.modJobsCh, rv)
			close(rv)
		}()
	}
	go func() {
		rv <- []*types.Job{}
	}()
	return rv
}

// NewInMemoryJobDB returns an extremely simple, inefficient, in-memory JobDB
// implementation.
func NewInMemoryJobDB() db.JobDB {
	db := &inMemoryJobDB{
		jobs:      map[string]*types.Job{},
		modJobsCh: map[chan<- []*types.Job]bool{},
	}
	return db
}

// NewInMemoryDB returns an extremely simple, inefficient, in-memory DB
// implementation.
func NewInMemoryDB() db.DB {
	return db.NewDB(NewInMemoryTaskDB(), NewInMemoryJobDB(), NewCommentBox())
}
