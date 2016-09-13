package db

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/skia-dev/glog"
)

const (
	// Allocate this much extra capacity when we need to reallocate
	// taskCache.tasksByTime.
	TASKS_BY_TIME_CUSHION = 500
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

	// GetTasksFromDateRange retrieves all tasks which were created in the given
	// date range.
	GetTasksFromDateRange(time.Time, time.Time) ([]*Task, error)

	// KnownTaskName returns true iff the given task name has been seen before.
	KnownTaskName(string, string) bool

	// UnfinishedTasks returns a list of tasks which were not finished at
	// the time of the last cache update.
	UnfinishedTasks() ([]*Task, error)

	// Update loads new tasks from the database.
	Update() error
}

type taskCache struct {
	db TaskReader
	// map[repo_name][task_spec_name]Task.Created for most recent Task.
	knownTaskNames map[string]map[string]time.Time
	mtx            sync.RWMutex
	queryId        string
	tasks          map[string]*Task
	// map[repo_name][commit_hash][task_spec_name]*Task
	tasksByCommit map[string]map[string]map[string]*Task
	// tasksByTime is sorted by Task.Created.
	tasksByTime []*Task
	timePeriod  time.Duration
	unfinished  map[string]*Task
}

// See documentation for TaskCache interface.
func (c *taskCache) GetTask(id string) (*Task, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	if t, ok := c.tasks[id]; ok {
		return t.Copy(), nil
	}
	return nil, ErrNotFound
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

// searchTaskSlice returns the index in tasks of the first Task whose Created
// time is >= ts.
func searchTaskSlice(tasks []*Task, ts time.Time) int {
	return sort.Search(len(tasks), func(i int) bool {
		return !tasks[i].Created.Before(ts)
	})
}

// See documentation for TaskCache interface.
func (c *taskCache) GetTasksFromDateRange(from time.Time, to time.Time) ([]*Task, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	fromIdx := searchTaskSlice(c.tasksByTime, from)
	toIdx := searchTaskSlice(c.tasksByTime, to)
	rv := make([]*Task, toIdx-fromIdx)
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

// removeFromTasksByCommit removes task (which must be a previously-inserted
// Task, not a new Task) from c.tasksByCommit for all of task.Commits. Assumes
// the caller holds a lock.
func (c *taskCache) removeFromTasksByCommit(task *Task) {
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

// expireAndFindFirst removes data from c whose Created time is before start.
// Does not modify c.tasksByTime, but instead returns the index of the first
// task in c.tasksByTime that should not be expired. Assumes the caller holds a
// lock. This is a helper for expireAndUpdate.
func (c *taskCache) expireAndFindFirst(start time.Time) int {
	for _, nameMap := range c.knownTaskNames {
		for name, ts := range nameMap {
			if ts.Before(start) {
				delete(nameMap, name)
			}
		}
	}
	for i, task := range c.tasksByTime {
		if !task.Created.Before(start) {
			return i
		}
		delete(c.tasks, task.Id)
		c.removeFromTasksByCommit(task)
		if _, ok := c.unfinished[task.Id]; ok {
			glog.Warningf("Found unfinished task that is so old it is being expired. %#v", task)
			delete(c.unfinished, task.Id)
		}
	}
	return len(c.tasksByTime)
}

// insertTask updates c.tasks, c.unfinished, and c.knownTaskNames based on t,
// and inserts t into c.tasksByCommit. Does not insert t into c.tasksByTime.
// Assumes the caller holds a lock. This is a helper for expireAndUpdate.
func (c *taskCache) insertTask(t *Task) {
	// Insert the new task into the main map.
	c.tasks[t.Id] = t

	// Insert the task into tasksByCommits.
	repo := t.Repo
	commitMap, ok := c.tasksByCommit[repo]
	if !ok {
		commitMap = map[string]map[string]*Task{}
		c.tasksByCommit[repo] = commitMap
	}
	for _, commit := range t.Commits {
		if _, ok := commitMap[commit]; !ok {
			commitMap[commit] = map[string]*Task{}
		}
		commitMap[commit][t.Name] = t
	}

	// Unfinished tasks.
	if t.Done() {
		delete(c.unfinished, t.Id)
	} else {
		c.unfinished[t.Id] = t
	}

	// Known task names.
	if nameMap, ok := c.knownTaskNames[repo]; ok {
		if ts, ok := nameMap[t.Name]; !ok || ts.Before(t.Created) {
			nameMap[t.Name] = t.Created
		}
	} else {
		c.knownTaskNames[repo] = map[string]time.Time{t.Name: t.Created}
	}
}

// updateTasksByTime sets the new value of c.tasksByTime. Removes Tasks before
// firstIdx, adds newTasks, and sets existing tasks based on updatedTasks.
// newTasks and updatedTasks must be sorted. Assumes the caller holds a lock.
// This is a helper for expireAndUpdate.
func (c *taskCache) updateTasksByTime(firstIdx int, newTasks, updatedTasks []*Task) {
	if len(c.tasksByTime[firstIdx:]) == 0 {
		if len(updatedTasks) > 0 {
			// Could deal with this like:
			// c.tasksByTime = append(newTasks, updatedTasks...)
			// sort.Sort(TaskSlice(c.tasksByTime))
			panic(fmt.Sprintf("taskCache inconsistent; c.tasks contains %d tasks not in c.tasksByTime. %v", len(updatedTasks), updatedTasks))
		}
		c.tasksByTime = newTasks
		return
	}

	// Keep all tasks that haven't expired.
	keep := c.tasksByTime[firstIdx:]
	newSize := len(keep) + len(newTasks)
	if cap(c.tasksByTime) < newSize {
		c.tasksByTime = make([]*Task, 0, newSize+TASKS_BY_TIME_CUSHION)
	} else {
		c.tasksByTime = c.tasksByTime[:0]
	}
	c.tasksByTime = append(c.tasksByTime, keep...)
	c.tasksByTime = append(c.tasksByTime, newTasks...)
	keep = c.tasksByTime[:len(keep)]
	newTasks = c.tasksByTime[len(keep):]

	// updatedTasksRange is the slice of c.tasksByTime that contains the remaining
	// elements of updatedTasks.
	updatedTasksRange := keep
	for _, task := range updatedTasks {
		taskIdx := searchTaskSlice(updatedTasksRange, task.Created)
		// Later tasks will not be before taskIdx.
		updatedTasksRange = updatedTasksRange[taskIdx:]

		// Loop in case there are multiple tasks with the same Created time.
		found := false
		for i, other := range updatedTasksRange {
			if other.Id == task.Id {
				updatedTasksRange[i] = task
				found = true
				break
			}
			if !other.Created.Equal(task.Created) {
				break
			}
		}
		if !found {
			// Could deal with this like:
			// c.tasksByTime = append(c.tasksByTime, task)
			// newTasks = c.tasksByTime[len(keep):]
			// sort.Sort(TaskSlice(newTasks))
			panic(fmt.Sprintf("taskCache inconsistent; c.tasks contains task not in c.tasksByTime. %v", task))
		}
	}

	if len(newTasks) > 0 {
		// Identify the range that requires sorting. This should be a very
		// small range because new tasks should generally have creation times
		// after existing tasks.
		firstNewTask := newTasks[0]
		firstNewTaskIdx := searchTaskSlice(keep, firstNewTask.Created)
		lastExistingTask := keep[len(keep)-1]
		lastExistingTaskIdx := len(keep) + searchTaskSlice(newTasks, lastExistingTask.Created)
		if firstNewTaskIdx < lastExistingTaskIdx {
			sort.Sort(TaskSlice(c.tasksByTime[firstNewTaskIdx:lastExistingTaskIdx]))
		}
	}
}

// expireAndUpdate removes Tasks before start from the cache and inserts the
// new/updated tasks into the cache. Assumes the caller holds a lock. Assumes
// tasks are sorted by Created timestamp.
func (c *taskCache) expireAndUpdate(start time.Time, tasks []*Task) {
	firstIdx := c.expireAndFindFirst(start)
	// newTasks and updatedTasks will be sorted because we add in order of tasks.
	newTasks := make([]*Task, 0, len(tasks))
	updatedTasks := make([]*Task, 0, len(tasks))
	for _, t := range tasks {
		if t.Created.Before(start) {
			continue
		}

		cpy := t.Copy()

		// If we already know about this task, the blamelist might
		// have changed, so we need to remove it from tasksByCommit
		// and re-insert where needed.
		if old, ok := c.tasks[t.Id]; ok {
			if !t.Created.Equal(old.Created) {
				panic(fmt.Sprintf("Changing Created time is not supported. Previously %#v; now %#v", old, t))
			}
			c.removeFromTasksByCommit(old)
			updatedTasks = append(updatedTasks, cpy)
		} else {
			newTasks = append(newTasks, cpy)
		}

		c.insertTask(cpy)
	}

	c.updateTasksByTime(firstIdx, newTasks, updatedTasks)
}

// reset re-initializes c. Assumes the caller holds a lock.
func (c *taskCache) reset(now time.Time) error {
	if c.queryId != "" {
		c.db.StopTrackingModifiedTasks(c.queryId)
	}
	queryId, err := c.db.StartTrackingModifiedTasks()
	if err != nil {
		return err
	}
	start := now.Add(-c.timePeriod)
	glog.Infof("Reading Tasks from %s to %s.", start, now)
	tasks, err := c.db.GetTasksFromDateRange(start, now)
	if err != nil {
		c.db.StopTrackingModifiedTasks(queryId)
		return err
	}
	c.knownTaskNames = map[string]map[string]time.Time{}
	c.queryId = queryId
	c.tasks = map[string]*Task{}
	c.tasksByCommit = map[string]map[string]map[string]*Task{}
	c.unfinished = map[string]*Task{}
	c.expireAndUpdate(start, tasks)
	return nil
}

// update implements Update with the given current time for testing.
func (c *taskCache) update(now time.Time) error {
	newTasks, err := c.db.GetModifiedTasks(c.queryId)
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if IsUnknownId(err) {
		glog.Warningf("Connection to db lost; re-initializing cache from scratch.")
		if err := c.reset(now); err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}
	start := now.Add(-c.timePeriod)
	c.expireAndUpdate(start, newTasks)
	return nil
}

// See documentation for TaskCache interface.
func (c *taskCache) Update() error {
	return c.update(time.Now())
}

// NewTaskCache returns a local cache which provides more convenient views of
// task data than the database can provide.
func NewTaskCache(db TaskReader, timePeriod time.Duration) (TaskCache, error) {
	tc := &taskCache{
		db:         db,
		timePeriod: timePeriod,
	}
	if err := tc.reset(time.Now()); err != nil {
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
