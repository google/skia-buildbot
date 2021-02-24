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

type InMemoryTaskDB struct {
	tasks         map[string]*types.Task
	tasksMtx      sync.RWMutex
	mod           chan<- []*types.Task
	modClients    map[chan<- []*types.Task]context.Context
	modClientsMtx sync.Mutex
	modClientsWg  sync.WaitGroup
}

// See docs for TaskDB interface. Does not take any locks.
func (d *InMemoryTaskDB) AssignId(t *types.Task) error {
	if t.Id != "" {
		return fmt.Errorf("Task Id already assigned: %v", t.Id)
	}
	t.Id = uuid.New().String()
	return nil
}

// See docs for TaskDB interface.
func (d *InMemoryTaskDB) GetTaskById(id string) (*types.Task, error) {
	d.tasksMtx.RLock()
	defer d.tasksMtx.RUnlock()
	if task := d.tasks[id]; task != nil {
		return task.Copy(), nil
	}
	return nil, nil
}

// See docs for TaskDB interface.
func (d *InMemoryTaskDB) GetTasksFromDateRange(start, end time.Time, repo string) ([]*types.Task, error) {
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
func (d *InMemoryTaskDB) PutTask(task *types.Task) error {
	return d.PutTasks([]*types.Task{task})
}

// See docs for TaskDB interface.
func (d *InMemoryTaskDB) PutTasks(tasks []*types.Task) error {
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
	added := make([]*types.Task, 0, len(tasks))
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
		cpy := task.Copy()
		added = append(added, cpy)
		d.tasks[task.Id] = cpy
	}
	// Send the modified tasks to any listeners.
	d.modClientsWg.Add(1) // Corresponds to Done() in NewInMemoryTaskDB.
	d.mod <- added
	return nil
}

// See docs for TaskDB interface.
func (d *InMemoryTaskDB) PutTasksInChunks(tasks []*types.Task) error {
	return util.ChunkIter(len(tasks), firestore.MAX_TRANSACTION_DOCS, func(i, j int) error {
		return d.PutTasks(tasks[i:j])
	})
}

// See docs for TaskDB interface.
func (d *InMemoryTaskDB) ModifiedTasksCh(ctx context.Context) <-chan []*types.Task {
	d.modClientsMtx.Lock()
	defer d.modClientsMtx.Unlock()
	localCh := make(chan []*types.Task)
	rv := make(chan []*types.Task)
	d.modClients[localCh] = ctx
	done := ctx.Done()
	go func() {
		// The DB spec states that we should pass an initial value along
		// the channel.
		rv <- []*types.Task{}
		for {
			select {
			case mod := <-localCh:
				rv <- mod
				// Corresponds to Add() in NewInMemoryTaskDB.
				d.modClientsWg.Done()
			case <-done:
				close(rv)
				// Remove the local channel. Note that we don't
				// close it, because other goroutines might be
				// trying to write to it.
				d.modClientsMtx.Lock()
				delete(d.modClients, localCh)
				d.modClientsMtx.Unlock()
				return
			}
		}
	}()
	return rv
}

// Wait for all clients to receive modified data.
func (d *InMemoryTaskDB) Wait() {
	d.modClientsWg.Wait()
}

// watchModifiedData multiplexes modified data out to the various clients.
func (d *InMemoryTaskDB) watchModifiedData(mod <-chan []*types.Task) {
	for data := range mod {
		d.modClientsMtx.Lock()
		for ch, ctx := range d.modClients {
			if ctx.Err() != nil {
				continue
			}

			// Corresponds to Done() in ModifiedTasksCh.
			d.modClientsWg.Add(1)
			go func(ctx context.Context, ch chan<- []*types.Task, data []*types.Task) {
				send := make([]*types.Task, 0, len(data))
				for _, elem := range data {
					send = append(send, elem.Copy())
				}
				select {
				case ch <- send:
				case <-ctx.Done():
				}
			}(ctx, ch, data)
		}
		d.modClientsMtx.Unlock()
		// Corresponds to Add() in PutTasks.
		d.modClientsWg.Done()
	}
}

// SearchTasks implements db.TaskReader.
func (d *InMemoryTaskDB) SearchTasks(ctx context.Context, params *db.TaskSearchParams) ([]*types.Task, error) {
	d.tasksMtx.RLock()
	defer d.tasksMtx.RUnlock()
	rv := []*types.Task{}
	for _, task := range d.tasks {
		if db.MatchTask(task, params) {
			rv = append(rv, task.Copy())
		}
	}
	return rv, nil
}

// NewInMemoryTaskDB returns an extremely simple, inefficient, in-memory TaskDB
// implementation.
func NewInMemoryTaskDB() *InMemoryTaskDB {
	mod := make(chan []*types.Task)
	rv := &InMemoryTaskDB{
		tasks:      map[string]*types.Task{},
		mod:        mod,
		modClients: map[chan<- []*types.Task]context.Context{},
	}
	go rv.watchModifiedData(mod)
	return rv
}

type InMemoryJobDB struct {
	jobs          map[string]*types.Job
	jobsMtx       sync.RWMutex
	mod           chan<- []*types.Job
	modClients    map[chan<- []*types.Job]context.Context
	modClientsMtx sync.Mutex
	modClientsWg  sync.WaitGroup
}

func (d *InMemoryJobDB) assignId(j *types.Job) error {
	if j.Id != "" {
		return fmt.Errorf("Job Id already assigned: %v", j.Id)
	}
	j.Id = uuid.New().String()
	return nil
}

// See docs for JobDB interface.
func (d *InMemoryJobDB) GetJobById(id string) (*types.Job, error) {
	d.jobsMtx.RLock()
	defer d.jobsMtx.RUnlock()
	if job := d.jobs[id]; job != nil {
		return job.Copy(), nil
	}
	return nil, nil
}

// See docs for JobDB interface.
func (d *InMemoryJobDB) GetJobsFromDateRange(start, end time.Time, repo string) ([]*types.Job, error) {
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
func (d *InMemoryJobDB) PutJob(job *types.Job) error {
	return d.PutJobs([]*types.Job{job})
}

// See docs for JobDB interface.
func (d *InMemoryJobDB) PutJobs(jobs []*types.Job) error {
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
	added := make([]*types.Job, 0, len(jobs))
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
		cpy := job.Copy()
		added = append(added, cpy)
		d.jobs[job.Id] = cpy
	}
	d.modClientsWg.Add(1) // Corresponds to Done() in NewInMemoryTaskDB.
	d.mod <- added
	return nil
}

// See docs for JobDB interface.
func (d *InMemoryJobDB) PutJobsInChunks(jobs []*types.Job) error {
	return util.ChunkIter(len(jobs), firestore.MAX_TRANSACTION_DOCS, func(i, j int) error {
		return d.PutJobs(jobs[i:j])
	})
}

// See docs for JobDB interface.
func (d *InMemoryJobDB) ModifiedJobsCh(ctx context.Context) <-chan []*types.Job {
	d.modClientsMtx.Lock()
	defer d.modClientsMtx.Unlock()
	localCh := make(chan []*types.Job)
	rv := make(chan []*types.Job)
	d.modClients[localCh] = ctx
	done := ctx.Done()
	go func() {
		// The DB spec states that we should pass an initial value along
		// the channel.
		rv <- []*types.Job{}
		for {
			select {
			case mod := <-localCh:
				rv <- mod
				// Corresponds to Add() in NewInMemoryTaskDB.
				d.modClientsWg.Done()
			case <-done:
				close(rv)
				// Remove the local channel. Note that we don't
				// close it, because other goroutines might be
				// trying to write to it.
				d.modClientsMtx.Lock()
				delete(d.modClients, localCh)
				d.modClientsMtx.Unlock()
				return
			}
		}
	}()
	return rv
}

// Wait for all clients to receive modified data.
func (d *InMemoryJobDB) Wait() {
	d.modClientsWg.Wait()
}

// This goroutine multiplexes modified data out to the various clients.
func (d *InMemoryJobDB) watchModifiedData(mod <-chan []*types.Job) {
	for data := range mod {
		d.modClientsMtx.Lock()
		for ch, ctx := range d.modClients {
			if ctx.Err() != nil {
				continue
			}

			// Corresponds to Done() in ModifiedTasksCh.
			d.modClientsWg.Add(1)
			go func(ctx context.Context, ch chan<- []*types.Job, data []*types.Job) {
				send := make([]*types.Job, 0, len(data))
				for _, elem := range data {
					send = append(send, elem.Copy())
				}
				select {
				case ch <- send:
				case <-ctx.Done():
				}
			}(ctx, ch, data)
		}
		d.modClientsMtx.Unlock()
		d.modClientsWg.Done()
	}
}

// SearchJobs implements db.JobReader.
func (d *InMemoryJobDB) SearchJobs(ctx context.Context, params *db.JobSearchParams) ([]*types.Job, error) {
	d.jobsMtx.RLock()
	defer d.jobsMtx.RUnlock()
	rv := []*types.Job{}
	for _, job := range d.jobs {
		if db.MatchJob(job, params) {
			rv = append(rv, job.Copy())
		}
	}
	return rv, nil
}

// NewInMemoryJobDB returns an extremely simple, inefficient, in-memory JobDB
// implementation.
func NewInMemoryJobDB() *InMemoryJobDB {
	mod := make(chan []*types.Job)
	rv := &InMemoryJobDB{
		jobs:       map[string]*types.Job{},
		mod:        mod,
		modClients: map[chan<- []*types.Job]context.Context{},
	}
	go rv.watchModifiedData(mod)
	return rv
}

type InMemoryDB struct {
	*InMemoryTaskDB
	*InMemoryJobDB
	*CommentBox
}

func (d *InMemoryDB) Wait() {
	d.InMemoryTaskDB.Wait()
	d.InMemoryJobDB.Wait()
	d.CommentBox.Wait()
}

// NewInMemoryDB returns an extremely simple, inefficient, in-memory DB
// implementation.
func NewInMemoryDB() *InMemoryDB {
	return &InMemoryDB{
		InMemoryTaskDB: NewInMemoryTaskDB(),
		InMemoryJobDB:  NewInMemoryJobDB(),
		CommentBox:     NewCommentBox(),
	}
}

var _ db.DB = NewInMemoryDB()
