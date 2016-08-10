package db

import (
	"sync"
	"time"
)

type TaskCache struct {
	tasks         map[string]*Task
	tasksByCommit map[string]map[string]*Task
	db            DB
	lastUpdate    time.Time
	mtx           sync.RWMutex
	queryId       string
}

// GetTasksForCommits retrieves all tasks which included[1] each of the
// given commits. Returns a map whose keys are commit hashes and values are
// sub-maps whose keys are task spec names and values are tasks.
//
// 1) Blamelist calculation is outside the scope of the TaskCache, but the
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
func (c *TaskCache) GetTasksForCommits(commits []string) (map[string]map[string]*Task, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	rv := make(map[string]map[string]*Task, len(commits))
	for _, commit := range commits {
		if tasks, ok := c.tasksByCommit[commit]; ok {
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

// GetTaskForCommit retrieves the task with the given name which ran at the
// given commit, or nil if no such task exists.
func (c *TaskCache) GetTaskForCommit(name, commit string) (*Task, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	if tasks, ok := c.tasksByCommit[commit]; ok {
		if t, ok := tasks[name]; ok {
			return t.Copy(), nil
		}
	}
	return nil, nil
}

// update inserts the new/updated tasks into the cache. Assumes the caller
// holds a lock.
func (c *TaskCache) update(tasks []*Task) error {
	for _, t := range tasks {
		// If we already know about this task, the blamelist might,
		// have changed, so we need to remove it from tasksByCommit
		// and re-insert where needed.
		if old, ok := c.tasks[t.Id]; ok {
			for _, commit := range old.Commits {
				delete(c.tasksByCommit[commit], t.Name)
			}
		}

		// Insert the new task into the main map.
		c.tasks[t.Id] = t.Copy()

		// Insert the task into tasksByCommits.
		for _, commit := range t.Commits {
			if _, ok := c.tasksByCommit[commit]; !ok {
				c.tasksByCommit[commit] = map[string]*Task{}
			}
			c.tasksByCommit[commit][t.Name] = c.tasks[t.Id]
		}
	}
	return nil
}

// Load new tasks from the database.
func (c *TaskCache) Update() error {
	now := time.Now()
	newTasks, err := c.db.GetModifiedTasks(c.queryId)
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if err != nil {
		if err.Error() == ErrUnknownId.Error() {
			// The database may have restarted. Attempt to re-establish connection.
			queryId, err := c.db.StartTrackingModifiedTasks()
			if err != nil {
				return err
			}
			c.queryId = queryId
			// We may have missed something. Query for tasks since the last
			// successful query.
			tasks, err := c.db.GetTasksFromDateRange(c.lastUpdate, now)
			if err != nil {
				return err
			}
			if err := c.update(tasks); err == nil {
				c.lastUpdate = now
				return nil
			} else {
				return err
			}
		} else {
			return err
		}
	}
	if err := c.update(newTasks); err == nil {
		c.lastUpdate = now
		return nil
	} else {
		return err
	}
}

// NewTaskCache returns a local cache which provides more convenient views of
// task data than the database can provide.
func NewTaskCache(db DB, timePeriod time.Duration) (*TaskCache, error) {
	queryId, err := db.StartTrackingModifiedTasks()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	start := now.Add(-timePeriod)
	tasks, err := db.GetTasksFromDateRange(start, now)
	if err != nil {
		return nil, err
	}
	tc := &TaskCache{
		tasks:         map[string]*Task{},
		tasksByCommit: map[string]map[string]*Task{},
		db:            db,
		lastUpdate:    now,
		queryId:       queryId,
	}
	if err := tc.update(tasks); err != nil {
		return nil, err
	}
	return tc, nil
}
