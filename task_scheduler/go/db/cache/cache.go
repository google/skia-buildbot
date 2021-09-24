package cache

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

const (
	// Allocate enough space for this many tasks.
	TASKS_INIT_CAPACITY = 60000
	// Allocate enough space for this many jobs.
	JOBS_INIT_CAPACITY = 10000
)

type TaskCache interface {

	// GetTask returns the task with the given ID, or an error if no such task exists.
	GetTask(string) (*types.Task, error)

	// GetTaskMaybeExpired does the same as GetTask but tries to dig into
	// the DB in case the Task is old enough to have scrolled out of the
	// cache window.
	GetTaskMaybeExpired(string) (*types.Task, error)

	// GetTaskForCommit retrieves the task with the given name which ran at the
	// given commit, or nil if no such task exists.
	GetTaskForCommit(string, string, string) (*types.Task, error)

	// GetTasksByKey returns the tasks with the given TaskKey, sorted
	// by creation time.
	GetTasksByKey(*types.TaskKey) ([]*types.Task, error)

	// GetTasksForCommits retrieves all tasks which included[1] each of the
	// given commits. Returns a map whose keys are commit hashes and values are
	// sub-maps whose keys are task spec names and values are tasks.
	//
	// 1) Blamelist calculation is outside the scope of the taskCache, but the
	//    implied assumption here is that there is at most one task for each
	//    task spec which has a given commit in its blamelist. The user is
	//    responsible for inserting tasks into the database so that this invariant
	//    is maintained. Generally, a more recent task will "steal" commits from an
	//    earlier task's blamelist, if the blamelists overlap. There are three
	//    cases to consider:
	//       1. The newer task ran at a newer commit than the older task. Its
	//          blamelist consists of all commits not covered by the previous task,
	//          and therefore does not overlap with the older task's blamelist.
	//       2. The newer task ran at the same commit as the older task. Its
	//          blamelist is the same as the previous task's blamelist, and
	//          therefore it "steals" all commits from the previous task, whose
	//          blamelist becomes empty.
	//       3. The newer task ran at a commit which was in the previous task's
	//          blamelist. Its blamelist consists of the commits in the previous
	//          task's blamelist which it also covered. Those commits move out of
	//          the previous task's blamelist and into the newer task's blamelist.
	GetTasksForCommits(string, []string) (map[string]map[string]*types.Task, error)

	// GetTasksFromDateRange retrieves all tasks which were created in the given
	// date range.
	GetTasksFromDateRange(from time.Time, to time.Time) ([]*types.Task, error)

	// KnownTaskName returns true iff the given task name has been seen
	// before for a non-forced, non-tryjob run.
	KnownTaskName(string, string) bool

	// UnfinishedTasks returns a list of tasks which were not finished at
	// the time of the last cache update. Fake tasks are not included.
	UnfinishedTasks() ([]*types.Task, error)

	// Update loads new tasks from the database.
	Update(ctx context.Context) error

	// AddTasks adds tasks directly to the TaskCache.
	AddTasks([]*types.Task)
}

type taskCache struct {
	db db.TaskReader
	// map[repo_name][task_spec_name]Task.Created for most recent Task.
	knownTaskNames map[string]map[string]time.Time
	mtx            sync.RWMutex
	tasks          map[string]*types.Task
	// map[repo_name][commit_hash][task_spec_name]*Task
	tasksByCommit map[string]map[string]map[string]*types.Task
	// map[TaskKey]map[task_id]*Task
	tasksByKey map[types.TaskKey]map[string]*types.Task
	// tasksByTime is sorted by Task.Created.
	tasksByTime []*types.Task
	timeWindow  *window.Window
	unfinished  map[string]*types.Task

	// Stash modified tasks until Update() is called.
	modified map[string]*types.Task
	modMtx   sync.Mutex
	onModFn  func()
}

// See documentation for TaskCache interface.
func (c *taskCache) GetTask(id string) (*types.Task, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	if t, ok := c.tasks[id]; ok {
		return t.Copy(), nil
	}
	return nil, db.ErrNotFound
}

// See documentation for TaskCache interface.
func (c *taskCache) GetTaskMaybeExpired(id string) (*types.Task, error) {
	t, err := c.GetTask(id)
	if err == nil {
		return t, nil
	} else if err != db.ErrNotFound {
		return nil, err
	}
	// Fall back to searching the DB.
	t, err = c.db.GetTaskById(id)
	if err != nil {
		return nil, err
	} else if t == nil {
		return nil, db.ErrNotFound
	}
	return t, nil
}

// See documentation for TaskCache interface.
func (c *taskCache) GetTasksByKey(k *types.TaskKey) ([]*types.Task, error) {
	if !k.Valid() {
		return nil, fmt.Errorf("TaskKey is invalid: %v", k)
	}

	c.mtx.RLock()
	defer c.mtx.RUnlock()

	tasks := c.tasksByKey[*k]
	rv := make([]*types.Task, 0, len(tasks))
	for _, t := range tasks {
		rv = append(rv, t.Copy())
	}
	sort.Sort(types.TaskSlice(rv))
	return rv, nil
}

// See documentation for TaskCache interface.
func (c *taskCache) GetTasksForCommits(repo string, commits []string) (map[string]map[string]*types.Task, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	rv := make(map[string]map[string]*types.Task, len(commits))
	commitMap := c.tasksByCommit[repo]
	for _, commit := range commits {
		if tasks, ok := commitMap[commit]; ok {
			rv[commit] = make(map[string]*types.Task, len(tasks))
			for k, v := range tasks {
				rv[commit][k] = v.Copy()
			}
		} else {
			rv[commit] = map[string]*types.Task{}
		}
	}
	return rv, nil
}

// searchTaskSlice returns the index in tasks of the first Task whose Created
// time is >= ts.
func searchTaskSlice(tasks []*types.Task, ts time.Time) int {
	return sort.Search(len(tasks), func(i int) bool {
		return !tasks[i].Created.Before(ts)
	})
}

// See documentation for TaskCache interface.
func (c *taskCache) GetTasksFromDateRange(from time.Time, to time.Time) ([]*types.Task, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	fromIdx := searchTaskSlice(c.tasksByTime, from)
	toIdx := searchTaskSlice(c.tasksByTime, to)
	rv := make([]*types.Task, toIdx-fromIdx)
	for i, task := range c.tasksByTime[fromIdx:toIdx] {
		rv[i] = task.Copy()
	}
	return rv, nil
}

// See documentation for TaskCache interface.
func (c *taskCache) KnownTaskName(repo, name string) bool {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	_, ok := c.knownTaskNames[repo][name]
	return ok
}

// See documentation for TaskCache interface.
func (c *taskCache) GetTaskForCommit(repo, commit, name string) (*types.Task, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	commitMap, ok := c.tasksByCommit[repo]
	if !ok {
		return nil, nil
	}
	if tasks, ok := commitMap[commit]; ok {
		if t, ok := tasks[name]; ok {
			return t.Copy(), nil
		}
	}
	return nil, nil
}

// See documentation for TaskCache interface.
func (c *taskCache) UnfinishedTasks() ([]*types.Task, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	rv := make([]*types.Task, 0, len(c.unfinished))
	for _, t := range c.unfinished {
		rv = append(rv, t.Copy())
	}
	return rv, nil
}

// removeFromTasksByCommit removes task (which must be a previously-inserted
// Task, not a new Task) from c.tasksByCommit for all of task.Commits. Assumes
// the caller holds a lock.
func (c *taskCache) removeFromTasksByCommit(task *types.Task) {
	if commitMap, ok := c.tasksByCommit[task.Repo]; ok {
		for _, commit := range task.Commits {
			// Shouldn't be necessary to check other.Id == task.Id, but being paranoid.
			if other, ok := commitMap[commit][task.Name]; ok && other.Id == task.Id {
				delete(commitMap[commit], task.Name)
				if len(commitMap[commit]) == 0 {
					delete(commitMap, commit)
				}
			}
		}
	}

}

// expireTasks removes data from c whose Created time is before the beginning
// of the Window. Assumes the caller holds a lock. This is a helper for
// Update.
func (c *taskCache) expireTasks() {
	for repoUrl, nameMap := range c.knownTaskNames {
		for name, ts := range nameMap {
			if !c.timeWindow.TestTime(repoUrl, ts) {
				delete(nameMap, name)
			}
		}
	}
	for i, task := range c.tasksByTime {
		if c.timeWindow.TestTime(task.Repo, task.Created) {
			c.tasksByTime = c.tasksByTime[i:]
			return
		}

		// Tasks by ID.
		delete(c.tasks, task.Id)

		// Tasks by commit.
		c.removeFromTasksByCommit(task)

		// Tasks by key.
		byKey, ok := c.tasksByKey[task.TaskKey]
		if ok {
			delete(byKey, task.Id)
			if len(byKey) == 0 {
				delete(c.tasksByKey, task.TaskKey)
			}
		}

		// Tasks by time.
		c.tasksByTime[i] = nil // Allow GC.

		// Unfinished tasks.
		if _, ok := c.unfinished[task.Id]; ok {
			sklog.Warningf("Found unfinished task that is so old it is being expired. %#v", task)
			delete(c.unfinished, task.Id)
		}
	}
	if len(c.tasksByTime) > 0 {
		sklog.Warningf("All tasks expired because they are outside the window.")
		c.tasksByTime = nil
	}
}

// insertOrUpdateTask inserts task into the cache if it is a new task, or
// updates the existing entries if not. Assumes the caller holds a lock. This is
// a helper for Update.
func (c *taskCache) insertOrUpdateTask(task *types.Task) {
	old, isUpdate := c.tasks[task.Id]
	if isUpdate && !task.DbModified.After(old.DbModified) {
		return
	}

	// Insert the new task into the main map.
	c.tasks[task.Id] = task

	// Insert into tasksByKey.
	byKey, ok := c.tasksByKey[task.TaskKey]
	if !ok {
		byKey = map[string]*types.Task{}
		c.tasksByKey[task.TaskKey] = byKey
	}
	byKey[task.Id] = task

	if isUpdate {
		// If we already know about this task, the blamelist might have changed, so
		// we need to remove it from tasksByCommit and re-insert where needed.
		c.removeFromTasksByCommit(old)
	}
	// Insert the task into tasksByCommits.
	commitMap, ok := c.tasksByCommit[task.Repo]
	if !ok {
		commitMap = map[string]map[string]*types.Task{}
		c.tasksByCommit[task.Repo] = commitMap
	}
	for _, commit := range task.Commits {
		if _, ok := commitMap[commit]; !ok {
			commitMap[commit] = map[string]*types.Task{}
		}
		commitMap[commit][task.Name] = task
	}

	if isUpdate {
		// Loop in case there are multiple tasks with the same Created time.
		for i := searchTaskSlice(c.tasksByTime, task.Created); i < len(c.tasksByTime); i++ {
			other := c.tasksByTime[i]
			if other.Id == task.Id {
				c.tasksByTime[i] = task
				break
			}
			if !other.Created.Equal(task.Created) {
				panic(fmt.Sprintf("taskCache inconsistent; c.tasks contains task not in c.tasksByTime. old: %v, task: %v", old, task))
			}
		}
	} else {
		// If profiling indicates this code is slow or GCs too much, see
		// https://skia.googlesource.com/buildbot/+show/0cf94832dd57f0e7b5b9f1b28546181d15dbbbc6
		// for a different implementation.
		// Most common case is that the new task should be inserted at the end.
		if len(c.tasksByTime) == 0 {
			c.tasksByTime = append(make([]*types.Task, 0, TASKS_INIT_CAPACITY), task)
		} else if lastTask := c.tasksByTime[len(c.tasksByTime)-1]; !task.Created.Before(lastTask.Created) {
			c.tasksByTime = append(c.tasksByTime, task)
		} else {
			insertIdx := searchTaskSlice(c.tasksByTime, task.Created)
			// Extend size by one:
			c.tasksByTime = append(c.tasksByTime, nil)
			// Move later elements out of the way:
			copy(c.tasksByTime[insertIdx+1:], c.tasksByTime[insertIdx:])
			// Assign at the correct index:
			c.tasksByTime[insertIdx] = task
		}
	}

	// Unfinished tasks.
	if !task.Done() && !task.Fake() {
		c.unfinished[task.Id] = task
	} else if isUpdate {
		delete(c.unfinished, task.Id)
	}

	// Known task names.
	if !task.IsForceRun() && !task.IsTryJob() {
		if nameMap, ok := c.knownTaskNames[task.Repo]; ok {
			if ts, ok := nameMap[task.Name]; !ok || ts.Before(task.Created) {
				nameMap[task.Name] = task.Created
			}
		} else {
			c.knownTaskNames[task.Repo] = map[string]time.Time{task.Name: task.Created}
		}
	}
}

// See documentation for TaskCache interface.
func (c *taskCache) Update(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "taskcache_Update")
	defer span.End()
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.modMtx.Lock()
	defer c.modMtx.Unlock()
	c.expireTasks()
	for _, t := range c.modified {
		if c.timeWindow.TestTime(t.Repo, t.Created) {
			c.insertOrUpdateTask(t)
		}
	}
	c.modified = map[string]*types.Task{}
	return nil
}

// See documentation for TaskCache interface.
func (c *taskCache) AddTasks(tasks []*types.Task) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for _, t := range tasks {
		if c.timeWindow.TestTime(t.Repo, t.Created) {
			c.insertOrUpdateTask(t.Copy())
		}
	}
}

// NewTaskCache returns a local cache which provides more convenient views of
// task data than the database can provide. The last parameter is a callback
// function which is called when modified tasks are received from the DB. This
// is used for testing.
func NewTaskCache(ctx context.Context, d db.TaskReader, timeWindow *window.Window, onModifiedTasks func()) (TaskCache, error) {
	mod := d.ModifiedTasksCh(ctx)
	tasks, err := db.GetTasksFromWindow(d, timeWindow, time.Now())
	if err != nil {
		return nil, err
	}
	c := &taskCache{
		db:             d,
		knownTaskNames: map[string]map[string]time.Time{},
		tasks:          map[string]*types.Task{},
		tasksByCommit:  map[string]map[string]map[string]*types.Task{},
		tasksByKey:     map[types.TaskKey]map[string]*types.Task{},
		unfinished:     map[string]*types.Task{},
		modified:       map[string]*types.Task{},
		timeWindow:     timeWindow,
		onModFn:        onModifiedTasks,
	}
	for _, t := range tasks {
		if c.timeWindow.TestTime(t.Repo, t.Created) {
			c.insertOrUpdateTask(t)
		}
	}
	go func() {
		for tasks := range mod {
			c.modMtx.Lock()
			for _, task := range tasks {
				if old, ok := c.modified[task.Id]; !ok || task.DbModified.After(old.DbModified) {
					c.modified[task.Id] = task
				}
			}
			c.modMtx.Unlock()
			if c.onModFn != nil {
				c.onModFn()
			}
		}
		sklog.Warning("Modified tasks channel closed; is this expected?")
	}()
	return c, nil
}

type JobCache interface {
	// GetAllCachedJobs returns every job in the cache.
	GetAllCachedJobs() []*types.Job

	// GetJob returns the job with the given ID, or an error if no such job exists.
	GetJob(string) (*types.Job, error)

	// GetJobMaybeExpired does the same as GetJob but tries to dig into the
	// DB in case the Job is old enough to have scrolled out of the cache
	// window.
	GetJobMaybeExpired(string) (*types.Job, error)

	// GetJobsByRepoState retrieves all known jobs with the given name at
	// the given RepoState. Does not search the underlying DB.
	GetJobsByRepoState(string, types.RepoState) ([]*types.Job, error)

	// GetJobsFromDateRange retrieves all jobs which were created in the
	// given date range.
	GetJobsFromDateRange(time.Time, time.Time) ([]*types.Job, error)

	// GetMatchingJobsFromDateRange retrieves all jobs which were created
	// in the given date range and match one of the given job names.
	GetMatchingJobsFromDateRange(names []string, from time.Time, to time.Time) (map[string][]*types.Job, error)

	// UnfinishedJobs returns a list of jobs which were not finished at
	// the time of the last cache update.
	UnfinishedJobs() ([]*types.Job, error)

	// Update loads new jobs from the database.
	Update(ctx context.Context) error

	// AddJobs adds jobs directly to the JobCache.
	AddJobs([]*types.Job)
}

type jobCache struct {
	db                 db.JobReader
	mtx                sync.RWMutex
	jobs               map[string]*types.Job
	jobsByNameAndState map[types.RepoState]map[string]map[string]*types.Job
	// jobsByTime is sorted by Task.Created.
	jobsByTime []*types.Job
	timeWindow *window.Window
	unfinished map[string]*types.Job

	modified map[string]*types.Job
	modMtx   sync.Mutex
	onModFn  func()
}

// See documentation for JobCache interface.
func (c *jobCache) GetAllCachedJobs() []*types.Job {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	rv := make([]*types.Job, 0, len(c.jobs))
	for _, j := range c.jobs {
		rv = append(rv, j.Copy())
	}
	sort.Sort(types.JobSlice(rv))
	return rv
}

// See documentation for JobCache interface.
func (c *jobCache) GetJob(id string) (*types.Job, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	if j, ok := c.jobs[id]; ok {
		return j.Copy(), nil
	}
	return nil, db.ErrNotFound
}

// See documentation for JobCache interface.
func (c *jobCache) GetJobMaybeExpired(id string) (*types.Job, error) {
	j, err := c.GetJob(id)
	if err == nil {
		return j, nil
	}
	if err != db.ErrNotFound {
		return nil, err
	}
	return c.db.GetJobById(id)
}

// See documentation for JobCache interface.
func (c *jobCache) GetJobsByRepoState(name string, rs types.RepoState) ([]*types.Job, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	jobs := c.jobsByNameAndState[rs][name]
	rv := make([]*types.Job, 0, len(jobs))
	for _, j := range jobs {
		rv = append(rv, j)
	}
	sort.Sort(types.JobSlice(rv))
	return rv, nil
}

// searchJobSlice returns the index in tasks of the first Job whose Created
// time is >= ts.
func searchJobSlice(jobs []*types.Job, ts time.Time) int {
	return sort.Search(len(jobs), func(i int) bool {
		return !jobs[i].Created.Before(ts)
	})
}

// jobSliceIsSorted returns true if jobs is sorted by Created time.
func jobSliceIsSorted(jobs []*types.Job) bool {
	for i := 0; i < len(jobs)-1; i++ {
		if jobs[i+1].Created.Before(jobs[i].Created) {
			return false
		}
	}
	return true
}

// See documentation for JobCache interface.
func (c *jobCache) GetJobsFromDateRange(from time.Time, to time.Time) ([]*types.Job, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	fromIdx := searchJobSlice(c.jobsByTime, from)
	toIdx := searchJobSlice(c.jobsByTime, to)
	rv := make([]*types.Job, toIdx-fromIdx)
	for i, job := range c.jobsByTime[fromIdx:toIdx] {
		rv[i] = job.Copy()
	}
	return rv, nil
}

// See documentation for JobCache interface.
func (c *jobCache) GetMatchingJobsFromDateRange(names []string, from time.Time, to time.Time) (map[string][]*types.Job, error) {
	jobs, err := c.GetJobsFromDateRange(from, to)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(names))
	for _, name := range names {
		m[name] = true
	}
	rv := make(map[string][]*types.Job, len(names))
	for _, job := range jobs {
		if m[job.Name] {
			rv[job.Name] = append(rv[job.Name], job)
		}
	}
	return rv, nil
}

// See documentation for JobCache interface.
func (c *jobCache) UnfinishedJobs() ([]*types.Job, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	rv := make([]*types.Job, 0, len(c.unfinished))
	for _, t := range c.unfinished {
		rv = append(rv, t.Copy())
	}
	return rv, nil
}

// expireJobs removes data from c where Job.Created is before start. Assumes
// the caller holds a lock. This is a helper for Update.
func (c *jobCache) expireJobs() {
	expiredUnfinishedCount := 0
	defer func() {
		if expiredUnfinishedCount > 0 {
			sklog.Infof("Expired %d unfinished jobs created before window.", expiredUnfinishedCount)
		}
	}()
	defer func() {
		// Debugging for https://bugs.chromium.org/p/skia/issues/detail?id=9444
		if len(c.jobsByTime) > 0 {
			firstTs := c.jobsByTime[0].Created
			for _, job := range c.unfinished {
				if job.Created.Before(firstTs) {
					sklog.Warningf("Found job %q in unfinished, Created %s, before first jobsByTime, Created %s. %+v", job.Id, job.Created.Format(time.RFC3339Nano), firstTs.Format(time.RFC3339Nano), job)
				}
			}
		} else {
			if len(c.unfinished) > 0 {
				sklog.Warningf("Found %d jobs in unfinished when jobsByTime is empty", len(c.unfinished))
			}
		}
	}()
	for i, job := range c.jobsByTime {
		if c.timeWindow.TestTime(job.Repo, job.Created) {
			c.jobsByTime = c.jobsByTime[i:]
			return
		}
		// Allow GC.
		c.jobsByTime[i] = nil
		delete(c.jobs, job.Id)
		delete(c.unfinished, job.Id)

		byName := c.jobsByNameAndState[job.RepoState]
		byId := byName[job.Name]
		delete(byId, job.Id)
		if len(byId) == 0 {
			delete(byName, job.Name)
		}
		if len(byName) == 0 {
			delete(c.jobsByNameAndState, job.RepoState)
		}

		if !job.Done() {
			expiredUnfinishedCount++
		}
	}
	if len(c.jobsByTime) > 0 {
		sklog.Warningf("All jobs expired because they are outside the window.")
		c.jobsByTime = nil
	}
}

// insertOrUpdateJob inserts the new/updated job into the cache. Assumes the
// caller holds a lock. This is a helper for Update.
func (c *jobCache) insertOrUpdateJob(job *types.Job) {
	old, isUpdate := c.jobs[job.Id]
	if isUpdate && !job.DbModified.After(old.DbModified) {
		return
	}

	// Insert the new job into the main map.
	c.jobs[job.Id] = job

	// Map by RepoState, Name, and ID.
	byName, ok := c.jobsByNameAndState[job.RepoState]
	if !ok {
		byName = map[string]map[string]*types.Job{}
		c.jobsByNameAndState[job.RepoState] = byName
	}
	byId, ok := byName[job.Name]
	if !ok {
		byId = map[string]*types.Job{}
		byName[job.Name] = byId
	}
	byId[job.Id] = job

	// Unfinished jobs.
	if job.Done() {
		delete(c.unfinished, job.Id)
	} else {
		c.unfinished[job.Id] = job
	}

	if isUpdate {
		// Loop in case there are multiple tasks with the same Created time.
		for i := searchJobSlice(c.jobsByTime, job.Created); i < len(c.jobsByTime); i++ {
			other := c.jobsByTime[i]
			if other.Id == job.Id {
				c.jobsByTime[i] = job
				break
			}
			if !other.Created.Equal(job.Created) {
				panic(fmt.Sprintf("jobCache inconsistent; c.jobs contains job not in c.jobsByTime. old: %v, task: %v", old, job))
			}
		}
	} else {
		// If profiling indicates this code is slow or GCs too much, see
		// https://skia.googlesource.com/buildbot/+show/0cf94832dd57f0e7b5b9f1b28546181d15dbbbc6
		// for a different implementation.
		// Most common case is that the new job should be inserted at the end.
		if len(c.jobsByTime) == 0 {
			c.jobsByTime = append(make([]*types.Job, 0, JOBS_INIT_CAPACITY), job)
		} else if lastJob := c.jobsByTime[len(c.jobsByTime)-1]; !job.Created.Before(lastJob.Created) {
			c.jobsByTime = append(c.jobsByTime, job)
		} else {
			insertIdx := searchJobSlice(c.jobsByTime, job.Created)
			// Extend size by one:
			c.jobsByTime = append(c.jobsByTime, nil)
			// Move later elements out of the way:
			copy(c.jobsByTime[insertIdx+1:], c.jobsByTime[insertIdx:])
			// Assign at the correct index:
			c.jobsByTime[insertIdx] = job
		}
	}
}

// See documentation for JobCache interface.
func (c *jobCache) Update(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "jobcache_Update")
	defer span.End()
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.modMtx.Lock()
	defer c.modMtx.Unlock()
	c.expireJobs()
	for _, job := range c.modified {
		if c.timeWindow.TestTime(job.Repo, job.Created) {
			c.insertOrUpdateJob(job)
		}
	}
	// Debugging for https://bugs.chromium.org/p/skia/issues/detail?id=9444
	if !jobSliceIsSorted(c.jobsByTime) {
		sklog.Errorf("jobsByTime is not sorted after Update of %v", c.modified)
	}
	c.modified = map[string]*types.Job{}
	return nil
}

// See documentation for JobCache interface.
func (c *jobCache) AddJobs(jobs []*types.Job) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for _, job := range jobs {
		if c.timeWindow.TestTime(job.Repo, job.Created) {
			c.insertOrUpdateJob(job.Copy())
		}
	}
	// Debugging for https://bugs.chromium.org/p/skia/issues/detail?id=9444
	if !jobSliceIsSorted(c.jobsByTime) {
		sklog.Errorf("jobsByTime is not sorted after AddJobs of %v", jobs)
	}
}

// NewJobCache returns a local cache which provides more convenient views of
// job data than the database can provide. The last parameter is a callback
// function which is called when modified tasks are received from the DB. This
// is used for testing.
func NewJobCache(ctx context.Context, d db.JobReader, timeWindow *window.Window, onModifiedJobs func()) (JobCache, error) {
	mod := d.ModifiedJobsCh(ctx)
	jobs, err := db.GetJobsFromWindow(d, timeWindow, time.Now())
	if err != nil {
		return nil, err
	}
	c := &jobCache{
		db:                 d,
		jobs:               map[string]*types.Job{},
		jobsByNameAndState: map[types.RepoState]map[string]map[string]*types.Job{},
		unfinished:         map[string]*types.Job{},
		modified:           map[string]*types.Job{},
		timeWindow:         timeWindow,
		onModFn:            onModifiedJobs,
	}
	for _, job := range jobs {
		if c.timeWindow.TestTime(job.Repo, job.Created) {
			c.insertOrUpdateJob(job)
		}
	}
	// Debugging for https://bugs.chromium.org/p/skia/issues/detail?id=9444
	if !jobSliceIsSorted(c.jobsByTime) {
		return nil, fmt.Errorf("jobsByTime is not sorted after initial load")
	}
	go func() {
		for jobs := range mod {
			c.modMtx.Lock()
			for _, job := range jobs {
				if old, ok := c.modified[job.Id]; !ok || job.DbModified.After(old.DbModified) {
					c.modified[job.Id] = job
				}
			}
			c.modMtx.Unlock()
			if c.onModFn != nil {
				c.onModFn()
			}
		}
		sklog.Warning("Modified jobs channel closed; is this expected?")
	}()
	return c, nil
}
