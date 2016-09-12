package db

import (
	"fmt"
	"sync"
	"time"

	"github.com/skia-dev/glog"
)

type TaskCache interface {

	// GetTask returns the task with the given ID, or an error if no such task exists.
	GetTask(string) (*Task, error)

	// GetTaskForCommit retrieves the task with the given name which ran at the
	// given commit, or nil if no such task exists.
	GetTaskForCommit(string, string, string) (*Task, error)

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
	GetTasksForCommits(string, []string) (map[string]map[string]*Task, error)

	// KnownTaskName returns true iff the given task name has been seen before.
	KnownTaskName(string, string) bool

	// UnfinishedTasks returns a list of tasks which were not finished at
	// the time of the last cache update.
	UnfinishedTasks() ([]*Task, error)

	// Update loads new tasks from the database.
	Update() error
}

type taskCache struct {
	db TaskDB
	// map[repo_name][task_spec_name]bool
	knownTaskNames map[string]map[string]bool
	mtx            sync.RWMutex
	queryId        string
	tasks          map[string]*Task
	// map[repo_name][commit_hash][task_spec_name]*Task
	tasksByCommit map[string]map[string]map[string]*Task
	timePeriod    time.Duration
	unfinished    map[string]*Task
}

// See documentation for TaskCache interface.
func (c *taskCache) GetTask(id string) (*Task, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	if t, ok := c.tasks[id]; ok {
		return t.Copy(), nil
	}
	return nil, fmt.Errorf("No such task!")
}

// See documentation for TaskCache interface.
func (c *taskCache) GetTasksForCommits(repo string, commits []string) (map[string]map[string]*Task, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	rv := make(map[string]map[string]*Task, len(commits))
	commitMap := c.tasksByCommit[repo]
	for _, commit := range commits {
		if tasks, ok := commitMap[commit]; ok {
			rv[commit] = make(map[string]*Task, len(tasks))
			for k, v := range tasks {
				rv[commit][k] = v.Copy()
			}
		} else {
			rv[commit] = map[string]*Task{}
		}
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
func (c *taskCache) GetTaskForCommit(repo, commit, name string) (*Task, error) {
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
func (c *taskCache) UnfinishedTasks() ([]*Task, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	rv := make([]*Task, 0, len(c.unfinished))
	for _, t := range c.unfinished {
		rv = append(rv, t.Copy())
	}
	return rv, nil
}

// update inserts the new/updated tasks into the cache. Assumes the caller
// holds a lock.
func (c *taskCache) update(tasks []*Task) error {
	for _, t := range tasks {
		repo := t.Repo
		commitMap, ok := c.tasksByCommit[repo]
		if !ok {
			commitMap = map[string]map[string]*Task{}
			c.tasksByCommit[repo] = commitMap
		}

		// If we already know about this task, the blamelist might,
		// have changed, so we need to remove it from tasksByCommit
		// and re-insert where needed.
		if old, ok := c.tasks[t.Id]; ok {
			for _, commit := range old.Commits {
				delete(commitMap[commit], t.Name)
			}
		}

		// Insert the new task into the main map.
		cpy := t.Copy()
		c.tasks[t.Id] = cpy

		// Insert the task into tasksByCommits.
		for _, commit := range t.Commits {
			if _, ok := commitMap[commit]; !ok {
				commitMap[commit] = map[string]*Task{}
			}
			commitMap[commit][t.Name] = cpy
		}

		// Unfinished tasks.
		if _, ok := c.unfinished[t.Id]; ok {
			delete(c.unfinished, t.Id)
		}
		if !t.Done() {
			c.unfinished[t.Id] = cpy
		}

		// Known task names.
		if nameMap, ok := c.knownTaskNames[repo]; ok {
			nameMap[t.Name] = true
		} else {
			c.knownTaskNames[repo] = map[string]bool{t.Name: true}
		}
	}
	return nil
}

// reset re-initializes c. Assumes the caller holds a lock.
func (c *taskCache) reset() error {
	if c.queryId != "" {
		c.db.StopTrackingModifiedTasks(c.queryId)
	}
	queryId, err := c.db.StartTrackingModifiedTasks()
	if err != nil {
		return err
	}
	now := time.Now()
	start := now.Add(-c.timePeriod)
	glog.Infof("Reading Tasks from %s to %s.", start, now)
	tasks, err := c.db.GetTasksFromDateRange(start, now)
	if err != nil {
		c.db.StopTrackingModifiedTasks(queryId)
		return err
	}
	c.knownTaskNames = map[string]map[string]bool{}
	c.queryId = queryId
	c.tasks = map[string]*Task{}
	c.tasksByCommit = map[string]map[string]map[string]*Task{}
	c.unfinished = map[string]*Task{}
	if err := c.update(tasks); err != nil {
		return err
	}
	return nil
}

// See documentation for TaskCache interface.
func (c *taskCache) Update() error {
	// TODO(borenet): We need to flush old jobs/commits which are outside
	// of our timePeriod so that the cache size is not unbounded.
	newTasks, err := c.db.GetModifiedTasks(c.queryId)
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if IsUnknownId(err) {
		glog.Warningf("Connection to db lost; re-initializing cache from scratch.")
		if err := c.reset(); err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}
	if err := c.update(newTasks); err == nil {
		return nil
	} else {
		return err
	}
}

// NewTaskCache returns a local cache which provides more convenient views of
// task data than the database can provide.
func NewTaskCache(db TaskDB, timePeriod time.Duration) (TaskCache, error) {
	tc := &taskCache{
		db:         db,
		timePeriod: timePeriod,
	}
	if err := tc.reset(); err != nil {
		return nil, err
	}
	return tc, nil
}

type JobCache interface {
	// GetJob returns the job with the given ID, or an error if no such job exists.
	GetJob(string) (*Job, error)

	// ScheduledJobsForCommit indicates whether or not we triggered any jobs
	// for the given repo/commit.
	ScheduledJobsForCommit(string, string) (bool, error)

	// UnfinishedJobs returns a list of jobs which were not finished at
	// the time of the last cache update.
	UnfinishedJobs() ([]*Job, error)

	// Update loads new jobs from the database.
	Update() error
}

type jobCache struct {
	db                 JobDB
	mtx                sync.RWMutex
	queryId            string
	jobs               map[string]*Job
	timePeriod         time.Duration
	triggeredForCommit map[string]map[string]bool
	unfinished         map[string]*Job
}

// See documentation for JobCache interface.
func (c *jobCache) GetJob(id string) (*Job, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	if j, ok := c.jobs[id]; ok {
		return j.Copy(), nil
	}
	return nil, ErrNotFound
}

// See documentation for JobCache interface.
func (c *jobCache) ScheduledJobsForCommit(repo, rev string) (bool, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	return c.triggeredForCommit[repo][rev], nil
}

// See documentation for JobCache interface.
func (c *jobCache) UnfinishedJobs() ([]*Job, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	rv := make([]*Job, 0, len(c.unfinished))
	for _, t := range c.unfinished {
		rv = append(rv, t.Copy())
	}
	return rv, nil
}

// update inserts the new/updated jobs into the cache. Assumes the caller
// holds a lock.
func (c *jobCache) update(jobs []*Job) error {
	for _, j := range jobs {
		// Insert the new job into the main map.
		cpy := j.Copy()
		c.jobs[j.Id] = cpy

		// ScheduledJobsForCommit.
		if _, ok := c.triggeredForCommit[j.Repo]; !ok {
			c.triggeredForCommit[j.Repo] = map[string]bool{}
		}
		c.triggeredForCommit[j.Repo][j.Revision] = true

		// Unfinished jobs.
		if j.Done() {
			delete(c.unfinished, j.Id)
		} else {
			c.unfinished[j.Id] = cpy
		}
	}
	return nil
}

// reset re-initializes c. Assumes the caller holds a lock.
func (c *jobCache) reset() error {
	if c.queryId != "" {
		c.db.StopTrackingModifiedJobs(c.queryId)
	}
	queryId, err := c.db.StartTrackingModifiedJobs()
	if err != nil {
		return err
	}
	now := time.Now()
	start := now.Add(-c.timePeriod)
	glog.Infof("Reading Jobs from %s to %s.", start, now)
	jobs, err := c.db.GetJobsFromDateRange(start, now)
	if err != nil {
		c.db.StopTrackingModifiedJobs(queryId)
		return err
	}
	c.queryId = queryId
	c.jobs = map[string]*Job{}
	c.triggeredForCommit = map[string]map[string]bool{}
	c.unfinished = map[string]*Job{}
	if err := c.update(jobs); err != nil {
		return err
	}
	return nil
}

// See documentation for JobCache interface.
func (c *jobCache) Update() error {
	// TODO(borenet): We need to flush old jobs/commits which are outside
	// of our timePeriod so that the cache size is not unbounded.
	newJobs, err := c.db.GetModifiedJobs(c.queryId)
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if IsUnknownId(err) {
		glog.Warningf("Connection to db lost; re-initializing cache from scratch.")
		if err := c.reset(); err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}
	if err := c.update(newJobs); err == nil {
		return nil
	} else {
		return err
	}
}

// NewJobCache returns a local cache which provides more convenient views of
// job data than the database can provide.
func NewJobCache(db JobDB, timePeriod time.Duration) (JobCache, error) {
	tc := &jobCache{
		db:         db,
		timePeriod: timePeriod,
	}
	if err := tc.reset(); err != nil {
		return nil, err
	}
	return tc, nil
}
